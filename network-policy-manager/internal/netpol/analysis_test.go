// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import (
	"encoding/json"
	"testing"
)

// fleetFixture builds a single-cluster fleet exercising every M2 analyzer and
// the connectivity model: a fully-covered namespace with an allow rule, a
// partially-covered namespace, an asymmetric egress/ingress pair, an allow-all
// policy, and metadata-egress (exposed vs. excepted).
func fleetFixture(t *testing.T) *ClusterNetpol {
	t.Helper()
	docs := []string{
		// covered ns: default-deny ingress + allow web from client.
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"covered-default-deny","namespace":"covered"},"spec":{"podSelector":{},"policyTypes":["Ingress"]}}`,
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"covered-allow-web","namespace":"covered"},"spec":{"podSelector":{"matchLabels":{"app":"web"}},"policyTypes":["Ingress"],"ingress":[{"from":[{"podSelector":{"matchLabels":{"app":"client"}}}]}]}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"cov-web","namespace":"covered"},"spec":{"template":{"metadata":{"labels":{"app":"web"}}}}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"cov-client","namespace":"covered"},"spec":{"template":{"metadata":{"labels":{"app":"client"}}}}}`,

		// partial ns: only a targeted policy; api is left uncovered, no default-deny.
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"partial-web-only","namespace":"partial"},"spec":{"podSelector":{"matchLabels":{"app":"web"}},"policyTypes":["Ingress"],"ingress":[{"from":[{"podSelector":{"matchLabels":{"app":"client"}}}]}]}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"part-web","namespace":"partial"},"spec":{"template":{"metadata":{"labels":{"app":"web"}}}}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"part-api","namespace":"partial"},"spec":{"template":{"metadata":{"labels":{"app":"api"}}}}}`,

		// asym ns: frontend egress allows backend, but backend ingress admits only "other".
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"asym-fe-egress","namespace":"asym"},"spec":{"podSelector":{"matchLabels":{"app":"frontend"}},"policyTypes":["Egress"],"egress":[{"to":[{"podSelector":{"matchLabels":{"app":"backend"}}}]}]}}`,
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"asym-be-ingress","namespace":"asym"},"spec":{"podSelector":{"matchLabels":{"app":"backend"}},"policyTypes":["Ingress"],"ingress":[{"from":[{"podSelector":{"matchLabels":{"app":"other"}}}]}]}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"asym-fe","namespace":"asym"},"spec":{"template":{"metadata":{"labels":{"app":"frontend"}}}}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"asym-be","namespace":"asym"},"spec":{"template":{"metadata":{"labels":{"app":"backend"}}}}}`,

		// allowall ns: default-deny shape but with an allow-all ingress rule.
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allowall","namespace":"allowall"},"spec":{"podSelector":{},"policyTypes":["Ingress"],"ingress":[{}]}}`,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"aa","namespace":"allowall"},"spec":{"template":{"metadata":{"labels":{"app":"x"}}}}}`,

		// meta ns: one egress exposes the metadata IP, one excepts it. No workloads.
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"meta-open","namespace":"meta"},"spec":{"podSelector":{"matchLabels":{"app":"egressor"}},"policyTypes":["Egress"],"egress":[{"to":[{"ipBlock":{"cidr":"0.0.0.0/0"}}]}]}}`,
		`{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"meta-blocked","namespace":"meta"},"spec":{"podSelector":{"matchLabels":{"app":"egressor2"}},"policyTypes":["Egress"],"egress":[{"to":[{"ipBlock":{"cidr":"0.0.0.0/0","except":["169.254.169.254/32"]}}]}]}}`,
	}
	var res []FleetResource
	for _, d := range docs {
		var doc any
		if err := json.Unmarshal([]byte(d), &doc); err != nil {
			t.Fatalf("bad fixture: %v", err)
		}
		res = append(res, FleetResource{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc})
	}
	c := BuildFleet(res)["c1"]
	if c == nil {
		t.Fatal("cluster c1 missing")
	}
	return c
}

func findWorkload(c *ClusterNetpol, name string) *WorkloadEntity {
	for _, w := range c.Workloads {
		if w.Name == name {
			return w
		}
	}
	return nil
}

func TestCoverage(t *testing.T) {
	c := fleetFixture(t)
	nss, _ := c.Coverage()
	byNS := map[string]NamespaceCoverage{}
	for _, nc := range nss {
		byNS[nc.Namespace] = nc
	}

	if !byNS["covered"].DefaultDenyIngress {
		t.Error("covered namespace should have default-deny ingress")
	}
	if len(byNS["covered"].UncoveredIngress) != 0 {
		t.Errorf("covered namespace should have no uncovered ingress, got %v", byNS["covered"].UncoveredIngress)
	}
	if byNS["partial"].DefaultDenyIngress {
		t.Error("partial namespace should NOT have default-deny ingress")
	}
	if got := byNS["partial"].UncoveredIngress; len(got) != 1 || got[0] != "part-api" {
		t.Errorf("partial uncovered ingress = %v, want [part-api]", got)
	}
}

func TestConnectivity(t *testing.T) {
	c := fleetFixture(t)

	// In the covered namespace, the client is allowed to reach web; nothing else is.
	web := findWorkload(c, "cov-web")
	sources := WhoCanReach(c, web)
	if len(sources) != 1 || sources[0].Name != "cov-client" {
		t.Fatalf("who-can-reach cov-web = %v, want [cov-client]", names(sources))
	}

	// The asymmetric flow is actually blocked: frontend cannot reach backend
	// because backend's ingress does not admit it.
	be := findWorkload(c, "asym-be")
	if got := names(WhoCanReach(c, be)); contains(got, "asym-fe") {
		t.Errorf("asym-fe should NOT be able to reach asym-be, got sources %v", got)
	}
	fe := findWorkload(c, "asym-fe")
	if got := names(ReachableFrom(c, fe)); contains(got, "asym-be") {
		t.Errorf("asym-fe should not reach asym-be, got dests %v", got)
	}
}

func TestFindings(t *testing.T) {
	c := fleetFixture(t)
	byAnalyzer := map[string]int{}
	for _, f := range AnalyzeFindings(c) {
		byAnalyzer[f.Analyzer]++
	}
	want := map[string]int{
		"missing-default-deny-ingress": 2, // partial, asym
		"uncovered-ingress":            2, // part-api, asym-fe
		"allow-all":                    1, // allowall
		"metadata-egress":              1, // meta-open only (meta-blocked is excepted)
		"ingress-egress-asymmetry":     1, // frontend egress vs backend ingress
	}
	for analyzer, n := range want {
		if byAnalyzer[analyzer] != n {
			t.Errorf("analyzer %q = %d findings, want %d", analyzer, byAnalyzer[analyzer], n)
		}
	}
}

func names(ws []*WorkloadEntity) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Name)
	}
	return out
}
