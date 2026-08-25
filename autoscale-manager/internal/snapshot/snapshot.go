// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of the resources autoscaling analysis
// needs — HorizontalPodAutoscalers, KEDA ScaledObjects, scalable workloads
// (Deployment/StatefulSet), and PodDisruptionBudgets — from ConfigHub and joins
// them with Unit / Space / Target metadata into the autoscale model.//
// They arrive in one query: Resources are queried in SQL, so the resource type
// is a predicate the database evaluates rather than a function invoked over
// every Unit, and each resource's configuration comes back as already-parsed
// JSON.
package snapshot

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/autoscale-manager/internal/autoscale"
)

const k8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"

// ClusterNone is the cluster key for Units the fleet view cannot attribute to
// a cluster: their Space has no release Target, so there is nothing to name.
// They group under one bucket rather than each Space standing in for a cluster
// of its own, which inflated the cluster count with things that are not clusters.
const ClusterNone = "None"

// resourceTypePatterns match the ResourceTypes the autoscaling model needs: the
// autoscalers, their scale targets, and the PodDisruptionBudgets that can block
// a scale-down.
var resourceTypePatterns = []string{
	`autoscaling/[^/]+/HorizontalPodAutoscaler`,
	`keda\.sh/[^/]+/ScaledObject`,
	`apps/v1/(Deployment|StatefulSet)`,
	`policy/[^/]+/PodDisruptionBudget`,
}

// resourceTypeMatch is the exact test, applied to what comes back.
var resourceTypeMatch = regexp.MustCompile(`(?i)^(` + strings.Join(resourceTypePatterns, "|") + `)$`)

// resourceTypeWhere asks the server for the same union in one clause: the filter
// language is flat AND-only, so a union of types is one regular expression
// rather than ORed equalities. A filter literal cannot carry a backslash, so the
// escapes are dropped — an unescaped `.` matches any character, which makes the
// clause broader than the patterns, never narrower, and resourceTypeMatch
// narrows it again on the way out.
var resourceTypeWhere = "ResourceType ~* '^(" +
	strings.ReplaceAll(strings.Join(resourceTypePatterns, "|"), `\`, "") + ")$'"

// UnitMeta is the per-Unit metadata the snapshot joins onto resources.
type UnitMeta struct {
	UnitID              string            `json:"unitId"`
	Slug                string            `json:"slug"`
	SpaceID             string            `json:"spaceId"`
	SpaceSlug           string            `json:"spaceSlug"`
	SpaceLabels         map[string]string `json:"spaceLabels,omitempty"`
	TargetID            string            `json:"targetId,omitempty"`
	TargetSlug          string            `json:"targetSlug,omitempty"`
	GateCount           int               `json:"gateCount"`
	HeadRevisionNum     int64             `json:"headRevisionNum"`
	LiveRevisionNum     int64             `json:"liveRevisionNum"`
	UpstreamRevisionNum int64             `json:"upstreamRevisionNum,omitempty"`
}

func (u UnitMeta) Gated() bool { return u.GateCount > 0 }
func (u UnitMeta) Unapplied() bool {
	return u.LiveRevisionNum == 0 || u.LiveRevisionNum < u.HeadRevisionNum
}

// Snapshot is a fleet-wide autoscaling view.
type Snapshot struct {
	Clusters  map[string]*autoscale.ClusterAutoscale
	Resources []autoscale.FleetResource
	Units     map[string]UnitMeta
	Filter    string `json:"filter,omitempty"`
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
	units, err := listUnits(ctx, c, unitWhere)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}

	inScope := make(map[string]UnitMeta, len(units))
	unitIDs := make([]goclientnew.UUID, 0, len(units))
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
			UnitID:              unitID,
			Slug:                eu.Unit.Slug,
			SpaceID:             eu.Unit.SpaceID.String(),
			SpaceSlug:           spaceSlug,
			SpaceLabels:         spaceLabels,
			TargetID:            targetID,
			TargetSlug:          targetSlug,
			GateCount:           len(eu.Unit.ApplyGates),
			HeadRevisionNum:     eu.Unit.HeadRevisionNum,
			LiveRevisionNum:     eu.Unit.LiveRevisionNum,
			UpstreamRevisionNum: eu.Unit.UpstreamRevisionNum,
		}
	}

	extended, err := listResources(ctx, c, unitIDs)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	var resources []autoscale.FleetResource
	for _, er := range extended {
		if er.Resource == nil || er.Resource.Data == nil {
			continue
		}
		r := er.Resource
		// The clause is deliberately broader than the patterns where a filter
		// literal cannot spell them exactly, so match again.
		if !resourceTypeMatch.MatchString(r.ResourceType) {
			continue
		}
		meta, ok := inScope[r.UnitID.String()]
		if !ok {
			continue // out of scope
		}
		space := r.SpaceSlug
		if space == "" {
			space = meta.SpaceSlug
		}
		cluster := meta.TargetSlug
		if cluster == "" {
			cluster = ClusterNone
		}
		resources = append(resources, autoscale.FleetResource{
			Origin: autoscale.ResourceOrigin{
				Cluster:      cluster,
				Target:       meta.TargetSlug,
				Space:        space,
				SpaceID:      r.SpaceID.String(),
				SpaceLabels:  meta.SpaceLabels,
				UnitID:       r.UnitID.String(),
				UnitSlug:     firstNonEmpty(r.UnitSlug, meta.Slug),
				ResourceName: r.ResourceName,
				Canonical:    isCanonicalSpace(meta.SpaceLabels),
			},
			Doc: r.Data,
		})
	}

	var forAnalysis []autoscale.FleetResource
	for _, r := range resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}

	return &Snapshot{
		Clusters:  autoscale.BuildFleet(forAnalysis),
		Resources: resources,
		Units:     inScope,
		Filter:    where,
	}, nil
}

func listUnits(ctx context.Context, c *cubapi.Client, where string) ([]*goclientnew.ExtendedUnit, error) {
	return cubapi.ListUnits(ctx, c, cubapi.NewWhere(where), cubapi.ListOpts{Include: "SpaceID,TargetID"})
}

func isZeroUUID(id goclientnew.UUID) bool { return id == goclientnew.UUID{} }

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// listResources reads the resources inside the in-scope Units from the Resource
// entity, which mirrors the configuration in each Unit's data and is queried in
// SQL.
//
// The Units are named by ID rather than by re-sending the caller's predicate:
// that predicate selects Units and is written against Unit attributes, which the
// resource query would need re-spelled with a `Unit.` prefix, and the IDs are
// already in hand from the Unit list the snapshot needs anyway.
func listResources(ctx context.Context, c *cubapi.Client, unitIDs []goclientnew.UUID) ([]*goclientnew.ExtendedResource, error) {
	if len(unitIDs) == 0 {
		return nil, nil
	}
	// No Include: the Space and Unit slugs are columns on the row, and the
	// Target slug comes from the Unit metadata already loaded. No RawData
	// either: Data is the resource's configuration as parsed JSON, which is
	// what the analyzers walk.
	return cubapi.ListResources(ctx, c,
		cubapi.NewWhere(resourceTypeWhere).In("UnitID", unitIDs), cubapi.ListOpts{})
}
