// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of the Crossplane managed resources
// that make up EKS clusters from ConfigHub via the API. It discovers every
// Kubernetes/YAML Unit the user can view (optionally narrowed by scope filters),
// extracts the eks / ec2 / iam managed resources server-side via the
// get-resources function, and joins them with Unit / Space / Target metadata
// into the EKS analysis model.
//
// Several get-resources invocations run in parallel because a single
// WhereResource conjunction can't express the union of API groups.
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

	"github.com/confighub/examples/eks-manager/internal/eks"
)

const k8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"

// SpaceLabelCluster is the Space label naming the EKS cluster a Space describes.
// A cluster is a Space here (its Units describe a cluster rather than deploy to
// one), so this label — not the Target — identifies the cluster.
const SpaceLabelCluster = "Cluster"

// resourceQuery is one (whereData, whereResource) pair driving a get-resources
// invocation. whereData narrows Units server-side by what their config carries;
// whereResource selects which resource types get returned.
type resourceQuery struct {
	name          string
	whereData     string
	whereResource string
}

// resourceQueries enumerate the Crossplane API groups the EKS model needs, one
// per group, run as parallel get-resources invocations.
//
// Both clauses key off the API group rather than an enumeration of kinds. That
// matters: a Crossplane provider ships hundreds of kinds per group and adds more
// every release, so a `kind IN (...)` list would silently go stale — whereas
// `apiVersion LIKE 'eks.aws.upbound.io/%'` picks up a new EKS kind for free.
// The model buckets whatever comes back, and unrecognized kinds land in the
// generic inventory rather than being dropped.
//
// Adding a group (say rds, for a sibling tool) is one line here.
var resourceQueries = []resourceQuery{
	{"eks", "apiVersion LIKE 'eks.aws.upbound.io/%'", "ConfigHub.ResourceType LIKE 'eks.aws.upbound.io/%'"},
	{"ec2", "apiVersion LIKE 'ec2.aws.upbound.io/%'", "ConfigHub.ResourceType LIKE 'ec2.aws.upbound.io/%'"},
	{"iam", "apiVersion LIKE 'iam.aws.upbound.io/%'", "ConfigHub.ResourceType LIKE 'iam.aws.upbound.io/%'"},
}

// UnitMeta is the per-Unit metadata the snapshot joins onto resources.
type UnitMeta struct {
	UnitID                 string            `json:"unitId"`
	Slug                   string            `json:"slug"`
	SpaceID                string            `json:"spaceId"`
	SpaceSlug              string            `json:"spaceSlug"`
	SpaceLabels            map[string]string `json:"spaceLabels,omitempty"`
	TargetID               string            `json:"targetId,omitempty"`
	TargetSlug             string            `json:"targetSlug,omitempty"`
	Labels                 map[string]string `json:"labels,omitempty"`
	GateCount              int               `json:"gateCount"`
	WarningCount           int               `json:"warningCount"`
	HeadRevisionNum        int64             `json:"headRevisionNum"`
	LiveRevisionNum        int64             `json:"liveRevisionNum"`
	LastAppliedRevisionNum int64             `json:"lastAppliedRevisionNum,omitempty"`
	UpstreamRevisionNum    int64             `json:"upstreamRevisionNum,omitempty"`
	LastChangeDescription  string            `json:"lastChangeDescription,omitempty"`
}

// Gated reports whether the Unit has any ApplyGates attached.
func (u UnitMeta) Gated() bool { return u.GateCount > 0 }

// Unapplied reports whether the Unit's head revision has not been applied live.
func (u UnitMeta) Unapplied() bool {
	return u.LiveRevisionNum == 0 || u.LiveRevisionNum < u.HeadRevisionNum
}

// Snapshot is a fleet-wide view of the EKS clusters ConfigHub manages.
type Snapshot struct {
	// Clusters holds the managed resources per EKS cluster, excluding canonical
	// (base/policy) definitions. Keyed by the Space's Cluster label, falling back
	// to the Space slug.
	Clusters map[string]*eks.ClusterSet
	// Resources is every parsed resource, including canonical ones, for the
	// explorer.
	Resources []eks.FleetResource
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

// clusterKey names the EKS cluster a Unit's resources belong to: the Space's
// Cluster label when set, else the Space slug. Deliberately NOT the Target slug
// — the Target is the Crossplane management cluster these resources are applied
// to, which is a different cluster from the one they describe.
func clusterKey(meta UnitMeta, spaceSlug string) string {
	if v := meta.SpaceLabels[SpaceLabelCluster]; v != "" {
		return v
	}
	if spaceSlug != "" {
		return spaceSlug
	}
	return meta.SpaceSlug
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
			return nil, fmt.Errorf("get resources (%s): %w", resourceQueries[i].name, err)
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
			UnitID:                 unitID,
			Slug:                   eu.Unit.Slug,
			SpaceID:                eu.Unit.SpaceID.String(),
			SpaceSlug:              spaceSlug,
			SpaceLabels:            spaceLabels,
			TargetID:               targetID,
			TargetSlug:             targetSlug,
			Labels:                 eu.Unit.Labels,
			GateCount:              len(eu.Unit.ApplyGates),
			WarningCount:           len(eu.Unit.ApplyWarnings),
			HeadRevisionNum:        eu.Unit.HeadRevisionNum,
			LiveRevisionNum:        eu.Unit.LiveRevisionNum,
			LastAppliedRevisionNum: eu.Unit.LastAppliedRevisionNum,
			UpstreamRevisionNum:    eu.Unit.UpstreamRevisionNum,
			LastChangeDescription:  eu.Unit.LastChangeDescription,
		}
	}

	var resources []eks.FleetResource
	seen := map[string]bool{} // unitID|resourceName, in case groups overlap
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
			canonical := isCanonicalSpace(meta.SpaceLabels)
			for _, raw := range decodeResourceList(r.Outputs["ResourceList"]) {
				if raw.ResourceBody == "" {
					continue
				}
				key := r.UnitID + "|" + raw.ResourceName
				if seen[key] {
					continue
				}
				var doc any
				if err := json.Unmarshal([]byte(raw.ResourceBody), &doc); err != nil {
					continue
				}
				seen[key] = true
				resources = append(resources, eks.FleetResource{
					Origin: eks.ResourceOrigin{
						Cluster:      clusterKey(meta, space),
						Space:        space,
						SpaceID:      r.SpaceID,
						SpaceLabels:  meta.SpaceLabels,
						Target:       meta.TargetSlug,
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
	var forAnalysis []eks.FleetResource
	for _, r := range resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}

	return &Snapshot{
		Clusters:  eks.BuildFleet(forAnalysis),
		Resources: resources,
		Units:     inScope,
		Filter:    where,
	}, nil
}

// listUnits returns every Kubernetes/YAML Unit matching the given where
// predicate, with Space and Target expanded so the snapshot can join their slugs
// and labels.
//
// ListOpts.Select is deliberately left empty, which the API reads as "every
// field" (see cubapi.SelectFields, where the "*" wildcard normalizes to ""). Do
// not narrow it as an optimization: `plan` depends on LastAppliedRevisionNum,
// and a restricted field set returns it as null rather than erroring — the
// classifier would then silently treat every Unit as never-applied and grade
// nothing. Note this differs from the `cub unit list` CLI, where omitting
// --select yields only a small default field set.
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
