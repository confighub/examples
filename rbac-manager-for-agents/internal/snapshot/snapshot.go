// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of Kubernetes RBAC config from
// ConfigHub by shelling out to cub. It discovers every Kubernetes/YAML Unit the
// user can view (optionally narrowed by scope filters), extracts just the RBAC
// resources server-side via the get-resources function, and joins them with
// Unit / Space / Target metadata into the rbac analysis model.
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

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
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

// --- cub JSON shapes (only the fields we read) ---

type extUnit struct {
	Unit struct {
		UnitID                string            `json:"UnitID"`
		Slug                  string            `json:"Slug"`
		SpaceID               string            `json:"SpaceID"`
		TargetID              string            `json:"TargetID"`
		Labels                map[string]string `json:"Labels"`
		ApplyGates            map[string]any    `json:"ApplyGates"`
		HeadRevisionNum       int64             `json:"HeadRevisionNum"`
		LiveRevisionNum       int64             `json:"LiveRevisionNum"`
		UpstreamRevisionNum   int64             `json:"UpstreamRevisionNum"`
		LastChangeDescription string            `json:"LastChangeDescription"`
	} `json:"Unit"`
	Space struct {
		SpaceID string            `json:"SpaceID"`
		Slug    string            `json:"Slug"`
		Labels  map[string]string `json:"Labels"`
	} `json:"Space"`
	Target struct {
		TargetID string `json:"TargetID"`
		Slug     string `json:"Slug"`
	} `json:"Target"`
}

type funcResp struct {
	Success   bool              `json:"Success"`
	UnitID    string            `json:"UnitID"`
	SpaceID   string            `json:"SpaceID"`
	SpaceSlug string            `json:"SpaceSlug"`
	UnitSlug  string            `json:"UnitSlug"`
	Outputs   map[string]string `json:"Outputs"`
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

// Load fetches and assembles the fleet snapshot.
func Load(ctx context.Context, scope Scope) (*Snapshot, error) {
	// Fetch unit metadata and both resource invocations concurrently.
	var (
		wg                 sync.WaitGroup
		units              []extUnit
		rbacResps, saResps []funcResp
		unitsErr, rbacErr  error
		saErr              error
	)
	wg.Add(3)
	go func() { defer wg.Done(); units, unitsErr = listUnits(ctx) }()
	go func() { defer wg.Done(); rbacResps, rbacErr = getResources(ctx, rbacWhereData, rbacWhereResource) }()
	go func() { defer wg.Done(); saResps, saErr = getResources(ctx, saWhereData, saWhereResource) }()
	wg.Wait()
	for _, e := range []struct {
		what string
		err  error
	}{{"list units", unitsErr}, {"get RBAC resources", rbacErr}, {"get ServiceAccounts", saErr}} {
		if e.err != nil {
			return nil, fmt.Errorf("%s: %w", e.what, e.err)
		}
	}

	scopedSpaceIDs, scopedTargetIDs, err := resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}

	// Build in-scope unit map. Scope rule: targeted units are in scope iff
	// their Target matches the target filter; untargeted (base) units iff their
	// Space matches the space filter.
	inScope := make(map[string]UnitMeta, len(units))
	for _, eu := range units {
		if eu.Unit.UnitID == "" {
			continue
		}
		var ok bool
		if eu.Unit.TargetID != "" {
			ok = scopedTargetIDs == nil || scopedTargetIDs[eu.Unit.TargetID]
		} else {
			ok = scopedSpaceIDs == nil || scopedSpaceIDs[eu.Unit.SpaceID]
		}
		if !ok {
			continue
		}
		inScope[eu.Unit.UnitID] = UnitMeta{
			UnitID:                eu.Unit.UnitID,
			Slug:                  eu.Unit.Slug,
			SpaceID:               eu.Unit.SpaceID,
			SpaceSlug:             eu.Space.Slug,
			SpaceLabels:           eu.Space.Labels,
			TargetID:              eu.Unit.TargetID,
			TargetSlug:            eu.Target.Slug,
			Labels:                eu.Unit.Labels,
			GateCount:             len(eu.Unit.ApplyGates),
			HeadRevisionNum:       eu.Unit.HeadRevisionNum,
			LiveRevisionNum:       eu.Unit.LiveRevisionNum,
			UpstreamRevisionNum:   eu.Unit.UpstreamRevisionNum,
			LastChangeDescription: eu.Unit.LastChangeDescription,
		}
	}

	var resources []rbac.FleetResource
	collect := func(resps []funcResp) {
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

func listUnits(ctx context.Context) ([]extUnit, error) {
	out, err := cub.Run(ctx, "unit", "list", "--space", "*", "--where", k8sUnitsWhere, "-o", "json")
	if err != nil {
		return nil, err
	}
	return decodeJSON[[]extUnit](out)
}

func getResources(ctx context.Context, whereData, whereResource string) ([]funcResp, error) {
	out, err := cub.Run(ctx, "function", "do", "--space", "*",
		"--where", k8sUnitsWhere,
		"--where-data", whereData,
		"--where-resource", whereResource,
		"--toolchain", "Kubernetes/YAML",
		"get-resources", "json", "-o", "json")
	if err != nil {
		return nil, err
	}
	return decodeJSON[[]funcResp](out)
}

// resolveScope returns the in-scope Space and Target ID sets, or nil sets when
// the corresponding filter is empty (meaning "everything").
func resolveScope(ctx context.Context, scope Scope) (spaceIDs, targetIDs map[string]bool, err error) {
	if scope.SpaceWhere != "" {
		spaceIDs, err = idSet(ctx, []string{"space", "list", "--where", scope.SpaceWhere, "-o", "json"}, "SpaceID")
		if err != nil {
			return nil, nil, fmt.Errorf("scope space filter: %w", err)
		}
	}
	if scope.TargetWhere != "" {
		targetIDs, err = idSet(ctx, []string{"target", "list", "--space", "*", "--where", scope.TargetWhere, "-o", "json"}, "TargetID")
		if err != nil {
			return nil, nil, fmt.Errorf("scope target filter: %w", err)
		}
	}
	return spaceIDs, targetIDs, nil
}

// idSet runs a cub list command and collects the given ID field from each
// element, looking inside an embedded entity wrapper when present.
func idSet(ctx context.Context, args []string, idField string) (map[string]bool, error) {
	out, err := cub.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	rows, err := decodeJSON[[]map[string]json.RawMessage](out)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(rows))
	for _, row := range rows {
		// The element may be {"<Entity>": {"<idField>": ...}} or flat.
		for _, raw := range row {
			var obj map[string]any
			if json.Unmarshal(raw, &obj) == nil {
				if id, ok := obj[idField].(string); ok && id != "" {
					set[id] = true
				}
			}
		}
		if v, ok := row[idField]; ok {
			var id string
			if json.Unmarshal(v, &id) == nil && id != "" {
				set[id] = true
			}
		}
	}
	return set, nil
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

func decodeJSON[T any](s string) (T, error) {
	var v T
	if s == "" {
		return v, nil
	}
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
