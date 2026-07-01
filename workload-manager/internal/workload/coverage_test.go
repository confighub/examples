// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package workload

import "testing"

func TestSelectorMatches(t *testing.T) {
	tests := []struct {
		name   string
		sel    LabelSelector
		labels map[string]string
		want   bool
	}{
		{"empty selector matches all", LabelSelector{}, map[string]string{"a": "b"}, true},
		{"matchLabels equal", LabelSelector{MatchLabels: map[string]string{"app": "web"}}, map[string]string{"app": "web"}, true},
		{"matchLabels mismatch", LabelSelector{MatchLabels: map[string]string{"app": "web"}}, map[string]string{"app": "api"}, false},
		{"matchLabels missing key", LabelSelector{MatchLabels: map[string]string{"app": "web"}}, map[string]string{}, false},
		{
			"In present", LabelSelector{MatchExpressions: []SelectorRequirement{{Key: "tier", Operator: "In", Values: []string{"fe", "be"}}}},
			map[string]string{"tier": "be"}, true,
		},
		{
			"In absent", LabelSelector{MatchExpressions: []SelectorRequirement{{Key: "tier", Operator: "In", Values: []string{"fe"}}}},
			map[string]string{}, false,
		},
		{
			"NotIn excludes", LabelSelector{MatchExpressions: []SelectorRequirement{{Key: "tier", Operator: "NotIn", Values: []string{"fe"}}}},
			map[string]string{"tier": "fe"}, false,
		},
		{
			"Exists", LabelSelector{MatchExpressions: []SelectorRequirement{{Key: "app", Operator: "Exists"}}},
			map[string]string{"app": "x"}, true,
		},
		{
			"DoesNotExist", LabelSelector{MatchExpressions: []SelectorRequirement{{Key: "app", Operator: "DoesNotExist"}}},
			map[string]string{"app": "x"}, false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sel.Matches(tc.labels); got != tc.want {
				t.Errorf("Matches(%v) = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}

func TestPDBBlocksAllEvictions(t *testing.T) {
	three := int64(3)
	tests := []struct {
		name     string
		pdb      PDBEntity
		replicas *int64
		want     bool
	}{
		{"maxUnavailable 0", PDBEntity{MaxUnavailable: "0"}, &three, true},
		{"minAvailable 100%", PDBEntity{MinAvailable: "100%"}, &three, true},
		{"minAvailable >= replicas", PDBEntity{MinAvailable: "3"}, &three, true},
		{"minAvailable < replicas", PDBEntity{MinAvailable: "1"}, &three, false},
		{"minAvailable percent below 100 not flagged", PDBEntity{MinAvailable: "50%"}, &three, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pdbBlocksAllEvictions(&tc.pdb, tc.replicas); got != tc.want {
				t.Errorf("pdbBlocksAllEvictions = %v, want %v", got, tc.want)
			}
		})
	}
}

// A multi-replica Deployment with no PDB fails availability; adding a matching
// PDB and anti-affinity makes it pass.
func TestAvailability_Coverage(t *testing.T) {
	dep := `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "web", "namespace": "shop"},
      "spec": {"replicas": 3, "template": {
        "metadata": {"labels": {"app": "web"}},
        "spec": {"affinity": {"podAntiAffinity": {"requiredDuringSchedulingIgnoredDuringExecution": [{"topologyKey": "kubernetes.io/hostname"}]}},
                 "containers": [{"name": "web"}]}
      }}
    }`

	// No PDB → uncovered (fail).
	uncovered := BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, dep)}})
	if got := availStatus(t, uncovered); got != StatusFail {
		t.Errorf("no PDB: want availability fail, got %s", got)
	}

	// Matching PDB → covered (pass; anti-affinity present so no spread warning).
	pdb := `{
      "apiVersion": "policy/v1", "kind": "PodDisruptionBudget",
      "metadata": {"name": "web-pdb", "namespace": "shop"},
      "spec": {"minAvailable": 1, "selector": {"matchLabels": {"app": "web"}}}
    }`
	covered := BuildFleet([]FleetResource{
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, dep)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, pdb)},
	})
	if got := availStatus(t, covered); got != StatusPass {
		t.Errorf("matching PDB + anti-affinity: want availability pass, got %s", got)
	}
}

// A wrong-namespace PDB does not cover the workload.
func TestAvailability_NamespaceScoped(t *testing.T) {
	dep := `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "web", "namespace": "shop"},
      "spec": {"replicas": 2, "template": {"metadata": {"labels": {"app": "web"}},
        "spec": {"containers": [{"name": "web"}]}}}
    }`
	pdb := `{
      "apiVersion": "policy/v1", "kind": "PodDisruptionBudget",
      "metadata": {"name": "web-pdb", "namespace": "other"},
      "spec": {"minAvailable": 1, "selector": {"matchLabels": {"app": "web"}}}
    }`
	clusters := BuildFleet([]FleetResource{
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, dep)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, pdb)},
	})
	if got := availStatus(t, clusters); got != StatusFail {
		t.Errorf("cross-namespace PDB should not cover: want fail, got %s", got)
	}
}

// availStatus scores a fleet and returns the availability status of its single
// workload.
func availStatus(t *testing.T, clusters map[string]*ClusterWorkloads) Status {
	t.Helper()
	scores := ScoreFleet(clusters, []string{DimAvailability})
	if len(scores) != 1 {
		t.Fatalf("want 1 workload, got %d", len(scores))
	}
	for _, d := range scores[0].Dimensions {
		if d.Dimension == DimAvailability {
			return d.Status
		}
	}
	t.Fatalf("no availability dimension in score")
	return ""
}

// Findings are ranked most-severe first and carry the analyzer + origin.
func TestFindings_Severity(t *testing.T) {
	dep := `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "bad", "namespace": "x"},
      "spec": {"replicas": 1, "template": {"spec": {"containers": [{"name": "c"}]}}}
    }`
	clusters := BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "c1"}, Doc: mustDoc(t, dep)}})
	findings := Findings(clusters)
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	// Sorted most-severe first.
	for i := 1; i < len(findings); i++ {
		if severityRank(findings[i-1].Severity) < severityRank(findings[i].Severity) {
			t.Errorf("findings not sorted by severity at %d: %s before %s", i, findings[i-1].Severity, findings[i].Severity)
		}
	}
	// A no-memory-limit finding must be high severity from the resources analyzer.
	var found bool
	for _, f := range findings {
		if f.Analyzer == DimResources && f.Severity == SeverityHigh {
			found = true
		}
	}
	if !found {
		t.Error("expected a high-severity resources finding (no memory limit)")
	}
}
