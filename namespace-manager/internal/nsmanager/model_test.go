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

func netpolDoc(name, namespace string, podSelector map[string]any, policyTypes []any) map[string]any {
	return map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec":       map[string]any{"podSelector": podSelector, "policyTypes": policyTypes},
	}
}

func rbacDoc(kind, name, namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}
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
		res("c1", "c1", "dd", netpolDoc("default-deny-all", "payments", map[string]any{}, []any{"Ingress", "Egress"})),
		res("c1", "c1", "sa", rbacDoc("ServiceAccount", "baseline", "payments")),
		res("c1", "c1", "rb", rbacDoc("RoleBinding", "baseline", "payments")),
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
	if len(c.NetworkPolicies) != 1 {
		t.Errorf("networkPolicies = %d, want 1", len(c.NetworkPolicies))
	}
	if len(c.RBAC) != 2 {
		t.Errorf("rbac = %d, want 2", len(c.RBAC))
	}
	if len(c.Workloads) != 1 {
		t.Errorf("workloads = %d, want 1", len(c.Workloads))
	}
	if got := c.Namespaces[0].PodSecurityEnforce(); got != "baseline" {
		t.Errorf("podSecurityEnforce = %q, want baseline", got)
	}
}

func TestIsDefaultDenyIngress(t *testing.T) {
	tests := []struct {
		name        string
		podSelector map[string]any
		policyTypes []any
		want        bool
	}{
		{"empty selector ingress+egress", map[string]any{}, []any{"Ingress", "Egress"}, true},
		{"empty selector ingress only", map[string]any{}, []any{"Ingress"}, true},
		{"empty selector egress only", map[string]any{}, []any{"Egress"}, false},
		{"empty selector no policyTypes (ingress implied)", map[string]any{}, nil, true},
		{"specific selector", map[string]any{"matchLabels": map[string]any{"app": "web"}}, []any{"Ingress"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters := BuildFleet([]FleetResource{
				res("c1", "c1", "np", netpolDoc("p", "ns", tt.podSelector, tt.policyTypes)),
			})
			got := clusters["c1"].NetworkPolicies[0].IsDefaultDenyIngress()
			if got != tt.want {
				t.Errorf("IsDefaultDenyIngress() = %v, want %v", got, tt.want)
			}
		})
	}
}
