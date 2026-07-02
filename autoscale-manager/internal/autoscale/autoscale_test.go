// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package autoscale

import (
	"encoding/json"
	"testing"
)

func doc(t *testing.T, jsonStr string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		t.Fatalf("bad test doc: %v", err)
	}
	return v
}

func res(cluster string, d any) FleetResource {
	return FleetResource{Origin: ResourceOrigin{Cluster: cluster, Space: cluster, UnitSlug: "u"}, Doc: d}
}

const hpaJSON = `{
  "apiVersion": "autoscaling/v2", "kind": "HorizontalPodAutoscaler",
  "metadata": {"name": "web", "namespace": "prod"},
  "spec": {"scaleTargetRef": {"kind": "Deployment", "name": "web"}, "minReplicas": 3, "maxReplicas": 10}
}`

const pinnedHPAJSON = `{
  "apiVersion": "autoscaling/v2", "kind": "HorizontalPodAutoscaler",
  "metadata": {"name": "api", "namespace": "prod"},
  "spec": {"scaleTargetRef": {"kind": "Deployment", "name": "api"}, "minReplicas": 4, "maxReplicas": 4}
}`

const scaledObjectJSON = `{
  "apiVersion": "keda.sh/v1alpha1", "kind": "ScaledObject",
  "metadata": {"name": "worker", "namespace": "prod"},
  "spec": {"scaleTargetRef": {"name": "worker"}, "minReplicaCount": 1, "maxReplicaCount": 20}
}`

func deploymentJSON(name string, replicas int, appLabel string) string {
	return `{
  "apiVersion": "apps/v1", "kind": "Deployment",
  "metadata": {"name": "` + name + `", "namespace": "prod"},
  "spec": {"replicas": ` + itoa(int64(replicas)) + `,
    "template": {"metadata": {"labels": {"app": "` + appLabel + `"}}}}
}`
}

func pdbJSON(name, minAvailable, appLabel string) string {
	return `{
  "apiVersion": "policy/v1", "kind": "PodDisruptionBudget",
  "metadata": {"name": "` + name + `", "namespace": "prod"},
  "spec": {"minAvailable": ` + minAvailable + `, "selector": {"matchLabels": {"app": "` + appLabel + `"}}}
}`
}

func TestBuildFleet(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		res("c1", doc(t, hpaJSON)),
		res("c1", doc(t, scaledObjectJSON)),
		res("c1", doc(t, deploymentJSON("web", 3, "web"))),
		res("c1", doc(t, pdbJSON("web", "2", "web"))),
	})
	c := clusters["c1"]
	if c == nil {
		t.Fatal("cluster c1 missing")
	}
	if len(c.Autoscalers) != 2 || len(c.Workloads) != 1 || len(c.PDBs) != 1 {
		t.Fatalf("counts: autoscalers=%d workloads=%d pdbs=%d", len(c.Autoscalers), len(c.Workloads), len(c.PDBs))
	}
	var hpa *Autoscaler
	for _, a := range c.Autoscalers {
		if a.Kind == KindHPA {
			hpa = a
		}
	}
	if hpa == nil || hpa.Min == nil || *hpa.Min != 3 || hpa.Max == nil || *hpa.Max != 10 {
		t.Fatalf("hpa parse: %+v", hpa)
	}
	if hpa.TargetName != "web" {
		t.Fatalf("hpa target: %q", hpa.TargetName)
	}
}

func TestFindings_Pinned(t *testing.T) {
	clusters := BuildFleet([]FleetResource{res("c1", doc(t, pinnedHPAJSON))})
	fs := Findings(clusters)
	if !hasFinding(fs, "autoscaler-pinned", "api") {
		t.Fatalf("expected autoscaler-pinned for api, got %+v", fs)
	}
}

func TestFindings_NoAutoscaler(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		res("c1", doc(t, deploymentJSON("lonely", 2, "lonely"))),
		res("c1", doc(t, hpaJSON)), // targets "web", not "lonely"
		res("c1", doc(t, deploymentJSON("web", 3, "web"))),
	})
	fs := Findings(clusters)
	if !hasFinding(fs, "no-autoscaler", "lonely") {
		t.Fatalf("expected no-autoscaler for lonely, got %+v", fs)
	}
	if hasFinding(fs, "no-autoscaler", "web") {
		t.Fatalf("web is autoscaled; should not be flagged: %+v", fs)
	}
}

func TestFindings_PDBBlocksMinScale(t *testing.T) {
	// HPA minReplicas=3, PDB minAvailable=3 → at min scale no pod may be evicted.
	clusters := BuildFleet([]FleetResource{
		res("c1", doc(t, hpaJSON)),
		res("c1", doc(t, deploymentJSON("web", 3, "web"))),
		res("c1", doc(t, pdbJSON("web-pdb", "3", "web"))),
	})
	fs := Findings(clusters)
	if !hasFinding(fs, "pdb-blocks-min-scale", "web") {
		t.Fatalf("expected pdb-blocks-min-scale for web, got %+v", fs)
	}

	// minAvailable=2 < minReplicas=3 → OK, no finding.
	clusters = BuildFleet([]FleetResource{
		res("c1", doc(t, hpaJSON)),
		res("c1", doc(t, deploymentJSON("web", 3, "web"))),
		res("c1", doc(t, pdbJSON("web-pdb", "2", "web"))),
	})
	if hasFinding(Findings(clusters), "pdb-blocks-min-scale", "web") {
		t.Fatalf("minAvailable 2 < minReplicas 3 should not block")
	}
}

func TestFindings_PDBPercentBlocks(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		res("c1", doc(t, hpaJSON)),
		res("c1", doc(t, deploymentJSON("web", 3, "web"))),
		res("c1", doc(t, pdbJSON("web-pdb", `"100%"`, "web"))),
	})
	if !hasFinding(Findings(clusters), "pdb-blocks-min-scale", "web") {
		t.Fatalf("minAvailable 100%% should block voluntary eviction")
	}
}

func TestSelectorMatches(t *testing.T) {
	sel := LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	if !sel.Matches(map[string]string{"app": "web", "tier": "fe"}) {
		t.Fatal("should match superset labels")
	}
	if sel.Matches(map[string]string{"app": "api"}) {
		t.Fatal("should not match different value")
	}
	expr := LabelSelector{MatchExpressions: []SelectorRequirement{{Key: "tier", Operator: "In", Values: []string{"fe", "be"}}}}
	if !expr.Matches(map[string]string{"tier": "be"}) {
		t.Fatal("In should match")
	}
	if expr.Matches(map[string]string{"tier": "db"}) {
		t.Fatal("In should not match db")
	}
}

func hasFinding(fs []Finding, analyzer, name string) bool {
	for _, f := range fs {
		if f.Analyzer == analyzer && f.Name == name {
			return true
		}
	}
	return false
}
