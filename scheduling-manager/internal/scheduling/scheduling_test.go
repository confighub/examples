// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package scheduling

import (
	"encoding/json"
	"testing"
)

func buildOne(t *testing.T, manifest string) *WorkloadPlacement {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(manifest), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	clusters := BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc}})
	c := clusters["c1"]
	if c == nil || len(c.Workloads) != 1 {
		t.Fatalf("want 1 workload, got %+v", c)
	}
	return c.Workloads[0]
}

func TestParse_Placement(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "gpu-job", "namespace": "ml"},
      "spec": {"template": {"spec": {
        "nodeSelector": {"pool": "gpu"},
        "tolerations": [{"key": "nvidia.com/gpu", "operator": "Exists", "effect": "NoSchedule"}],
        "affinity": {"nodeAffinity": {"requiredDuringSchedulingIgnoredDuringExecution": {"nodeSelectorTerms": []}}},
        "containers": [{"name": "app"}]
      }}}
    }`)
	if w.NodeSelector["pool"] != "gpu" {
		t.Errorf("nodeSelector not parsed: %v", w.NodeSelector)
	}
	if len(w.Tolerations) != 1 || w.Tolerations[0].Key != "nvidia.com/gpu" {
		t.Errorf("tolerations not parsed: %v", w.Tolerations)
	}
	if !w.HasRequiredNodeAffinity {
		t.Errorf("required node affinity not detected")
	}
	if !w.Constrained() {
		t.Errorf("workload with nodeSelector should be constrained")
	}
}

func TestParse_CronJobPath(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "batch/v1", "kind": "CronJob",
      "metadata": {"name": "cron", "namespace": "x"},
      "spec": {"jobTemplate": {"spec": {"template": {"spec": {
        "nodeSelector": {"pool": "batch"},
        "containers": [{"name": "task"}]
      }}}}}
    }`)
	if w.NodeSelector["pool"] != "batch" {
		t.Errorf("CronJob nodeSelector not parsed from jobTemplate path: %v", w.NodeSelector)
	}
}

func TestConstrained_TolerationAloneIsNotConstrained(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "d", "namespace": "x"},
      "spec": {"template": {"spec": {
        "tolerations": [{"key": "spot", "operator": "Exists"}],
        "containers": [{"name": "app"}]
      }}}
    }`)
	if w.Constrained() {
		t.Errorf("tolerations alone must not count as constrained")
	}
	if !w.HasTolerations() {
		t.Errorf("tolerations should be detected")
	}
}

func TestFindings_TolerationWithoutPlacement(t *testing.T) {
	// Tolerates a taint but pins nothing -> flagged.
	unconstrained := buildFleet(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "loose", "namespace": "x"},
      "spec": {"template": {"spec": {
        "tolerations": [{"key": "gpu", "operator": "Exists"}],
        "containers": [{"name": "app"}]
      }}}
    }`)
	fs := Findings(unconstrained)
	if len(fs) != 1 || fs[0].Analyzer != analyzerPlacement || fs[0].Severity != SeverityMedium {
		t.Fatalf("expected one medium placement finding, got %+v", fs)
	}

	// Tolerates AND pins with a nodeSelector -> not flagged.
	constrained := buildFleet(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "tight", "namespace": "x"},
      "spec": {"template": {"spec": {
        "nodeSelector": {"pool": "gpu"},
        "tolerations": [{"key": "gpu", "operator": "Exists"}],
        "containers": [{"name": "app"}]
      }}}
    }`)
	if fs := Findings(constrained); len(fs) != 0 {
		t.Errorf("constrained workload should not be flagged, got %+v", fs)
	}
}

func buildFleet(t *testing.T, manifest string) map[string]*ClusterWorkloads {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(manifest), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc}})
}
