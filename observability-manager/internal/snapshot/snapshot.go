// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of the resources observability analysis
// needs — ServiceMonitors, Services, and pod-bearing workloads — from ConfigHub
// and joins them with Unit / Space / Target metadata into the observability model.
package snapshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/observability-manager/internal/observability"
)

const k8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"

type resourceQuery struct {
	whereData     string
	whereResource string
}

// resourceQueries enumerate the resource families observability analysis needs.
var resourceQueries = []resourceQuery{
	{"kind = 'ServiceMonitor'", "ConfigHub.ResourceType LIKE 'monitoring.coreos.com/%/ServiceMonitor'"},
	{"kind = 'Service'", "ConfigHub.ResourceType = 'v1/Service'"},
	{"kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet')", "ConfigHub.ResourceType LIKE 'apps/v1/%'"},
	{"kind IN ('Job', 'CronJob')", "ConfigHub.ResourceType LIKE 'batch/%'"},
	{"kind = 'Pod'", "ConfigHub.ResourceType = 'v1/Pod'"},
}

// UnitMeta is the per-Unit metadata the snapshot joins onto resources.
type UnitMeta struct {
	UnitID                string            `json:"unitId"`
	Slug                  string            `json:"slug"`
	SpaceID               string            `json:"spaceId"`
	SpaceSlug             string            `json:"spaceSlug"`
	SpaceLabels           map[string]string `json:"spaceLabels,omitempty"`
	TargetID              string            `json:"targetId,omitempty"`
	TargetSlug            string            `json:"targetSlug,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	GateCount             int               `json:"gateCount"`
	HeadRevisionNum       int64             `json:"headRevisionNum"`
	LiveRevisionNum       int64             `json:"liveRevisionNum"`
	UpstreamRevisionNum   int64             `json:"upstreamRevisionNum,omitempty"`
	LastChangeDescription string            `json:"lastChangeDescription,omitempty"`
}

func (u UnitMeta) Gated() bool { return u.GateCount > 0 }
func (u UnitMeta) Unapplied() bool {
	return u.LiveRevisionNum == 0 || u.LiveRevisionNum < u.HeadRevisionNum
}

// Snapshot is a fleet-wide observability view.
type Snapshot struct {
	Clusters  map[string]*observability.ClusterObservability
	Resources []observability.FleetResource
	Units     map[string]UnitMeta
	Filter    string `json:"filter,omitempty"`
}

type rawResource struct {
	ResourceType string `json:"ResourceType"`
	ResourceName string `json:"ResourceName"`
	ResourceBody string `json:"ResourceBody"`
}

func isCanonicalSpace(labels map[string]string) bool {
	switch labels["Variant"] {
	case "base":
		return true
	}
	switch labels["role"] {
	case "base", "policy":
		return true
	}
	return false
}

// Load fetches and assembles the fleet snapshot scoped by a single ConfigHub Unit
// `where` predicate (empty = everything the user can view).
func Load(ctx context.Context, c *cubapi.Client, where string) (*Snapshot, error) {
	unitWhere := k8sUnitsWhere
	if where != "" {
		unitWhere = k8sUnitsWhere + " AND " + where
	}
	var (
		wg       sync.WaitGroup
		units    []*goclientnew.ExtendedUnit
		unitsErr error
		outcomes = make([][]cubapi.UnitOutcome, len(resourceQueries))
		queryErr = make([]error, len(resourceQueries))
	)
	wg.Add(1 + len(resourceQueries))
	go func() { defer wg.Done(); units, unitsErr = listUnits(ctx, c, unitWhere) }()
	for i, q := range resourceQueries {
		go func(i int, q resourceQuery) {
			defer wg.Done()
			outcomes[i], queryErr[i] = getResources(ctx, c, unitWhere, q.whereData, q.whereResource)
		}(i, q)
	}
	wg.Wait()

	if unitsErr != nil {
		return nil, fmt.Errorf("list units: %w", unitsErr)
	}
	for i, err := range queryErr {
		if err != nil {
			return nil, fmt.Errorf("get resources (%s): %w", resourceQueries[i].whereResource, err)
		}
	}

	inScope := make(map[string]UnitMeta, len(units))
	for _, eu := range units {
		if eu.Unit == nil || isZeroUUID(eu.Unit.UnitID) {
			continue
		}
		unitID := eu.Unit.UnitID.String()
		targetID := ""
		if eu.Unit.TargetID != nil {
			targetID = eu.Unit.TargetID.String()
		}
		var spaceSlug string
		var spaceLabels map[string]string
		if eu.Space != nil {
			spaceSlug = eu.Space.Slug
			spaceLabels = eu.Space.Labels
		}
		targetSlug := ""
		if eu.Target != nil {
			targetSlug = eu.Target.Slug
		}
		inScope[unitID] = UnitMeta{
			UnitID:                unitID,
			Slug:                  eu.Unit.Slug,
			SpaceID:               eu.Unit.SpaceID.String(),
			SpaceSlug:             spaceSlug,
			SpaceLabels:           spaceLabels,
			TargetID:              targetID,
			TargetSlug:            targetSlug,
			Labels:                eu.Unit.Labels,
			GateCount:             len(eu.Unit.ApplyGates),
			HeadRevisionNum:       eu.Unit.HeadRevisionNum,
			LiveRevisionNum:       eu.Unit.LiveRevisionNum,
			UpstreamRevisionNum:   eu.Unit.UpstreamRevisionNum,
			LastChangeDescription: eu.Unit.LastChangeDescription,
		}
	}

	var resources []observability.FleetResource
	collect := func(resps []cubapi.UnitOutcome) {
		for _, r := range resps {
			if !r.Success || r.UnitID == "" {
				continue
			}
			meta, ok := inScope[r.UnitID]
			if !ok {
				continue
			}
			space := r.SpaceSlug
			if space == "" {
				space = meta.SpaceSlug
			}
			cluster := meta.TargetSlug
			if cluster == "" {
				cluster = space
			}
			canonical := isCanonicalSpace(meta.SpaceLabels)
			for _, raw := range decodeResourceList(r.Outputs["ResourceList"]) {
				if raw.ResourceBody == "" {
					continue
				}
				var doc any
				if err := json.Unmarshal([]byte(raw.ResourceBody), &doc); err != nil {
					continue
				}
				resources = append(resources, observability.FleetResource{
					Origin: observability.ResourceOrigin{
						Cluster:      cluster,
						Target:       meta.TargetSlug,
						Space:        space,
						SpaceID:      r.SpaceID,
						SpaceLabels:  meta.SpaceLabels,
						UnitID:       r.UnitID,
						UnitSlug:     firstNonEmpty(r.UnitSlug, meta.Slug),
						ResourceName: raw.ResourceName,
						Canonical:    canonical,
					},
					Doc: doc,
				})
			}
		}
	}
	for _, o := range outcomes {
		collect(o)
	}

	var forAnalysis []observability.FleetResource
	for _, r := range resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}

	return &Snapshot{
		Clusters:  observability.BuildFleet(forAnalysis),
		Resources: resources,
		Units:     inScope,
		Filter:    where,
	}, nil
}

func listUnits(ctx context.Context, c *cubapi.Client, where string) ([]*goclientnew.ExtendedUnit, error) {
	return cubapi.ListUnits(ctx, c, cubapi.NewWhere(where), cubapi.ListOpts{Include: "SpaceID,TargetID"})
}

func getResources(ctx context.Context, c *cubapi.Client, where, whereData, whereResource string) ([]cubapi.UnitOutcome, error) {
	res, err := cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{FunctionName: "get-resources", Arguments: []api.FunctionArgument{{Value: "json"}}},
		cubapi.Selector{Where: where, WhereData: whereData, WhereResource: whereResource},
		cubapi.Change{})
	if err != nil {
		return nil, err
	}
	return res.Outcomes, nil
}

func decodeResourceList(encoded string) []rawResource {
	if encoded == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var list []rawResource
	if err := json.Unmarshal(decoded, &list); err != nil {
		return nil
	}
	return list
}

func isZeroUUID(id goclientnew.UUID) bool { return id == goclientnew.UUID{} }

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
