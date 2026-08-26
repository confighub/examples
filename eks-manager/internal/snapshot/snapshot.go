// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads a fleet-wide view of the Crossplane managed resources
// that make up EKS clusters from ConfigHub via the API. It discovers every
// Kubernetes/YAML Unit the user can view (optionally narrowed by scope filters),
// reads the eks / ec2 / iam managed resources inside them from the Resource
// entity, and joins them with Unit / Space / Target metadata into the EKS
// analysis model.
//
// The groups arrive in one query: Resources are queried in SQL, so the API
// group is a predicate the database evaluates rather than a function invoked
// over every Unit, and each resource's configuration comes back as
// already-parsed JSON.
package snapshot

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/eks-manager/internal/eks"
)

const k8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"

// ClusterNone is the cluster key for Units the fleet view cannot attribute to a
// cluster. They group under one bucket rather than each Space standing in for a
// cluster of its own, which inflated the cluster count with things that are not
// clusters.
const ClusterNone = "None"

// SpaceLabelCluster is the Space label naming the EKS cluster a Space describes.
// A cluster is a Space here (its Units describe a cluster rather than deploy to
// one), so this label — not the Target — identifies the cluster.
const SpaceLabelCluster = "Cluster"

// maxFilterLength mirrors the server's cap on a filter expression. Going over it
// is rejected with a 400, so a clause that grows with the size of the fleet has
// to be optional.
const maxFilterLength = 8192

// unitInclude expands the two related entities the snapshot cannot read off the
// Unit row. Space is included for its Labels, which mark a canonical base/policy
// Space -- not for its slug, which the Unit carries as SpaceSlug. Target is
// included for its slug, the cluster key, which the Unit has no field for.
const unitInclude = "SpaceID,TargetID"

// unitSelectFields are the Unit fields UnitMeta carries. Naming them keeps a
// fleet-wide list from serializing every column of every Unit; it is the bulk of
// what the snapshot costs.
const unitSelectFields = "UnitID,SpaceID,SpaceSlug,Slug,TargetID,Labels,ApplyGates,ApplyWarnings," +
	"HeadRevisionNum,LastAppliedRevisionNum,LiveRevisionNum," +
	"UpstreamRevisionNum,LastChangeDescription"

// resourceOrderBy makes the fetch reproducible. An unordered query comes back in
// "the database's default order", which is not a promise, and the analyzers'
// own sorts tie-break on the order they were handed. ResourceID is the primary
// key, so ordering by it alone is total -- and order_by takes only one field:
// a comma-separated list is documented but reaches SQL as a single quoted
// identifier and fails with a 500.
var resourceOrderBy = "ResourceID"

// resourceTypePatterns match the ResourceTypes the EKS model needs, one per
// Crossplane API group.
//
// They key off the API group rather than an enumeration of kinds. That matters:
// a Crossplane provider ships hundreds of kinds per group and adds more every
// release, so a `kind IN (...)` list would silently go stale — whereas the group
// prefix picks up a new EKS kind for free. The model buckets whatever comes
// back, and unrecognized kinds land in the generic inventory rather than being
// dropped.
//
// Adding a group (say rds, for a sibling tool) is one line here.
var resourceTypePatterns = []string{
	`eks\.aws\.upbound\.io/.+`,
	`ec2\.aws\.upbound\.io/.+`,
	`iam\.aws\.upbound\.io/.+`,
}

// resourceTypeMatch is the exact test, applied to what comes back.
var resourceTypeMatch = regexp.MustCompile(`(?i)^(` + strings.Join(resourceTypePatterns, "|") + `)$`)

// resourceTypeWhere asks the server for the same union in one clause: the filter
// language is flat AND-only, so a union of groups is one regular expression
// rather than ORed prefixes. A filter literal cannot carry a backslash, so the
// escapes are dropped — an unescaped `.` matches any character, which makes the
// clause broader than the patterns, never narrower, and resourceTypeMatch
// narrows it again on the way out.
var resourceTypeWhere = "ResourceType ~* '^(" +
	strings.ReplaceAll(strings.Join(resourceTypePatterns, "|"), `\`, "") + ")$'"

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
// Cluster label when set, else ClusterNone. Deliberately NOT the Target slug
// — the Target is the Crossplane management cluster these resources are applied
// to, which is a different cluster from the one they describe.
//
// A Space carrying no Cluster label describes no cluster this tool can name, so
// its Units group under ClusterNone rather than the Space slug standing in for
// a cluster of its own.
func clusterKey(meta UnitMeta) string {
	if v := meta.SpaceLabels[SpaceLabelCluster]; v != "" {
		return v
	}
	return ClusterNone
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

	// The server has already scoped the Units to the predicate; build metadata for
	// every returned Unit and join resources onto it by UnitID.
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
		spaceSlug := eu.Unit.SpaceSlug
		var spaceLabels map[string]string
		if eu.Space != nil {
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

	extended, err := listResources(ctx, c, unitIDs)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	// One query returns each resource once, so there is nothing to deduplicate.
	// The per-group invocations it replaces could return the same resource twice
	// when groups overlapped, and the guard against that keyed on
	// unitID|resourceName -- which also dropped a second resource that merely
	// shared a name with the first.
	var resources []eks.FleetResource
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
		resources = append(resources, eks.FleetResource{
			Origin: eks.ResourceOrigin{
				Cluster:      clusterKey(meta),
				Space:        space,
				SpaceID:      r.SpaceID.String(),
				SpaceLabels:  meta.SpaceLabels,
				Target:       meta.TargetSlug,
				UnitID:       r.UnitID.String(),
				UnitSlug:     firstNonEmpty(r.UnitSlug, meta.Slug),
				ResourceName: r.ResourceName,
				Canonical:    isCanonicalSpace(meta.SpaceLabels),
			},
			Doc: r.Data,
		})
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
		cubapi.ListOpts{Include: unitInclude, Select: unitSelectFields})
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
	where := cubapi.NewWhere(k8sUnitsWhere).And(resourceTypeWhere)

	// Naming the in-scope Units keeps the server from sending resources that
	// would only be discarded, but it is an optimization and nothing more: scope
	// is enforced where each resource is joined back onto the Unit metadata. So
	// the clause goes in only when it fits under the server's filter-length cap
	// -- a fleet-wide run names more Units than 8192 characters hold, and asking
	// anyway is a 400, not a truncated answer.
	if scoped := where.In("UnitID", unitIDs); len(scoped.String()) <= maxFilterLength {
		where = scoped
	}

	// No Include: the Space and Unit slugs are columns on the row, and the
	// Target slug comes from the Unit metadata already loaded. No RawData
	// either: Data is the resource's configuration as parsed JSON, which is
	// what the analyzers walk.
	return cubapi.ListResources(ctx, c, where, cubapi.ListOpts{},
		func(p *goclientnew.ListAllResourcesParams) { p.OrderBy = &resourceOrderBy })
}
