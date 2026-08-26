// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import "testing"

// res builds a FleetResource on cluster `cluster` from a decoded-JSON doc.
func res(cluster, target, unitSlug string, doc map[string]any) FleetResource {
	return FleetResource{
		Origin: ResourceOrigin{Cluster: cluster, Target: target, UnitSlug: unitSlug},
		Doc:    doc,
	}
}

func nsDoc(name string, labels map[string]any) map[string]any {
	md := map[string]any{"name": name}
	if labels != nil {
		md["labels"] = labels
	}
	return map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": md}
}

func workloadDoc(kind, name, namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "apps/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}
}

func TestBuildFleetClassifies(t *testing.T) {
	resources := []FleetResource{
		res("c1", "c1", "ns", nsDoc("payments", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		// A NetworkPolicy and an RBAC object are no longer this tool's subject
		// matter, so they classify as nothing and are dropped.
		res("c1", "c1", "dd", map[string]any{"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy",
			"metadata": map[string]any{"name": "default-deny-all", "namespace": "payments"}}),
		res("c1", "c1", "rb", map[string]any{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "RoleBinding",
			"metadata": map[string]any{"name": "baseline", "namespace": "payments"}}),
		res("c1", "c1", "web", workloadDoc("Deployment", "web", "payments")),
		res("c1", "c1", "bad", map[string]any{"kind": "Namespace"}), // no name → skipped
	}
	clusters := BuildFleet(resources)
	c := clusters["c1"]
	if c == nil {
		t.Fatal("cluster c1 missing")
	}
	if len(c.Namespaces) != 1 {
		t.Errorf("namespaces = %d, want 1", len(c.Namespaces))
	}
	if len(c.Workloads) != 1 {
		t.Errorf("workloads = %d, want 1", len(c.Workloads))
	}
	if got := c.Namespaces[0].PodSecurityEnforce(); got != "baseline" {
		t.Errorf("podSecurityEnforce = %q, want baseline", got)
	}
}
