// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package snapshot

import (
	"strings"
	"testing"
)

func TestIsCanonicalSpace(t *testing.T) {
	cases := []struct {
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
	}
	for _, c := range cases {
		if got := isCanonicalSpace(c.labels); got != c.want {
			t.Errorf("isCanonicalSpace(%v) = %v, want %v", c.labels, got, c.want)
		}
	}
}

func TestUnitMetaState(t *testing.T) {
	cases := []struct {
		name      string
		u         UnitMeta
		gated     bool
		unapplied bool
	}{
		{"never applied", UnitMeta{HeadRevisionNum: 3, LiveRevisionNum: 0}, false, true},
		{"behind", UnitMeta{HeadRevisionNum: 5, LiveRevisionNum: 4}, false, true},
		{"in sync", UnitMeta{HeadRevisionNum: 5, LiveRevisionNum: 5}, false, false},
		{"gated", UnitMeta{HeadRevisionNum: 5, LiveRevisionNum: 5, GateCount: 2}, true, false},
	}
	for _, c := range cases {
		if got := c.u.Gated(); got != c.gated {
			t.Errorf("%s: Gated() = %v, want %v", c.name, got, c.gated)
		}
		if got := c.u.Unapplied(); got != c.unapplied {
			t.Errorf("%s: Unapplied() = %v, want %v", c.name, got, c.unapplied)
		}
	}
}

func TestResourceTypeWhere(t *testing.T) {
	// A filter string literal admits no quote or backslash, so a type name
	// carrying one could not be sent at all.
	for _, rt := range resourceTypes {
		if strings.ContainsAny(rt, "'\"\\") {
			t.Errorf("%q cannot appear in a filter literal", rt)
		}
		if !strings.Contains(resourceTypeWhere, "'"+rt+"'") {
			t.Errorf("%q missing from the IN clause", rt)
		}
	}
	if !strings.HasPrefix(resourceTypeWhere, "ResourceType IN ('") ||
		!strings.HasSuffix(resourceTypeWhere, "')") {
		t.Errorf("not a well-formed IN clause: %s", resourceTypeWhere)
	}
}
