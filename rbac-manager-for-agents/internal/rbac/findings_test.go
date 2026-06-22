// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package rbac

import (
	"slices"
	"strings"
	"testing"
)

func byAnalyzer(findings []Finding, analyzer string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Analyzer == analyzer {
			out = append(out, f)
		}
	}
	return out
}

func TestAnalyze_Wildcards(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", clusterRole("legacy-admin", []any{ruleDoc([]string{"*"}, []string{"*"}, []string{"*"})})),
	}))
	wild := byAnalyzer(findings, "wildcard-rules")
	if len(wild) != 1 {
		t.Fatalf("got %d wildcard findings, want 1", len(wild))
	}
	if wild[0].Severity != SeverityHigh {
		t.Errorf("severity = %q, want high", wild[0].Severity)
	}
	if wild[0].ResourceName != "legacy-admin" {
		t.Errorf("resourceName = %q, want legacy-admin", wild[0].ResourceName)
	}
}

func TestAnalyze_EscalationVerbs(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", clusterRole("escalator", []any{
			ruleDoc([]string{"rbac.authorization.k8s.io"}, []string{"clusterroles"}, []string{"bind", "escalate"}),
		})),
	}))
	esc := byAnalyzer(findings, "privilege-escalation-verbs")
	if len(esc) != 1 {
		t.Fatalf("got %d escalation findings, want 1", len(esc))
	}
	if !strings.Contains(esc[0].Message, "escalate") {
		t.Errorf("message %q should mention escalate", esc[0].Message)
	}
}

func TestAnalyze_RiskyGrants(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", clusterRole("ops", []any{
			ruleDoc([]string{""}, []string{"secrets"}, []string{"get", "list"}),
			ruleDoc([]string{""}, []string{"pods/exec"}, []string{"create"}),
		})),
	}))
	risky := byAnalyzer(findings, "risky-grants")
	if len(risky) != 1 {
		t.Fatalf("got %d risky findings, want 1", len(risky))
	}
	if !strings.Contains(risky[0].Message, "secrets read") {
		t.Errorf("message %q should mention secrets read", risky[0].Message)
	}
	if !strings.Contains(risky[0].Message, "pod exec") {
		t.Errorf("message %q should mention pod exec", risky[0].Message)
	}
}

func TestAnalyze_ClusterAdminBindings(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", clusterRoleBinding("breakglass", "cluster-admin", []any{group("oncall")})),
		res("c1", clusterRole("shadow-admin", []any{ruleDoc([]string{"*"}, []string{"*"}, []string{"*"})})),
		res("c1", clusterRoleBinding("shadow", "shadow-admin", []any{user("bob")})),
	}))
	admin := byAnalyzer(findings, "cluster-admin-bindings")
	var names []string
	for _, f := range admin {
		names = append(names, f.ResourceName)
	}
	// AnalyzeFleet sorts by severity/cluster/id; both are CRBs (high) here.
	if len(names) != 2 || !slices.Contains(names, "breakglass") || !slices.Contains(names, "shadow") {
		t.Errorf("cluster-admin findings = %v, want breakglass and shadow", names)
	}
}

func TestAnalyze_OrphanedBindingsNotBuiltins(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", roleBinding("orphan", "monitoring", RoleRef{Kind: "Role", Name: "grafana-viewer"}, []any{user("alice@example.com")})),
		res("c1", clusterRoleBinding("ok-builtin", "view", []any{group("devs")})),
		res("c1", clusterRoleBinding("ok-system", "system:metrics-reader", []any{group("devs")})),
	}))
	orphans := byAnalyzer(findings, "orphaned-bindings")
	if len(orphans) != 1 {
		t.Fatalf("got %d orphan findings, want 1", len(orphans))
	}
	if orphans[0].ResourceName != "orphan" {
		t.Errorf("resourceName = %q, want orphan", orphans[0].ResourceName)
	}
	if !strings.Contains(orphans[0].Message, "grafana-viewer") {
		t.Errorf("message %q should mention grafana-viewer", orphans[0].Message)
	}
}

func TestAnalyze_UnboundServiceAccounts(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", serviceAccount("used", "apps")),
		res("c1", serviceAccount("unused", "apps")),
		res("c1", roleBinding("rb", "apps", RoleRef{Kind: "ClusterRole", Name: "edit"}, []any{sa("used", "apps")})),
	}))
	unbound := byAnalyzer(findings, "unbound-service-accounts")
	if len(unbound) != 1 {
		t.Fatalf("got %d unbound findings, want 1", len(unbound))
	}
	if unbound[0].ResourceName != "unused" {
		t.Errorf("resourceName = %q, want unused", unbound[0].ResourceName)
	}
}

func TestAnalyze_CleanCluster(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("c1", clusterRole("developer", []any{
			ruleDoc([]string{"", "apps"}, []string{"pods", "deployments"}, []string{"get", "list", "watch", "create", "update", "patch"}),
		})),
		res("c1", clusterRoleBinding("developer", "developer", []any{group("oidc:developers")})),
	}))
	if len(findings) != 0 {
		t.Errorf("clean cluster produced %d findings: %+v", len(findings), findings)
	}
}

func TestAnalyze_SortBySeverityThenCluster(t *testing.T) {
	findings := AnalyzeFleet(build([]FleetResource{
		res("b-cluster", serviceAccount("unused", "apps")),
		res("a-cluster", clusterRole("legacy", []any{ruleDoc([]string{"*"}, []string{"*"}, []string{"*"})})),
	}))
	if len(findings) < 2 {
		t.Fatalf("got %d findings, want >= 2", len(findings))
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("first severity = %q, want high", findings[0].Severity)
	}
	if findings[len(findings)-1].Severity != SeverityLow {
		t.Errorf("last severity = %q, want low", findings[len(findings)-1].Severity)
	}
}
