// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of the Kubernetes workloads and
// PodDisruptionBudgets relevant to workload-posture analysis from ConfigHub via
// the API. It discovers every Kubernetes/YAML Unit the user can view (optionally
// narrowed by scope filters), extracts the pod-bearing workloads and PDBs
// server-side via the get-resources function, and joins them with Unit / Space /
// Target metadata into the workload analysis model.
//
// Several get-resources invocations run in parallel because a single
// WhereResource conjunction can't express the union of resource types.
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

	"github.com/confighub/examples/workload-manager/internal/workload"
)

const k8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"

// resourceQuery is one (whereData, whereResource) pair driving a get-resources
// invocation. whereData narrows Units server-side by the kind their config
// carries; whereResource selects which resource types get returned.
type resourceQuery struct {
	whereData     string
	whereResource string
}

// resourceQueries enumerate the resource families the workload model needs. They
// run as parallel get-resources invocations.
var resourceQueries = []resourceQuery{
	{"kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet')", "ConfigHub.ResourceType LIKE 'apps/v1/%'"},
	{"kind IN ('Job', 'CronJob')", "ConfigHub.ResourceType LIKE 'batch/%'"},
	{"kind = 'Pod'", "ConfigHub.ResourceType = 'v1/Pod'"},
	{"kind = 'PodDisruptionBudget'", "ConfigHub.ResourceType LIKE 'policy/%/PodDisruptionBudget'"},
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

// Gated reports whether the Unit has any ApplyGates attached.
func (u UnitMeta) Gated() bool { return u.GateCount > 0 }

// Unapplied reports whether the Unit's head revision has not been applied live.
func (u UnitMeta) Unapplied() bool {
	return u.LiveRevisionNum == 0 || u.LiveRevisionNum < u.HeadRevisionNum
}

// Snapshot is a fleet-wide workload-posture view.
type Snapshot struct {
	// Clusters holds workload-relevant entities per cluster, excluding canonical
	// (base/policy) definitions. Keyed by Target slug (Space slug for unbound
	// Units).
	Clusters map[string]*workload.ClusterWorkloads
	// Resources is every parsed resource, including canonical ones, for the
	// explorer.
	Resources []workload.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped by
	// (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

type rawResource struct {
	ResourceType string `json:"ResourceType"`
	ResourceName string `json:"ResourceName"`
	ResourceBody string `json:"ResourceBody"`
}

// Canonical Spaces hold definitions, not deployed config, so their Units stay out
// of cluster analysis. The standard Variant=base label marks a base/template
// Space; a `role` label of base/policy is also treated as canonical.
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

// Load fetches and assembles the fleet snapshot using the given API client,
// scoped by a single ConfigHub Unit `where` predicate (empty = everything the
// user can view). The predicate may reference Unit, Space, and Target metadata;
// it is applied server-side to both the Unit list and the get-resources fetch,
// so only matching Units' resources are pulled.
func Load(ctx context.Context, c *cubapi.Client, where string) (*Snapshot, error) {
	// ConfigHub `where` is flat AND-only (no parentheses), so clauses are joined
	// with a bare AND.
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

	// The server has already scoped the Units to the predicate; build metadata for
	// every returned Unit and join resources onto it by UnitID.
	inScope := make(map[string]UnitMeta, len(units))
	for _, eu := range units {
		if eu.Unit == nil || isZeroUUID(eu.Unit.UnitID) {
			continue
		}
		unitID := eu.Unit.UnitID.String()
		spaceID := eu.Unit.SpaceID.String()
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
			SpaceID:               spaceID,
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

	var resources []workload.FleetResource
	collect := func(resps []cubapi.UnitOutcome) {
		for _, r := range resps {
			if !r.Success || r.UnitID == "" {
				continue
			}
			meta, ok := inScope[r.UnitID]
			if !ok {
				continue // out of scope
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
				resources = append(resources, workload.FleetResource{
					Origin: workload.ResourceOrigin{
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

	// Canonical definitions stay out of cluster analysis.
	var forAnalysis []workload.FleetResource
	for _, r := range resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}

	return &Snapshot{
		Clusters:  workload.BuildFleet(forAnalysis),
		Resources: resources,
		Units:     inScope,
		Filter:    where,
	}, nil
}

// listUnits returns every Kubernetes/YAML Unit matching the given where
// predicate, with Space and Target expanded so the snapshot can join their slugs
// and labels.
func listUnits(ctx context.Context, c *cubapi.Client, where string) ([]*goclientnew.ExtendedUnit, error) {
	return cubapi.ListUnits(ctx, c, cubapi.NewWhere(where),
		cubapi.ListOpts{Include: "SpaceID,TargetID"})
}

// getResources runs the get-resources function over the matching Units and
// returns the per-Unit outcomes (the resource list lands in Outputs). The same
// where predicate that scopes the Unit list scopes the resource fetch.
func getResources(ctx context.Context, c *cubapi.Client, where, whereData, whereResource string) ([]cubapi.UnitOutcome, error) {
	res, err := cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{FunctionName: "get-resources", Arguments: []api.FunctionArgument{{Value: "json"}}},
		cubapi.Selector{
			Where:         where,
			WhereData:     whereData,
			WhereResource: whereResource,
		},
		cubapi.Change{}) // read-only: dry-run
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

func isZeroUUID(id goclientnew.UUID) bool {
	return id == goclientnew.UUID{}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
