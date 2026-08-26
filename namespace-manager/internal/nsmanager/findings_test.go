// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"testing"

	api "github.com/confighub/sdk/core/function/api"
)

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
		// orders: workload + Namespace object without pod-security.
		resFull("prod", "orders-prod", "orders", nsDoc("orders", nil)),
		resFull("prod", "orders-prod", "orders", workloadDoc("Deployment", "api", "orders")),
		// payments: complete envelope → no missing-* findings.
		resFull("prod", "payments-prod", "payments", nsDoc("payments", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		resFull("prod", "payments-prod", "payments", workloadDoc("Deployment", "web", "payments")),
	}
	fs := AnalyzeFindings(BuildFleet(resources))

	if len(findingsByAnalyzer(fs, "missing-pod-security")) != 1 {
		t.Errorf("want 1 missing-pod-security (orders), got %d", len(findingsByAnalyzer(fs, "missing-pod-security")))
	}
	// payments is complete — no finding should name it.
	for _, f := range fs {
		if f.Namespace == "payments" {
			t.Errorf("payments should have no findings, got %+v", f)
		}
	}
	// Findings are ranked most-severe first. Asserting the ordering rather than a
	// particular severity keeps the test honest as the analyzer set changes.
	for i := 1; i < len(fs); i++ {
		if api.ScoreToNumber[fs[i-1].Severity] < api.ScoreToNumber[fs[i].Severity] {
			t.Errorf("findings not sorted most-severe first at %d: %q before %q",
				i, fs[i-1].Severity, fs[i].Severity)
		}
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
