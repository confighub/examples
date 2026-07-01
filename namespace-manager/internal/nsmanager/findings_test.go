// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import "testing"

func findingsByAnalyzer(fs []Finding, analyzer string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Analyzer == analyzer {
			out = append(out, f)
		}
	}
	return out
}

func TestAnalyzeFindings(t *testing.T) {
	resources := []FleetResource{
		// orders: workload + Namespace object without pod-security, no default-deny, no rbac.
		resFull("prod", "orders-prod", "orders", nsDoc("orders", nil)),
		resFull("prod", "orders-prod", "orders", workloadDoc("Deployment", "api", "orders")),
		// payments: complete envelope → no missing-* findings.
		resFull("prod", "payments-prod", "payments", nsDoc("payments", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		resFull("prod", "payments-prod", "payments", netpolDoc("default-deny-all", "payments", map[string]any{}, []any{"Ingress"})),
		resFull("prod", "payments-prod", "payments", rbacDoc("RoleBinding", "baseline", "payments")),
		resFull("prod", "payments-prod", "payments", workloadDoc("Deployment", "web", "payments")),
	}
	fs := AnalyzeFindings(BuildFleet(resources))

	if len(findingsByAnalyzer(fs, "missing-default-deny")) != 1 {
		t.Errorf("want 1 missing-default-deny, got %d", len(findingsByAnalyzer(fs, "missing-default-deny")))
	}
	if len(findingsByAnalyzer(fs, "missing-pod-security")) != 1 {
		t.Errorf("want 1 missing-pod-security (orders), got %d", len(findingsByAnalyzer(fs, "missing-pod-security")))
	}
	if len(findingsByAnalyzer(fs, "missing-baseline-rbac")) != 1 {
		t.Errorf("want 1 missing-baseline-rbac (orders), got %d", len(findingsByAnalyzer(fs, "missing-baseline-rbac")))
	}
	// payments is complete — no finding should name it.
	for _, f := range fs {
		if f.Namespace == "payments" {
			t.Errorf("payments should have no findings, got %+v", f)
		}
	}
	// High severity sorts first.
	if len(fs) > 0 && fs[0].Severity != SeverityHigh {
		t.Errorf("first finding severity = %q, want high", fs[0].Severity)
	}
}

func TestAnalyzeFindingsDuplicateAndInconsistency(t *testing.T) {
	resources := []FleetResource{
		// duplicate namespace "shared" on target prod.
		resFull("prod", "a-prod", "a", nsDoc("shared", nil)),
		resFull("prod", "b-prod", "b", nsDoc("shared", nil)),
		// component "billing": namespace name inconsistent across variants.
		resFull("dev", "billing-dev", "billing", nsDoc("billing", nil)),
		resFull("prod", "billing-prod", "billing", nsDoc("billing-prod", nil)),
	}
	fs := AnalyzeFindings(BuildFleet(resources))

	if len(findingsByAnalyzer(fs, "duplicate-namespace")) != 1 {
		t.Errorf("want 1 duplicate-namespace, got %d", len(findingsByAnalyzer(fs, "duplicate-namespace")))
	}
	inc := findingsByAnalyzer(fs, "namespace-name-inconsistent")
	if len(inc) != 1 || inc[0].Component != "billing" {
		t.Errorf("want 1 namespace-name-inconsistent for billing, got %+v", inc)
	}
}
