// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of Kubernetes RBAC config from
// ConfigHub via the API. It discovers every Kubernetes/YAML Unit the user can
// view (optionally narrowed by a single Unit `where` filter), reads just the
// RBAC resources inside them from the Resource entity, and joins them with Unit
// / Space / Target metadata into the rbac analysis model.
//
// This is the Go port of the web app's fleet snapshot loader.//
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

	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

const (
	k8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"
)

// resourceTypePatterns match the ResourceTypes the RBAC model needs: the roles
// and bindings themselves, and the ServiceAccounts a binding can name.
var resourceTypePatterns = []string{
	`rbac\.authorization\.k8s\.io/[^/]+/(Role|ClusterRole|RoleBinding|ClusterRoleBinding)`,
	`v1/ServiceAccount`,
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

// ClusterNone is the cluster key for Units the fleet view cannot attribute to
// a cluster: their Space has no release Target, so there is nothing to name.
// They group under one bucket rather than each Space standing in for a cluster
// of its own, which inflated the cluster count with things that are not clusters.
const ClusterNone = "None"

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

// Snapshot is a fleet-wide RBAC view.
type Snapshot struct {
	// Clusters holds RBAC entities per cluster, excluding canonical (base/
	// policy) definitions — nothing deploys there, so they'd produce phantom
	// grants and findings. Keyed by Target slug (Space slug for unbound Units).
	Clusters map[string]*rbac.ClusterRbac
	// Resources is every parsed RBAC/ServiceAccount resource, including
	// canonical ones, for the explorer.
	Resources []rbac.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped
	// by (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

// Canonical Spaces hold definitions, not deployed config, so their Units stay
// out of cluster analysis. The standard Variant=base label marks a base/
// template Space; the demo fleet additionally uses a `role` label.
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
// it scopes the Unit list server-side, and the resource query is narrowed to the
// Units it returned.
func Load(ctx context.Context, c *cubapi.Client, where string) (*Snapshot, error) {
	// ConfigHub `where` is flat AND-only (no parentheses), so clauses are joined
	// with a bare AND.
	unitWhere := k8sUnitsWhere
	if where != "" {
		unitWhere = k8sUnitsWhere + " AND " + where
	}
	units, err := listUnits(ctx, c, unitWhere)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}

	// The server has already scoped the Units to the predicate; build metadata
	// for every returned Unit and join resources onto it by UnitID.
	inScope := make(map[string]UnitMeta, len(units))
	unitIDs := make([]goclientnew.UUID, 0, len(units))
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
		unitIDs = append(unitIDs, eu.Unit.UnitID)
	}

	extended, err := listResources(ctx, c, unitIDs)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	var resources []rbac.FleetResource
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
		resources = append(resources, rbac.FleetResource{
			Origin: rbac.ResourceOrigin{
				Cluster:      cluster,
				Target:       meta.TargetSlug,
				Space:        space,
				SpaceID:      r.SpaceID.String(),
				UnitID:       r.UnitID.String(),
				UnitSlug:     firstNonEmpty(r.UnitSlug, meta.Slug),
				ResourceName: r.ResourceName,
				Canonical:    isCanonicalSpace(meta.SpaceLabels),
			},
			Doc: r.Data,
		})
	}

	// Canonical definitions stay out of cluster analysis.
	var forAnalysis []rbac.FleetResource
	for _, r := range resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}

	return &Snapshot{
		Clusters:  rbac.BuildClusterRbac(forAnalysis),
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

func isZeroUUID(id goclientnew.UUID) bool {
	return id == goclientnew.UUID{}
}

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
