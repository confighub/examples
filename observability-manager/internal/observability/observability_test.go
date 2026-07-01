// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package observability

import (
	"encoding/json"
	"testing"
)

func fleet(t *testing.T, manifests ...string) map[string]*ClusterObservability {
	t.Helper()
	var frs []FleetResource
	for _, m := range manifests {
		var doc any
		if err := json.Unmarshal([]byte(m), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		frs = append(frs, FleetResource{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc})
	}
	return BuildFleet(frs)
}

const metricsService = `{
  "apiVersion": "v1", "kind": "Service",
  "metadata": {"name": "web", "namespace": "shop", "labels": {"app": "web"}},
  "spec": {"ports": [{"name": "http", "port": 80}, {"name": "metrics", "port": 9090}]}
}`

const matchingMonitor = `{
  "apiVersion": "monitoring.coreos.com/v1", "kind": "ServiceMonitor",
  "metadata": {"name": "web-sm", "namespace": "shop"},
  "spec": {"selector": {"matchLabels": {"app": "web"}}, "endpoints": [{"port": "metrics"}]}
}`

func TestExposesMetrics(t *testing.T) {
	clusters := fleet(t, metricsService)
	svc := clusters["c1"].Services[0]
	if !svc.ExposesMetrics() {
		t.Errorf("service with a metrics port should expose metrics")
	}

	plain := fleet(t, `{"apiVersion":"v1","kind":"Service","metadata":{"name":"p","namespace":"x"},"spec":{"ports":[{"name":"http","port":80}]}}`)
	if plain["c1"].Services[0].ExposesMetrics() {
		t.Errorf("service with only http port should not expose metrics")
	}

	annotated := fleet(t, `{"apiVersion":"v1","kind":"Service","metadata":{"name":"a","namespace":"x","annotations":{"prometheus.io/scrape":"true"}},"spec":{"ports":[{"name":"http","port":80}]}}`)
	if !annotated["c1"].Services[0].ExposesMetrics() {
		t.Errorf("prometheus.io/scrape=true should expose metrics")
	}
}

func TestCoverage_UncoveredThenCovered(t *testing.T) {
	// Metrics service, no ServiceMonitor -> uncovered.
	uncovered := AnalyzeCoverage(fleet(t, metricsService))
	if len(uncovered) != 1 || uncovered[0].Covered {
		t.Fatalf("want 1 uncovered metrics service, got %+v", uncovered)
	}

	// Add a matching ServiceMonitor -> covered.
	covered := AnalyzeCoverage(fleet(t, metricsService, matchingMonitor))
	if len(covered) != 1 || !covered[0].Covered || covered[0].Monitors[0] != "web-sm" {
		t.Fatalf("want covered by web-sm, got %+v", covered)
	}
}

func TestCoverage_NamespaceScoped(t *testing.T) {
	otherNsMonitor := `{
      "apiVersion": "monitoring.coreos.com/v1", "kind": "ServiceMonitor",
      "metadata": {"name": "web-sm", "namespace": "other"},
      "spec": {"selector": {"matchLabels": {"app": "web"}}}
    }`
	res := AnalyzeCoverage(fleet(t, metricsService, otherNsMonitor))
	if len(res) != 1 || res[0].Covered {
		t.Errorf("a ServiceMonitor in another namespace must not cover: %+v", res)
	}
}

func TestFindings_UncoveredAndDangling(t *testing.T) {
	danglingMonitor := `{
      "apiVersion": "monitoring.coreos.com/v1", "kind": "ServiceMonitor",
      "metadata": {"name": "orphan", "namespace": "shop"},
      "spec": {"selector": {"matchLabels": {"app": "nope"}}}
    }`
	fs := Findings(fleet(t, metricsService, danglingMonitor))
	var haveCoverage, haveDangling bool
	for _, f := range fs {
		if f.Analyzer == "coverage" && f.Severity == SeverityMedium {
			haveCoverage = true
		}
		if f.Analyzer == "dangling" && f.Severity == SeverityLow {
			haveDangling = true
		}
	}
	if !haveCoverage {
		t.Error("expected an uncovered-metrics-service coverage finding")
	}
	if !haveDangling {
		t.Error("expected a dangling-servicemonitor finding")
	}
	// Most-severe first.
	if len(fs) >= 2 && severityRank(fs[0].Severity) < severityRank(fs[1].Severity) {
		t.Error("findings not sorted most-severe first")
	}
}

func TestSidecars_Presence(t *testing.T) {
	dep := `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "web", "namespace": "shop"},
      "spec": {"template": {"spec": {"containers": [{"name": "app"}, {"name": "otel-collector"}]}}}
    }`
	res := AnalyzeSidecars(fleet(t, dep), nil)
	if len(res) != 1 || !res[0].HasSidecar || res[0].Sidecar != "otel-collector" {
		t.Fatalf("want otel-collector sidecar detected, got %+v", res)
	}
}
