// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package workload

import (
	"encoding/json"
	"testing"
)

func buildOne(t *testing.T, manifest string) *WorkloadEntity {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(manifest), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	clusters := BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "c1"}, Doc: doc}})
	c := clusters["c1"]
	if c == nil || len(c.Workloads) != 1 {
		t.Fatalf("want 1 workload in cluster c1, got %+v", c)
	}
	return c.Workloads[0]
}

// The pod securityContext (runAsNonRoot, seccomp) defaults down to a container
// that doesn't set them itself.
func TestParse_EffectiveSecurityContextFoldsDown(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "web", "namespace": "x"},
      "spec": {"replicas": 2, "template": {"spec": {
        "securityContext": {"runAsNonRoot": true, "seccompProfile": {"type": "RuntimeDefault"}},
        "containers": [{"name": "web"}]
      }}}
    }`)
	if len(w.Containers) != 1 {
		t.Fatalf("want 1 container, got %d", len(w.Containers))
	}
	c := w.Containers[0]
	if c.RunAsNonRoot == nil || !*c.RunAsNonRoot {
		t.Errorf("runAsNonRoot should fold down from pod, got %v", c.RunAsNonRoot)
	}
	if c.SeccompProfileType != "RuntimeDefault" {
		t.Errorf("seccomp should fold down from pod, got %q", c.SeccompProfileType)
	}
	if w.Replicas == nil || *w.Replicas != 2 {
		t.Errorf("replicas: want 2, got %v", w.Replicas)
	}
	if !w.MultiReplica() {
		t.Errorf("MultiReplica: want true for replicas=2")
	}
}

// The container securityContext overrides the pod default.
func TestParse_ContainerOverridesPod(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "web", "namespace": "x"},
      "spec": {"template": {"spec": {
        "securityContext": {"runAsNonRoot": true},
        "containers": [{"name": "web", "securityContext": {"runAsNonRoot": false}}]
      }}}
    }`)
	c := w.Containers[0]
	if c.RunAsNonRoot == nil || *c.RunAsNonRoot {
		t.Errorf("container runAsNonRoot:false should override pod true, got %v", c.RunAsNonRoot)
	}
}

// CronJob's pod template lives under spec.jobTemplate.spec.template.spec.
func TestParse_CronJobPodTemplatePath(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "batch/v1", "kind": "CronJob",
      "metadata": {"name": "cron", "namespace": "x"},
      "spec": {"jobTemplate": {"spec": {"template": {"spec": {
        "containers": [{"name": "task", "resources": {"limits": {"memory": "64Mi"}}}]
      }}}}}
    }`)
	if len(w.Containers) != 1 || w.Containers[0].Name != "task" {
		t.Fatalf("CronJob container not parsed from jobTemplate path: %+v", w.Containers)
	}
	if !w.Containers[0].HasMemoryLimit {
		t.Errorf("CronJob container memory limit not parsed")
	}
}

func TestParse_PDBSelectorAndPolicy(t *testing.T) {
	clusters := BuildFleet([]FleetResource{{
		Origin: ResourceOrigin{Cluster: "c1"},
		Doc: mustDoc(t, `{
          "apiVersion": "policy/v1", "kind": "PodDisruptionBudget",
          "metadata": {"name": "web-pdb", "namespace": "shop"},
          "spec": {"minAvailable": 2, "selector": {"matchLabels": {"app": "web"}}}
        }`),
	}})
	pdbs := clusters["c1"].PDBs
	if len(pdbs) != 1 {
		t.Fatalf("want 1 pdb, got %d", len(pdbs))
	}
	p := pdbs[0]
	if p.MinAvailable != "2" {
		t.Errorf("minAvailable: want \"2\", got %q", p.MinAvailable)
	}
	if p.Selector.MatchLabels["app"] != "web" {
		t.Errorf("selector matchLabels: got %v", p.Selector.MatchLabels)
	}
}

// hasAntiAffinity / hasTopologySpread presence detection.
func TestParse_SpreadPresence(t *testing.T) {
	w := buildOne(t, `{
      "apiVersion": "apps/v1", "kind": "Deployment",
      "metadata": {"name": "web", "namespace": "x"},
      "spec": {"template": {"spec": {
        "topologySpreadConstraints": [{"maxSkew": 1, "topologyKey": "zone"}],
        "containers": [{"name": "web"}]
      }}}
    }`)
	if !w.HasTopologySpread {
		t.Errorf("HasTopologySpread: want true")
	}
	if w.HasAntiAffinity {
		t.Errorf("HasAntiAffinity: want false (none set)")
	}
}

func mustDoc(t *testing.T, s string) any {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc
}
