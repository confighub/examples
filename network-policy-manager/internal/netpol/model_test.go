// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import (
	"encoding/json"
	"testing"
)

// mustDoc parses a JSON resource body into the decoded form BuildFleet expects.
func mustDoc(t *testing.T, body string) any {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("bad fixture JSON: %v", err)
	}
	return doc
}

func TestBuildFleetClassifiesAndExtracts(t *testing.T) {
	res := []FleetResource{
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy",
			"metadata":{"name":"default-deny","namespace":"app"},
			"spec":{"podSelector":{},"policyTypes":["Ingress"]}}`)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy",
			"metadata":{"name":"allow-web","namespace":"app"},
			"spec":{"podSelector":{"matchLabels":{"app":"web"}},
			        "policyTypes":["Ingress","Egress"],
			        "ingress":[{}],"egress":[{}]}}`)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"apps/v1","kind":"Deployment",
			"metadata":{"name":"web","namespace":"app"},
			"spec":{"template":{"metadata":{"labels":{"app":"web"}}}}}`)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"batch/v1","kind":"CronJob",
			"metadata":{"name":"report","namespace":"app"},
			"spec":{"jobTemplate":{"spec":{"template":{"metadata":{"labels":{"app":"report"}}}}}}}`)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"v1","kind":"Namespace",
			"metadata":{"name":"app","labels":{"team":"payments"}}}`)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"v1","kind":"Service",
			"metadata":{"name":"web","namespace":"app"},
			"spec":{"selector":{"app":"web"}}}`)},
	}

	fleet := BuildFleet(res)
	c := fleet["c1"]
	if c == nil {
		t.Fatal("cluster c1 missing")
	}
	if got := len(c.NetworkPolicies); got != 2 {
		t.Fatalf("networkPolicies = %d, want 2", got)
	}
	if got := len(c.Workloads); got != 2 {
		t.Fatalf("workloads = %d, want 2", got)
	}
	if got := len(c.Namespaces); got != 1 {
		t.Fatalf("namespaces = %d, want 1", got)
	}
	if got := len(c.Services); got != 1 {
		t.Fatalf("services = %d, want 1", got)
	}

	// default-deny: present empty podSelector, single Ingress policy type, no rules.
	dd := c.NetworkPolicies[0]
	if !dd.PodSelector.Present || !dd.PodSelector.Empty() {
		t.Errorf("default-deny podSelector: present=%v empty=%v, want present+empty", dd.PodSelector.Present, dd.PodSelector.Empty())
	}
	if len(dd.Ingress) != 0 || len(dd.Egress) != 0 {
		t.Errorf("default-deny rule counts = %d/%d, want 0/0", len(dd.Ingress), len(dd.Egress))
	}
	if !dd.IsolatesIngress() || dd.IsolatesEgress() {
		t.Errorf("default-deny isolation = in:%v eg:%v, want in:true eg:false", dd.IsolatesIngress(), dd.IsolatesEgress())
	}

	// allow-web: matchLabels selector, both policy types, one rule each.
	aw := c.NetworkPolicies[1]
	if aw.PodSelector.Empty() {
		t.Error("allow-web podSelector should not be empty")
	}
	if aw.PodSelector.MatchLabels["app"] != "web" {
		t.Errorf("allow-web podSelector matchLabels = %v, want app=web", aw.PodSelector.MatchLabels)
	}
	if len(aw.Ingress) != 1 || len(aw.Egress) != 1 {
		t.Errorf("allow-web rule counts = %d/%d, want 1/1", len(aw.Ingress), len(aw.Egress))
	}
	if !aw.IsolatesIngress() || !aw.IsolatesEgress() {
		t.Errorf("allow-web isolation = in:%v eg:%v, want both true", aw.IsolatesIngress(), aw.IsolatesEgress())
	}

	// Workload pod-template label extraction, including the CronJob nested path.
	labels := map[string]string{}
	for _, w := range c.Workloads {
		labels[w.Kind] = w.PodLabels["app"]
	}
	if labels["Deployment"] != "web" {
		t.Errorf("Deployment podLabels app = %q, want web", labels["Deployment"])
	}
	if labels["CronJob"] != "report" {
		t.Errorf("CronJob podLabels app = %q, want report", labels["CronJob"])
	}

	if c.Services[0].Selector["app"] != "web" {
		t.Errorf("service selector = %v, want app=web", c.Services[0].Selector)
	}
	if c.Namespaces[0].Labels["team"] != "payments" {
		t.Errorf("namespace labels = %v, want team=payments", c.Namespaces[0].Labels)
	}
}

func TestBuildFleetSkipsMalformed(t *testing.T) {
	res := []FleetResource{
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `"not-an-object"`)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{"kind":"NetworkPolicy"}`)}, // no name
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, `{
			"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy",
			"metadata":{"name":"ok","namespace":"app"},"spec":{"podSelector":{}}}`)},
	}
	fleet := BuildFleet(res)
	c := fleet["c1"]
	if c == nil || len(c.NetworkPolicies) != 1 {
		t.Fatalf("want exactly 1 well-formed NetworkPolicy, got %+v", c)
	}
}
