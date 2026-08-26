// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package fleet

import (
	"context"
	"fmt"
	"strings"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// K8sUnitsWhere selects the Units whose configuration is Kubernetes YAML. It is
// the usual value for Loader.UnitWhere.
const K8sUnitsWhere = "ToolchainType = 'Kubernetes/YAML'"

// ClusterNone is the cluster key for Units the fleet view cannot attribute to a
// cluster: their Space has no release Target, so there is nothing to name. They
// group under one bucket rather than each Space standing in for a cluster of its
// own, which would count things that are not clusters.
const ClusterNone = "None"

// maxFilterLength mirrors the server's cap on a filter expression. Going over it
// is rejected with a 400, so a clause that grows with the size of the fleet has
// to be optional.
const maxFilterLength = 8192

// unitInclude expands the two related entities the snapshot cannot read off the
// Unit row. Space is included for its Labels, which mark a canonical base/policy
// Space -- not for its slug, which the Unit carries as SpaceSlug. Target is
// included for its slug, the cluster key, which the Unit has no field for.
//
// Fetching Spaces and Targets separately instead was measured and is slower at
// this shape: two more round trips cost more than the joins do over hundreds of
// Unit rows.
const unitInclude = "SpaceID,TargetID"

// unitSelectFields are the Unit fields UnitMeta carries. Naming them keeps a
// fleet-wide list from serializing every column of every Unit, which is most of
// what a snapshot costs.
const unitSelectFields = "UnitID,SpaceID,SpaceSlug,Slug,TargetID,Labels,ApplyGates,ApplyWarnings," +
	"HeadRevisionNum,LastAppliedRevisionNum,LiveRevisionNum,UpstreamRevisionNum,LastChangeDescription"

// resourceOrderBy makes the fetch reproducible. An unordered query comes back in
// "the database's default order", which the API documents as no promise at all,
// and a caller's own sorts tie-break on the order they were handed. ResourceID
// is the primary key, so ordering by it alone is a total order.
var resourceOrderBy = "ResourceID"

// Origin is where one resource came from: the ConfigHub entities that hold it
// and the cluster the fleet view attributes it to.
type Origin struct {
	// Cluster is the key resources are grouped by, ClusterNone when the Unit's
	// Space has no release Target.
	Cluster string
	// Target is the Unit's Target slug, empty when it has none.
	Target       string
	Space        string
	SpaceID      string
	SpaceLabels  map[string]string
	UnitID       string
	UnitSlug     string
	ResourceName string
	ResourceType string
	// Canonical marks a definition in a base or policy Space: shown in
	// inventories, kept out of cluster analysis.
	Canonical bool
}

// UnitMeta is the per-Unit metadata a snapshot joins onto resources. It also
// stands on its own: a Unit holding none of the resource types a tool asked for
// still appears here, which is what lets a fleet inventory count Units the
// resource query cannot see.
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
	WarningCount           int               `json:"warningCount,omitempty"`
	HeadRevisionNum        int64             `json:"headRevisionNum"`
	LastAppliedRevisionNum int64             `json:"lastAppliedRevisionNum,omitempty"`
	LiveRevisionNum        int64             `json:"liveRevisionNum"`
	UpstreamRevisionNum    int64             `json:"upstreamRevisionNum,omitempty"`
	LastChangeDescription  string            `json:"lastChangeDescription,omitempty"`
}

// Gated reports whether the Unit has any ApplyGates attached.
func (u UnitMeta) Gated() bool { return u.GateCount > 0 }

// Unapplied reports whether the Unit's head revision has not been applied live.
func (u UnitMeta) Unapplied() bool {
	return u.LiveRevisionNum == 0 || u.LiveRevisionNum < u.HeadRevisionNum
}

// Snapshot is one fleet-wide read: every resource a tool asked for, and the
// metadata of every Unit in scope.
type Snapshot[R any] struct {
	// Resources is every resource that was read, canonical ones included.
	Resources []R
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the Unit `where` predicate the snapshot was scoped by; empty
	// means the whole fleet the caller can view.
	Filter string `json:"filter,omitempty"`
}

// IsCanonicalSpace reports whether a Space holds definitions rather than
// deployed config. The standard Variant=base label marks a base or template
// Space; a `role` label of base or policy is treated the same way.
func IsCanonicalSpace(labels map[string]string) bool {
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

// DefaultClusterKey names the cluster a Unit's resources belong to: its Target
// slug, or ClusterNone when its Space has no release Target.
func DefaultClusterKey(meta UnitMeta) string {
	if meta.TargetSlug != "" {
		return meta.TargetSlug
	}
	return ClusterNone
}

// Loader reads a fleet snapshot. The zero value is not usable: New is required,
// and so is one of ResourceTypes or ResourceWhere.
type Loader[R any] struct {
	// ResourceTypes are the exact ResourceTypes to read, e.g.
	// "apps/v1/Deployment". They become one IN clause, which is how a union of
	// values is written in a filter language that has no OR. Naming versions
	// exactly means a new API version has to be added here.
	ResourceTypes []string

	// ResourceWhere replaces ResourceTypes with a raw clause, for a set that
	// cannot be enumerated -- an API group whose kinds are open-ended, say.
	// Prefer ResourceTypes: an exact list needs no pattern and no re-matching.
	ResourceWhere string

	// UnitWhere scopes which Units are in scope at all, ANDed with the caller's
	// predicate. Defaults to K8sUnitsWhere.
	UnitWhere string

	// ClusterKey names the cluster a Unit's resources belong to. Defaults to
	// DefaultClusterKey. Override it where the cluster a tool reports is not the
	// Target the config is delivered to.
	ClusterKey func(UnitMeta) string

	// Canonical reports whether a Space holds definitions rather than deployed
	// config. Defaults to IsCanonicalSpace.
	Canonical func(spaceLabels map[string]string) bool

	// Keep, when set, decides whether a resource that came back is kept. Use it
	// where ResourceWhere is broader than the set actually wanted: a filter
	// literal cannot carry a backslash, so a pattern's dots match any character
	// and the clause can only be written broader than intended, never narrower.
	Keep func(Origin) bool

	// New builds the caller's own resource from its origin and its
	// configuration, which arrives as already-parsed JSON. It is called once per
	// resource read, in the order the query returned them.
	New func(Origin, map[string]any) R
}

func (l Loader[R]) unitWhere() string {
	if l.UnitWhere != "" {
		return l.UnitWhere
	}
	return K8sUnitsWhere
}

func (l Loader[R]) clusterKey(meta UnitMeta) string {
	if l.ClusterKey != nil {
		return l.ClusterKey(meta)
	}
	return DefaultClusterKey(meta)
}

func (l Loader[R]) canonical(labels map[string]string) bool {
	if l.Canonical != nil {
		return l.Canonical(labels)
	}
	return IsCanonicalSpace(labels)
}

// resourceWhere is the clause selecting the resource types to read.
func (l Loader[R]) resourceWhere() (string, error) {
	if l.ResourceWhere != "" {
		return l.ResourceWhere, nil
	}
	if len(l.ResourceTypes) == 0 {
		return "", fmt.Errorf("fleet: loader needs ResourceTypes or ResourceWhere")
	}
	for _, t := range l.ResourceTypes {
		// A filter string literal admits no quote or backslash, so a type name
		// carrying one could not be sent at all.
		if strings.ContainsAny(t, "'\"\\") {
			return "", fmt.Errorf("fleet: resource type %q cannot appear in a filter literal", t)
		}
	}
	return "ResourceType IN ('" + strings.Join(l.ResourceTypes, "', '") + "')", nil
}

// Load reads the snapshot, scoped by a single ConfigHub Unit `where` predicate;
// empty means everything the caller can view. The predicate may reference Unit,
// Space and Target metadata, and it scopes the Unit list server-side.
func (l Loader[R]) Load(ctx context.Context, c *cubapi.Client, where string) (*Snapshot[R], error) {
	if l.New == nil {
		return nil, fmt.Errorf("fleet: loader needs New")
	}
	resourceWhere, err := l.resourceWhere()
	if err != nil {
		return nil, err
	}

	// ConfigHub `where` is flat AND-only (no parentheses), so clauses are joined
	// with a bare AND.
	unitWhere := l.unitWhere()
	if where != "" {
		unitWhere += " AND " + where
	}

	units, err := cubapi.ListUnits(ctx, c, cubapi.NewWhere(unitWhere),
		cubapi.ListOpts{Include: unitInclude, Select: unitSelectFields})
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}

	inScope := make(map[string]UnitMeta, len(units))
	unitIDs := make([]goclientnew.UUID, 0, len(units))
	for _, eu := range units {
		if eu.Unit == nil || eu.Unit.UnitID == (goclientnew.UUID{}) {
			continue
		}
		meta := unitMeta(eu)
		inScope[meta.UnitID] = meta
		unitIDs = append(unitIDs, eu.Unit.UnitID)
	}

	extended, err := listResources(ctx, c, resourceWhere, unitIDs)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	resources := make([]R, 0, len(extended))
	for _, er := range extended {
		if er.Resource == nil || er.Resource.Data == nil {
			continue
		}
		r := er.Resource
		meta, ok := inScope[r.UnitID.String()]
		if !ok {
			continue // out of scope
		}
		space := r.SpaceSlug
		if space == "" {
			space = meta.SpaceSlug
		}
		origin := Origin{
			Cluster:      l.clusterKey(meta),
			Target:       meta.TargetSlug,
			Space:        space,
			SpaceID:      r.SpaceID.String(),
			SpaceLabels:  meta.SpaceLabels,
			UnitID:       r.UnitID.String(),
			UnitSlug:     firstNonEmpty(r.UnitSlug, meta.Slug),
			ResourceName: r.ResourceName,
			ResourceType: r.ResourceType,
			Canonical:    l.canonical(meta.SpaceLabels),
		}
		if l.Keep != nil && !l.Keep(origin) {
			continue
		}
		resources = append(resources, l.New(origin, r.Data))
	}

	return &Snapshot[R]{Resources: resources, Units: inScope, Filter: where}, nil
}

// listResources reads the resources inside the in-scope Units from the Resource
// entity, which mirrors the configuration in each Unit's data and is queried in
// SQL.
//
// The Units are named by ID rather than by re-sending the caller's predicate:
// that predicate selects Units and is written against Unit attributes, which the
// resource query would need re-spelled with a `Unit.` prefix, and the IDs are
// already in hand from the Unit list a snapshot needs anyway.
func listResources(ctx context.Context, c *cubapi.Client, resourceWhere string, unitIDs []goclientnew.UUID) ([]*goclientnew.ExtendedResource, error) {
	if len(unitIDs) == 0 {
		return nil, nil
	}
	where := cubapi.NewWhere(K8sUnitsWhere).And(resourceWhere)

	// Naming the in-scope Units keeps the server from sending resources that
	// would only be discarded, but it is an optimization and nothing more: scope
	// is enforced by the join onto the Unit metadata. So the clause goes in only
	// when it fits under the server's filter-length cap -- a fleet-wide run names
	// more Units than 8192 characters hold, and asking anyway is a 400, not a
	// truncated answer.
	if scoped := where.In("UnitID", unitIDs); len(scoped.String()) <= maxFilterLength {
		where = scoped
	}

	// No Include: the Space and Unit slugs are columns on the row, and the Target
	// slug comes from the Unit metadata already loaded. No RawData either: Data
	// is the resource's configuration as parsed JSON, which is what a caller
	// walks.
	return cubapi.ListResources(ctx, c, where, cubapi.ListOpts{},
		func(p *goclientnew.ListAllResourcesParams) { p.OrderBy = &resourceOrderBy })
}

func unitMeta(eu *goclientnew.ExtendedUnit) UnitMeta {
	meta := UnitMeta{
		UnitID:                 eu.Unit.UnitID.String(),
		Slug:                   eu.Unit.Slug,
		SpaceID:                eu.Unit.SpaceID.String(),
		SpaceSlug:              eu.Unit.SpaceSlug,
		Labels:                 eu.Unit.Labels,
		GateCount:              len(eu.Unit.ApplyGates),
		WarningCount:           len(eu.Unit.ApplyWarnings),
		HeadRevisionNum:        eu.Unit.HeadRevisionNum,
		LastAppliedRevisionNum: eu.Unit.LastAppliedRevisionNum,
		LiveRevisionNum:        eu.Unit.LiveRevisionNum,
		UpstreamRevisionNum:    eu.Unit.UpstreamRevisionNum,
		LastChangeDescription:  eu.Unit.LastChangeDescription,
	}
	if eu.Unit.TargetID != nil {
		meta.TargetID = eu.Unit.TargetID.String()
	}
	if eu.Space != nil {
		meta.SpaceLabels = eu.Space.Labels
	}
	if eu.Target != nil {
		meta.TargetSlug = eu.Target.Slug
	}
	return meta
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
