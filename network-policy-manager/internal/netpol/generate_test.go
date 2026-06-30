// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// parseManifest round-trips a generated manifest back through the engine model,
// so the tests assert on semantics rather than YAML text.
func parseManifest(t *testing.T, manifest string) *ClusterNetpol {
	t.Helper()
	var doc any
	if err := yaml.Unmarshal([]byte(manifest), &doc); err != nil {
		t.Fatalf("generated manifest is not valid YAML: %v\n%s", err, manifest)
	}
	return BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc}})["c1"]
}

func TestDefaultDenyYAML(t *testing.T) {
	slug, manifest := DefaultDenyYAML("payments", false)
	if slug != "default-deny-payments" {
		t.Errorf("slug = %q, want default-deny-payments", slug)
	}
	c := parseManifest(t, manifest)
	if c == nil || len(c.NetworkPolicies) != 1 {
		t.Fatalf("want 1 NetworkPolicy, got %+v", c)
	}
	np := c.NetworkPolicies[0]
	if np.Namespace != "payments" {
		t.Errorf("namespace = %q, want payments", np.Namespace)
	}
	if !np.PodSelector.Empty() {
		t.Error("default-deny should select all pods (empty podSelector)")
	}
	if !np.IsolatesIngress() || np.IsolatesEgress() {
		t.Errorf("ingress-only default-deny isolation = in:%v eg:%v, want in:true eg:false", np.IsolatesIngress(), np.IsolatesEgress())
	}
}

func TestDefaultDenyYAMLEgressAllowsDNS(t *testing.T) {
	_, manifest := DefaultDenyYAML("payments", true)
	c := parseManifest(t, manifest)
	np := c.NetworkPolicies[0]
	if !np.IsolatesIngress() || !np.IsolatesEgress() {
		t.Errorf("egress default-deny isolation = in:%v eg:%v, want both true", np.IsolatesIngress(), np.IsolatesEgress())
	}
	// Exactly one egress rule: the DNS allowance.
	if len(np.Egress) != 1 {
		t.Fatalf("egress rules = %d, want 1 (DNS)", len(np.Egress))
	}
	rule := np.Egress[0]
	hasDNSPort := false
	for _, p := range rule.Ports {
		if p.Port == "53" {
			hasDNSPort = true
		}
	}
	if !hasDNSPort {
		t.Errorf("egress DNS rule missing port 53; ports=%+v", rule.Ports)
	}
}

func TestAllowYAMLIngress(t *testing.T) {
	src := &WorkloadEntity{Kind: "Deployment", Name: "client", Namespace: "app", PodLabels: map[string]string{"app": "client"}}
	dst := &WorkloadEntity{Kind: "Deployment", Name: "web", Namespace: "app", PodLabels: map[string]string{"app": "web"}}
	slug, manifest := AllowYAML(src, dst, false, "8080")
	if slug != "allow-client-to-web" {
		t.Errorf("slug = %q, want allow-client-to-web", slug)
	}
	c := parseManifest(t, manifest)
	np := c.NetworkPolicies[0]
	if np.Namespace != "app" {
		t.Errorf("ingress allow should live in dst namespace, got %q", np.Namespace)
	}
	if np.PodSelector.MatchLabels["app"] != "web" {
		t.Errorf("podSelector should target dst (web), got %v", np.PodSelector.MatchLabels)
	}
	if !np.IsolatesIngress() || len(np.Ingress) != 1 || len(np.Ingress[0].Peers) != 1 {
		t.Fatalf("want one ingress rule with one peer, got %+v", np.Ingress)
	}
	peer := np.Ingress[0].Peers[0]
	if peer.PodSelector == nil || peer.PodSelector.MatchLabels["app"] != "client" {
		t.Errorf("ingress peer should select src (client), got %+v", peer.PodSelector)
	}
}

// TestAllowYAMLBridgesAsymmetry verifies a generated allow policy actually makes
// the flow reachable in the connectivity model when paired with a default-deny.
func TestAllowYAMLBridgesAsymmetry(t *testing.T) {
	src := &WorkloadEntity{Kind: "Deployment", Name: "client", Namespace: "app", PodLabels: map[string]string{"app": "client"}}
	dst := &WorkloadEntity{Kind: "Deployment", Name: "web", Namespace: "app", PodLabels: map[string]string{"app": "web"}}
	_, deny := DefaultDenyYAML("app", false)
	_, allow := AllowYAML(src, dst, false, "")

	var res []FleetResource
	for _, m := range []string{deny, allow} {
		var doc any
		if err := yaml.Unmarshal([]byte(m), &doc); err != nil {
			t.Fatal(err)
		}
		res = append(res, FleetResource{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc})
	}
	// Add the two workloads themselves.
	res = append(res,
		FleetResource{Origin: ResourceOrigin{Cluster: "c1"}, Doc: workloadDoc(t, "client", "app", "client")},
		FleetResource{Origin: ResourceOrigin{Cluster: "c1"}, Doc: workloadDoc(t, "web", "app", "web")},
	)
	c := BuildFleet(res)["c1"]
	got := names(WhoCanReach(c, findWorkload(c, "web")))
	if len(got) != 1 || got[0] != "client" {
		t.Errorf("after default-deny + allow, who-can-reach web = %v, want [client]", got)
	}
}

func TestAllowIngressYAMLConsolidatesSources(t *testing.T) {
	dst := &WorkloadEntity{Kind: "Service", Name: "cartservice", Namespace: "app", PodLabels: map[string]string{"app": "cartservice"}}
	a := &WorkloadEntity{Kind: "Deployment", Name: "checkoutservice", Namespace: "app", PodLabels: map[string]string{"app": "checkoutservice"}}
	b := &WorkloadEntity{Kind: "Deployment", Name: "frontend", Namespace: "app", PodLabels: map[string]string{"app": "frontend"}}
	slug, manifest := AllowIngressYAML(dst, []*WorkloadEntity{a, b, a}, "") // duplicate a on purpose
	if slug != "allow-cartservice-ingress" {
		t.Errorf("slug = %q, want allow-cartservice-ingress", slug)
	}
	c := parseManifest(t, manifest)
	np := c.NetworkPolicies[0]
	if np.PodSelector.MatchLabels["app"] != "cartservice" {
		t.Errorf("podSelector should target cartservice, got %v", np.PodSelector.MatchLabels)
	}
	// One ingress rule with two distinct from-peers (the duplicate is collapsed).
	if len(np.Ingress) != 1 {
		t.Fatalf("ingress rules = %d, want 1", len(np.Ingress))
	}
	if len(np.Ingress[0].Peers) != 2 {
		t.Fatalf("from-peers = %d, want 2 (deduped)", len(np.Ingress[0].Peers))
	}
	got := map[string]bool{}
	for _, p := range np.Ingress[0].Peers {
		if p.PodSelector != nil {
			got[p.PodSelector.MatchLabels["app"]] = true
		}
	}
	if !got["checkoutservice"] || !got["frontend"] {
		t.Errorf("from-peers = %v, want checkoutservice + frontend", got)
	}
}

func workloadDoc(t *testing.T, name, namespace, app string) any {
	t.Helper()
	doc := map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"template": map[string]any{"metadata": map[string]any{"labels": map[string]any{"app": app}}}},
	}
	return doc
}
