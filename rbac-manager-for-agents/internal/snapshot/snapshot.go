// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of Kubernetes RBAC config from
// ConfigHub via the API. It discovers every Kubernetes/YAML Unit the user can
// view (optionally narrowed by scope filters), extracts just the RBAC resources
// server-side via the get-resources function, and joins them with Unit / Space /
// Target metadata into the rbac analysis model.
//
// This is the Go port of the web app's fleet snapshot loader. Two get-resources
// invocations run in parallel because a single WhereResource conjunction can't
// express "rbac.authorization.k8s.io/* OR v1/ServiceAccount".
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

	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

const (
	k8sUnitsWhere     = "ToolchainType = 'Kubernetes/YAML'"
	rbacWhereData     = "kind IN ('Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding')"
	rbacWhereResource = "ConfigHub.ResourceType LIKE 'rbac.authorization.k8s.io/%'"
	saWhereData       = "kind = 'ServiceAccount'"
	saWhereResource   = "ConfigHub.ResourceType = 'v1/ServiceAccount'"
)

// Scope narrows the fleet. Both are ConfigHub filter expressions (the same
// `where` syntax cub uses); empty means everything the user can view. Targets
// scope deployed Units (Clusters are Targets); Spaces scope untargeted base
// Units.
type Scope struct {
	TargetWhere string
	SpaceWhere  string
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
	Scope Scope
}

type rawResource struct {
	ResourceType string `json:"ResourceType"`
	ResourceName string `json:"ResourceName"`
	ResourceBody string `json:"ResourceBody"`
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

// Load fetches and assembles the fleet snapshot using the given API client.
func Load(ctx context.Context, c *cubapi.Client, scope Scope) (*Snapshot, error) {
	// Fetch unit metadata and both resource invocations concurrently.
	var (
		wg                 sync.WaitGroup
		units              []*goclientnew.ExtendedUnit
		rbacResps, saResps []cubapi.UnitOutcome
		unitsErr, rbacErr  error
		saErr              error
	)
	wg.Add(3)
	go func() { defer wg.Done(); units, unitsErr = listUnits(ctx, c) }()
	go func() { defer wg.Done(); rbacResps, rbacErr = getResources(ctx, c, rbacWhereData, rbacWhereResource) }()
	go func() { defer wg.Done(); saResps, saErr = getResources(ctx, c, saWhereData, saWhereResource) }()
	wg.Wait()
	for _, e := range []struct {
		what string
		err  error
	}{{"list units", unitsErr}, {"get RBAC resources", rbacErr}, {"get ServiceAccounts", saErr}} {
		if e.err != nil {
			return nil, fmt.Errorf("%s: %w", e.what, e.err)
		}
	}

	scopedSpaceIDs, scopedTargetIDs, err := resolveScope(ctx, c, scope)
	if err != nil {
		return nil, err
	}

	// Build in-scope unit map. Scope rule: targeted units are in scope iff
	// their Target matches the target filter; untargeted (base) units iff their
	// Space matches the space filter.
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
		var ok bool
		if targetID != "" {
			ok = scopedTargetIDs == nil || scopedTargetIDs[targetID]
		} else {
			ok = scopedSpaceIDs == nil || scopedSpaceIDs[spaceID]
		}
		if !ok {
			continue
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

	var resources []rbac.FleetResource
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
				resources = append(resources, rbac.FleetResource{
					Origin: rbac.ResourceOrigin{
						Cluster:      cluster,
						Target:       meta.TargetSlug,
						Space:        space,
						SpaceID:      r.SpaceID,
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
	collect(rbacResps)
	collect(saResps)

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
		Scope:     scope,
	}, nil
}

// listUnits returns every Kubernetes/YAML Unit org-wide, with Space and Target
// expanded so the snapshot can join their slugs and labels.
func listUnits(ctx context.Context, c *cubapi.Client) ([]*goclientnew.ExtendedUnit, error) {
	return cubapi.ListUnits(ctx, c, cubapi.NewWhere(k8sUnitsWhere),
		cubapi.ListOpts{Include: "SpaceID,TargetID"})
}

// getResources runs the get-resources function over the matching Units and
// returns the per-Unit outcomes (the resource list lands in Outputs).
func getResources(ctx context.Context, c *cubapi.Client, whereData, whereResource string) ([]cubapi.UnitOutcome, error) {
	res, err := cubapi.InvokeFunction(ctx, c,
		api.FunctionInvocation{FunctionName: "get-resources", Arguments: []api.FunctionArgument{{Value: "json"}}},
		cubapi.Selector{
			Where:         k8sUnitsWhere,
			WhereData:     whereData,
			WhereResource: whereResource,
		},
		cubapi.Change{}) // read-only: dry-run
	if err != nil {
		return nil, err
	}
	return res.Outcomes, nil
}

// resolveScope returns the in-scope Space and Target ID sets, or nil sets when
// the corresponding filter is empty (meaning "everything").
func resolveScope(ctx context.Context, c *cubapi.Client, scope Scope) (spaceIDs, targetIDs map[string]bool, err error) {
	if scope.SpaceWhere != "" {
		spaces, err := cubapi.ListSpaces(ctx, c, cubapi.NewWhere(scope.SpaceWhere), cubapi.ListOpts{Select: "SpaceID,Slug"})
		if err != nil {
			return nil, nil, fmt.Errorf("scope space filter: %w", err)
		}
		spaceIDs = make(map[string]bool, len(spaces))
		for _, es := range spaces {
			if es.Space != nil {
				spaceIDs[es.Space.SpaceID.String()] = true
			}
		}
	}
	if scope.TargetWhere != "" {
		targets, err := cubapi.ListTargets(ctx, c, cubapi.NewWhere(scope.TargetWhere), cubapi.ListOpts{Select: "TargetID,Slug"})
		if err != nil {
			return nil, nil, fmt.Errorf("scope target filter: %w", err)
		}
		targetIDs = make(map[string]bool, len(targets))
		for _, et := range targets {
			if et.Target != nil {
				targetIDs[et.Target.TargetID.String()] = true
			}
		}
	}
	return spaceIDs, targetIDs, nil
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
