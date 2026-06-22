// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package rbac

import (
	"sort"
	"testing"
)

func secretReader() map[string]any {
	return clusterRole("secret-reader", []any{ruleDoc([]string{""}, []string{"secrets"}, []string{"get", "list"})})
}

func TestWhoCan_AcrossClustersWithProvenance(t *testing.T) {
	clusters := build([]FleetResource{
		res("prod-cluster", secretReader()),
		res("prod-cluster", clusterRoleBinding("ops-secrets", "secret-reader", []any{group("ops")})),
		res("dev-cluster", secretReader()),
		res("dev-cluster", clusterRoleBinding("dev-secrets", "secret-reader", []any{group("devs")})),
	})
	grants := WhoCan(clusters, AccessQuery{Verb: "get", Resource: "secrets", APIGroup: ""})
	if len(grants) != 2 {
		t.Fatalf("got %d grants, want 2", len(grants))
	}
	byCluster := map[string]Grant{}
	for _, g := range grants {
		byCluster[g.Cluster] = g
	}
	if byCluster["prod-cluster"].SubjectKey != "Group:ops" {
		t.Errorf("prod subject = %q, want Group:ops", byCluster["prod-cluster"].SubjectKey)
	}
	if byCluster["prod-cluster"].Binding.Name != "ops-secrets" {
		t.Errorf("prod binding = %q, want ops-secrets", byCluster["prod-cluster"].Binding.Name)
	}
	if byCluster["dev-cluster"].SubjectKey != "Group:devs" {
		t.Errorf("dev subject = %q, want Group:devs", byCluster["dev-cluster"].SubjectKey)
	}
}

func TestWhoCan_RoleBindingNamespaceScope(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", role("ns-reader", "payments", []any{ruleDoc([]string{""}, []string{"secrets"}, []string{"get"})})),
		res("c1", roleBinding("rb", "payments", RoleRef{Kind: "Role", Name: "ns-reader"}, []any{user("alice")})),
	})
	if got := WhoCan(clusters, AccessQuery{Verb: "get", Resource: "secrets", Namespace: "payments"}); len(got) != 1 {
		t.Errorf("payments query = %d grants, want 1", len(got))
	}
	if got := WhoCan(clusters, AccessQuery{Verb: "get", Resource: "secrets", Namespace: "other"}); len(got) != 0 {
		t.Errorf("other query = %d grants, want 0", len(got))
	}
	anywhere := WhoCan(clusters, AccessQuery{Verb: "get", Resource: "secrets"})
	if len(anywhere) != 1 {
		t.Fatalf("anywhere query = %d grants, want 1", len(anywhere))
	}
	if anywhere[0].Scope != "payments" {
		t.Errorf("scope = %q, want payments", anywhere[0].Scope)
	}
}

func TestWhoCan_ClusterAdminBuiltin(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRoleBinding("breakglass", "cluster-admin", []any{group("oncall")})),
	})
	grants := WhoCan(clusters, AccessQuery{Verb: "delete", Resource: "secrets", APIGroup: ""})
	if len(grants) != 1 {
		t.Fatalf("got %d grants, want 1", len(grants))
	}
	if !grants[0].ViaBuiltinRole {
		t.Error("expected ViaBuiltinRole=true")
	}
	if grants[0].RoleRefName != "cluster-admin" {
		t.Errorf("roleRefName = %q, want cluster-admin", grants[0].RoleRefName)
	}
}

func TestWhoCan_NoInventedAccessForUnknownBuiltins(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRoleBinding("viewers", "view", []any{group("everyone")})),
	})
	if got := WhoCan(clusters, AccessQuery{Verb: "get", Resource: "secrets"}); len(got) != 0 {
		t.Errorf("view binding = %d grants, want 0", len(got))
	}
}

func TestWhoCan_ThroughAggregatedRoles(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRole("agg", []any{}, crOpts{
			aggregationRule: map[string]any{"clusterRoleSelectors": []any{
				map[string]any{"matchLabels": map[string]any{"part": "yes"}},
			}},
		})),
		res("c1", clusterRole("part", []any{ruleDoc([]string{""}, []string{"secrets"}, []string{"get"})},
			crOpts{labels: map[string]any{"part": "yes"}})),
		res("c1", clusterRoleBinding("crb", "agg", []any{user("root")})),
	})
	if got := WhoCan(clusters, AccessQuery{Verb: "get", Resource: "secrets"}); len(got) != 1 {
		t.Errorf("aggregated query = %d grants, want 1", len(got))
	}
}

func TestSubjectAccess_AcrossFleet(t *testing.T) {
	clusters := build([]FleetResource{
		res("prod-cluster", secretReader()),
		res("prod-cluster", clusterRoleBinding("b1", "secret-reader", []any{group("ops"), user("alice")})),
		res("dev-cluster", clusterRoleBinding("b2", "cluster-admin", []any{user("alice")})),
	})
	grants := SubjectAccess(clusters, SubjectRef{Kind: "User", Name: "alice"})
	var got []string
	for _, g := range grants {
		got = append(got, g.Cluster+"/"+g.RoleRefName)
	}
	sort.Strings(got)
	want := []string{"dev-cluster/cluster-admin", "prod-cluster/secret-reader"}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSubjectAccess_ServiceAccountByNamespace(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", roleBinding("rb", "apps", RoleRef{Kind: "ClusterRole", Name: "edit"}, []any{sa("ci-deployer", "apps")})),
	})
	if got := SubjectAccess(clusters, SubjectRef{Kind: "ServiceAccount", Name: "ci-deployer", Namespace: "apps"}); len(got) != 1 {
		t.Errorf("apps SA = %d grants, want 1", len(got))
	}
	if got := SubjectAccess(clusters, SubjectRef{Kind: "ServiceAccount", Name: "ci-deployer", Namespace: "other"}); len(got) != 0 {
		t.Errorf("other SA = %d grants, want 0", len(got))
	}
}

func TestAllSubjects_Dedup(t *testing.T) {
	clusters := build([]FleetResource{
		res("c1", clusterRoleBinding("b1", "view", []any{group("devs"), user("alice")})),
		res("c2", clusterRoleBinding("b2", "view", []any{group("devs")})),
	})
	var names []string
	for _, s := range AllSubjects(clusters) {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	if !equalStrings(names, []string{"alice", "devs"}) {
		t.Errorf("got %v, want [alice devs]", names)
	}
}
