// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package snapshot

import (
	"regexp"
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

func TestResourceTypeMatch(t *testing.T) {
	for _, rt := range []string{
		"rbac.authorization.k8s.io/v1/Role",
		"rbac.authorization.k8s.io/v1/ClusterRoleBinding",
		"rbac.authorization.k8s.io/v1beta1/RoleBinding",
		"v1/ServiceAccount",
	} {
		if !resourceTypeMatch.MatchString(rt) {
			t.Errorf("%q rejected", rt)
		}
	}
	for _, rt := range []string{
		"v1/Pod",
		"apps/v1/Deployment",
		"rbac.authorization.k8s.io/v1/Role/extra",
		"other.rbac.authorization.k8s.io/v1/Role",
	} {
		if resourceTypeMatch.MatchString(rt) {
			t.Errorf("%q accepted", rt)
		}
	}
}

// The clause the server evaluates drops the escapes, because a filter literal
// cannot carry a backslash. That makes it broader than the patterns; it must
// never be narrower, or resources would go missing before anything local could
// notice.
func TestResourceTypeWhereIsBroaderNotNarrower(t *testing.T) {
	if strings.Contains(resourceTypeWhere, `\`) {
		t.Fatalf("a filter literal cannot carry a backslash: %s", resourceTypeWhere)
	}
	if !strings.HasPrefix(resourceTypeWhere, "ResourceType ~* '^(") {
		t.Fatalf("not a ResourceType regex clause: %s", resourceTypeWhere)
	}
	pattern := strings.TrimSuffix(strings.TrimPrefix(resourceTypeWhere, "ResourceType ~* '"), "'")
	wire := regexp.MustCompile("(?i)" + pattern)
	for _, rt := range []string{
		"rbac.authorization.k8s.io/v1/Role",
		"rbac.authorization.k8s.io/v1beta1/ClusterRoleBinding",
		"v1/ServiceAccount",
	} {
		if !wire.MatchString(rt) {
			t.Errorf("%q would not survive the server-side clause", rt)
		}
	}
}
