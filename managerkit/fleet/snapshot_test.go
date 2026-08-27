// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package fleet

import (
	"strings"
	"testing"
)

func TestResourceWhere(t *testing.T) {
	l := Loader[any]{ResourceTypes: []string{"v1/Pod", "apps/v1/Deployment"}}
	got, err := l.resourceWhere()
	if err != nil {
		t.Fatalf("resourceWhere: %v", err)
	}
	want := "ResourceType IN ('v1/Pod', 'apps/v1/Deployment')"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A filter string literal admits no quote or backslash. Catching it here beats a
// 400 from the server with a query nobody can read.
func TestResourceWhereRejectsUnsendableType(t *testing.T) {
	for _, bad := range []string{`v1/Po'd`, `v1/Po\d`, `v1/Po"d`} {
		l := Loader[any]{ResourceTypes: []string{bad}}
		if _, err := l.resourceWhere(); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestResourceWhereOverride(t *testing.T) {
	l := Loader[any]{ResourceWhere: "ResourceType ~* '^eks'"}
	got, err := l.resourceWhere()
	if err != nil || got != "ResourceType ~* '^eks'" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestResourceWhereNeedsOne(t *testing.T) {
	if _, err := (Loader[any]{}).resourceWhere(); err == nil {
		t.Error("a loader with neither ResourceTypes nor ResourceWhere was accepted")
	}
}

func TestDefaultClusterKey(t *testing.T) {
	if got := DefaultClusterKey(UnitMeta{TargetSlug: "prod-oci"}); got != "prod-oci" {
		t.Errorf("bound Unit: got %q", got)
	}
	// A Space with no release Target names no cluster, and every such Unit shares
	// the one bucket rather than standing in for a cluster of its own.
	if got := DefaultClusterKey(UnitMeta{SpaceSlug: "some-space"}); got != ClusterNone {
		t.Errorf("unbound Unit: got %q, want %q", got, ClusterNone)
	}
}

func TestIsCanonicalSpace(t *testing.T) {
	for _, tc := range []struct {
		labels map[string]string
		want   bool
	}{
		{nil, false},
		{map[string]string{"Variant": "base"}, true},
		{map[string]string{"Variant": "prod"}, false},
		{map[string]string{"role": "base"}, true},
		{map[string]string{"role": "policy"}, true},
		{map[string]string{"role": "app"}, false},
		{map[string]string{"Environment": "prod"}, false},
	} {
		if got := IsCanonicalSpace(tc.labels); got != tc.want {
			t.Errorf("IsCanonicalSpace(%v) = %v, want %v", tc.labels, got, tc.want)
		}
	}
}

func TestUnitMetaState(t *testing.T) {
	for _, tc := range []struct {
		name       string
		u          UnitMeta
		gated      bool
		unreleased bool
	}{
		{"never released", UnitMeta{HeadRevisionNum: 3, LastReleasedRevisionNum: 0}, false, true},
		{"behind", UnitMeta{HeadRevisionNum: 5, LastReleasedRevisionNum: 4}, false, true},
		{"in sync", UnitMeta{HeadRevisionNum: 5, LastReleasedRevisionNum: 5}, false, false},
		{"gated", UnitMeta{HeadRevisionNum: 5, LastReleasedRevisionNum: 5, GateCount: 2}, true, false},
	} {
		if got := tc.u.Gated(); got != tc.gated {
			t.Errorf("%s: Gated() = %v, want %v", tc.name, got, tc.gated)
		}
		if got := tc.u.Unreleased(); got != tc.unreleased {
			t.Errorf("%s: Unreleased() = %v, want %v", tc.name, got, tc.unreleased)
		}
	}
}

// Every field UnitMeta reads off the Unit row has to be named in the select, or
// it comes back zero with nothing to indicate the value was never fetched.
func TestUnitSelectCoversUnitMeta(t *testing.T) {
	for _, field := range []string{
		"UnitID", "SpaceID", "SpaceSlug", "Slug", "TargetID", "Labels", "ApplyGates",
		"ApplyWarnings", "HeadRevisionNum", "LastReleasedRevisionNum",
		"UpstreamRevisionNum", "LastChangeDescription",
	} {
		if !strings.Contains(unitSelectFields, field) {
			t.Errorf("%s is read into UnitMeta but not selected", field)
		}
	}
}
