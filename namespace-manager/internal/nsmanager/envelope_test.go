// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"strings"
	"testing"
)

func findEnvelope(es []NamespaceEnvelope, ns string) *NamespaceEnvelope {
	for i := range es {
		if es[i].Namespace == ns {
			return &es[i]
		}
	}
	return nil
}

func TestAnalyzeClusterCompleteness(t *testing.T) {
	resources := []FleetResource{
		// payments: complete envelope.
		res("c1", "c1", "ns", nsDoc("payments", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		res("c1", "c1", "dd", netpolDoc("default-deny-all", "payments", map[string]any{}, []any{"Ingress", "Egress"})),
		res("c1", "c1", "rb", rbacDoc("RoleBinding", "baseline", "payments")),
		res("c1", "c1", "web", workloadDoc("Deployment", "web", "payments")),
		// orders: workload only — missing everything.
		res("c1", "c1", "orders-web", workloadDoc("Deployment", "api", "orders")),
		// shipping: Namespace object without pod-security, no default-deny, no rbac.
		res("c1", "c1", "shipping-ns", nsDoc("shipping", nil)),
	}
	clusters := BuildFleet(resources)
	got := AnalyzeCluster(clusters["c1"])

	pay := findEnvelope(got, "payments")
	if pay == nil || !pay.Complete {
		t.Fatalf("payments should be complete, got %+v", pay)
	}
	if pay.WorkloadCount != 1 {
		t.Errorf("payments workloadCount = %d, want 1", pay.WorkloadCount)
	}

	orders := findEnvelope(got, "orders")
	if orders == nil || orders.Complete {
		t.Fatalf("orders should be incomplete, got %+v", orders)
	}
	wantMissing := []string{MemberNamespaceObject, MemberPodSecurity, MemberDefaultDeny, MemberBaselineRBAC}
	if strings.Join(orders.Missing, ",") != strings.Join(wantMissing, ",") {
		t.Errorf("orders.Missing = %v, want %v", orders.Missing, wantMissing)
	}

	ship := findEnvelope(got, "shipping")
	if ship == nil || ship.Complete {
		t.Fatalf("shipping should be incomplete, got %+v", ship)
	}
	if !ship.HasNamespaceObject {
		t.Error("shipping should have a namespace object")
	}
	if ship.PodSecurityEnforce != "" {
		t.Errorf("shipping podSecurityEnforce = %q, want empty", ship.PodSecurityEnforce)
	}
}

func TestAnalyzeClusterSkipsPlaceholder(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		res("c1", "c1", "ph", nsDoc(PlaceholderNamespace, nil)),
		res("c1", "c1", "ph-np", netpolDoc("dd", PlaceholderNamespace, map[string]any{}, []any{"Ingress"})),
	})
	got := AnalyzeCluster(clusters["c1"])
	if findEnvelope(got, PlaceholderNamespace) != nil {
		t.Errorf("placeholder namespace should be skipped, got %+v", got)
	}
}

func TestDuplicateNamespaces(t *testing.T) {
	resources := []FleetResource{
		// Two Namespace objects named "payments" on target prod → collision.
		res("prod", "prod", "ns-a", nsDoc("payments", nil)),
		res("prod", "prod", "ns-b", nsDoc("payments", nil)),
		// Same name on a different target → not a collision.
		res("dev", "dev", "ns-c", nsDoc("payments", nil)),
		// Placeholder base bound to a dummy target → exempt.
		res("base", "dummy", "ns-base-1", nsDoc(PlaceholderNamespace, nil)),
		res("base", "dummy", "ns-base-2", nsDoc(PlaceholderNamespace, nil)),
		// Unbound (no target) duplicates → not deployed, skipped.
		res("unbound", "", "ns-u1", nsDoc("orders", nil)),
		res("unbound", "", "ns-u2", nsDoc("orders", nil)),
	}
	dups := DuplicateNamespaces(BuildFleet(resources))
	if len(dups) != 1 {
		t.Fatalf("duplicates = %d, want 1: %+v", len(dups), dups)
	}
	d := dups[0]
	if d.Target != "prod" || d.Namespace != "payments" || d.Count != 2 {
		t.Errorf("duplicate = %+v, want prod/payments/2", d)
	}
	if strings.Join(d.UnitSlugs, ",") != "ns-a,ns-b" {
		t.Errorf("unitSlugs = %v, want [ns-a ns-b]", d.UnitSlugs)
	}
}
