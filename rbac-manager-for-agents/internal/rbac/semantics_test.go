// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package rbac

import (
	"sort"
	"testing"
)

// mkRule builds a PolicyRule from the fields a test cares about; the rest
// default to empty (mirrors the partial-rule helper in semantics.test.ts).
func mkRule(r PolicyRule) PolicyRule { return r }

func TestRuleMatches_ExactVerbGroupResource(t *testing.T) {
	r := mkRule(PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}})
	cases := []struct {
		q    AccessQuery
		want bool
	}{
		{AccessQuery{Verb: "get", Resource: "pods", APIGroup: ""}, true},
		{AccessQuery{Verb: "list", Resource: "pods", APIGroup: ""}, false},
		{AccessQuery{Verb: "get", Resource: "secrets", APIGroup: ""}, false},
	}
	for _, c := range cases {
		if got := RuleMatches(r, c.q); got != c.want {
			t.Errorf("RuleMatches(%+v) = %v, want %v", c.q, got, c.want)
		}
	}
}

func TestRuleMatches_EmptyAPIGroupIsCore(t *testing.T) {
	core := mkRule(PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}})
	if RuleMatches(core, AccessQuery{Verb: "get", Resource: "pods", APIGroup: "apps"}) {
		t.Error("core rule should not match apps group")
	}
	apps := mkRule(PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get"}})
	if RuleMatches(apps, AccessQuery{Verb: "get", Resource: "deployments"}) {
		t.Error("apps rule should not match default-core query")
	}
	if !RuleMatches(apps, AccessQuery{Verb: "get", Resource: "deployments", APIGroup: "apps"}) {
		t.Error("apps rule should match apps query")
	}
}

func TestRuleMatches_Wildcards(t *testing.T) {
	r := mkRule(PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}})
	if !RuleMatches(r, AccessQuery{Verb: "deletecollection", Resource: "anything", APIGroup: "x.io"}) {
		t.Error("wildcard rule should match anything")
	}
}

func TestRuleMatches_ResourcesVsSubresources(t *testing.T) {
	bare := mkRule(PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}})
	if RuleMatches(bare, AccessQuery{Verb: "get", Resource: "pods/log", APIGroup: ""}) {
		t.Error("bare pods should not match pods/log")
	}
	sub := mkRule(PolicyRule{APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: []string{"get"}})
	if RuleMatches(sub, AccessQuery{Verb: "get", Resource: "pods", APIGroup: ""}) {
		t.Error("pods/log should not match bare pods")
	}
	if !RuleMatches(sub, AccessQuery{Verb: "get", Resource: "pods/log", APIGroup: ""}) {
		t.Error("pods/log should match pods/log")
	}
}

func TestRuleMatches_SegmentWildcards(t *testing.T) {
	r := mkRule(PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"*/scale"}, Verbs: []string{"update"}})
	if !RuleMatches(r, AccessQuery{Verb: "update", Resource: "deployments/scale", APIGroup: "apps"}) {
		t.Error("*/scale should match deployments/scale")
	}
	if RuleMatches(r, AccessQuery{Verb: "update", Resource: "deployments", APIGroup: "apps"}) {
		t.Error("*/scale should not match bare deployments")
	}
}

func TestRuleMatches_ResourceNames(t *testing.T) {
	r := mkRule(PolicyRule{
		APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"},
		ResourceNames: []string{"app-config"},
	})
	if !RuleMatches(r, AccessQuery{Verb: "get", Resource: "configmaps", APIGroup: ""}) {
		t.Error("restricted rule should match a name-less query")
	}
	if !RuleMatches(r, AccessQuery{Verb: "get", Resource: "configmaps", APIGroup: "", Name: "app-config"}) {
		t.Error("restricted rule should match the named object")
	}
	if RuleMatches(r, AccessQuery{Verb: "get", Resource: "configmaps", APIGroup: "", Name: "other"}) {
		t.Error("restricted rule should not match a different name")
	}
}

func TestRuleMatches_NonResourceURLOnly(t *testing.T) {
	r := mkRule(PolicyRule{NonResourceURLs: []string{"/healthz"}, Verbs: []string{"get"}})
	if RuleMatches(r, AccessQuery{Verb: "get", Resource: "pods", APIGroup: ""}) {
		t.Error("nonResourceURL-only rule should never match a resource query")
	}
	if !NonResourceURLMatches(r, "get", "/healthz") {
		t.Error("should match /healthz")
	}
	if NonResourceURLMatches(r, "get", "/metrics") {
		t.Error("should not match /metrics")
	}
}

func TestNonResourceURL_PrefixWildcard(t *testing.T) {
	r := mkRule(PolicyRule{NonResourceURLs: []string{"/healthz/*"}, Verbs: []string{"get"}})
	if !NonResourceURLMatches(r, "get", "/healthz/ready") {
		t.Error("/healthz/* should match /healthz/ready")
	}
	if NonResourceURLMatches(r, "get", "/livez") {
		t.Error("/healthz/* should not match /livez")
	}
}

func TestEffectiveRules_PlainRole(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRole("plain", []any{ruleDoc([]string{""}, []string{"pods"}, []string{"get"})})),
	})
	c1 := clusters["c1"]
	if got := len(EffectiveRules(c1.Roles[0], c1)); got != 1 {
		t.Errorf("plain role effective rules = %d, want 1", got)
	}
}

func TestEffectiveRules_NestedAggregationFixedPoint(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRole("top", []any{}, crOpts{
			aggregationRule: map[string]any{"clusterRoleSelectors": []any{
				map[string]any{"matchLabels": map[string]any{"tier": "mid"}},
			}},
		})),
		res("c1", clusterRole("mid",
			[]any{ruleDoc([]string{""}, []string{"pods"}, []string{"get"})},
			crOpts{
				labels: map[string]any{"tier": "mid"},
				aggregationRule: map[string]any{"clusterRoleSelectors": []any{
					map[string]any{"matchLabels": map[string]any{"tier": "leaf"}},
				}},
			})),
		res("c1", clusterRole("leaf",
			[]any{ruleDoc([]string{""}, []string{"secrets"}, []string{"list"})},
			crOpts{labels: map[string]any{"tier": "leaf"}})),
	})
	c1 := clusters["c1"]
	top := findRole(c1, "top")
	rules := EffectiveRules(top, c1)
	var resources []string
	for _, r := range rules {
		resources = append(resources, r.Resources...)
	}
	sort.Strings(resources)
	want := []string{"pods", "secrets"}
	if !equalStrings(resources, want) {
		t.Errorf("aggregated resources = %v, want %v", resources, want)
	}
}

func TestResolveRoleRef_RoleBindingToRoleInOwnNamespaceOnly(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", role("reader", "team-a", []any{ruleDoc([]string{""}, []string{"pods"}, []string{"get"})})),
		res("c1", roleBinding("rb-a", "team-a", RoleRef{Kind: "Role", Name: "reader"}, []any{user("alice")})),
		res("c1", roleBinding("rb-b", "team-b", RoleRef{Kind: "Role", Name: "reader"}, []any{user("bob")})),
	})
	c1 := clusters["c1"]
	rbA := findBinding(c1, "rb-a")
	rbB := findBinding(c1, "rb-b")
	if got := ResolveRoleRef(rbA, c1); got == nil || got.Name != "reader" {
		t.Errorf("rb-a should resolve to reader, got %v", got)
	}
	if got := ResolveRoleRef(rbB, c1); got != nil {
		t.Errorf("rb-b should not resolve (Role not in team-b), got %v", got)
	}
}

func TestResolveRoleRef_RoleBindingToClusterRoleScoped(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRole("viewer", []any{ruleDoc([]string{""}, []string{"pods"}, []string{"get"})})),
		res("c1", roleBinding("rb", "team-a", RoleRef{Kind: "ClusterRole", Name: "viewer"}, []any{group("devs")})),
	})
	c1 := clusters["c1"]
	rb := c1.Bindings[0]
	if got := ResolveRoleRef(rb, c1); got == nil || got.Kind != "ClusterRole" {
		t.Errorf("rb should resolve to a ClusterRole, got %v", got)
	}
	if !BindingScopeMatches(rb, "team-a") {
		t.Error("RoleBinding should apply in its own namespace")
	}
	if BindingScopeMatches(rb, "team-b") {
		t.Error("RoleBinding should not apply in another namespace")
	}
	if !BindingScopeMatches(rb, "") {
		t.Error("RoleBinding should match an unspecified (anywhere) query")
	}
}

func TestBindingScope_ClusterRoleBindingClusterWide(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRole("viewer", []any{})),
		res("c1", clusterRoleBinding("crb", "viewer", []any{group("devs")})),
	})
	crb := clusters["c1"].Bindings[0]
	if !BindingScopeMatches(crb, "any-namespace") {
		t.Error("ClusterRoleBinding should be cluster-wide")
	}
}

func findRole(c *ClusterRbac, name string) *RoleEntity {
	for _, r := range c.Roles {
		if r.Name == name {
			return r
		}
	}
	return nil
}

func findBinding(c *ClusterRbac, name string) *BindingEntity {
	for _, b := range c.Bindings {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
