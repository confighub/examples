// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package workload

import (
	"encoding/json"
	"testing"
)

// parseDoc decodes a JSON manifest into the generic map the engine consumes.
func parseDoc(t *testing.T, manifest string) any {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(manifest), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc
}

// scoreOf builds a one-resource fleet, scores it, and returns the single
// workload's per-dimension status map.
func scoreOf(t *testing.T, manifest string) map[string]Status {
	t.Helper()
	clusters := BuildFleet([]FleetResource{{
		Origin: ResourceOrigin{Cluster: "c1"},
		Doc:    parseDoc(t, manifest),
	}})
	scores := ScoreFleet(clusters, nil)
	if len(scores) != 1 {
		t.Fatalf("want 1 workload, got %d", len(scores))
	}
	out := map[string]Status{}
	for _, d := range scores[0].Dimensions {
		out[d.Dimension] = d.Status
	}
	return out
}

// hardenedDeployment is a Deployment that passes every dimension: locked-down
// security context (pod defaults folded into the container), cpu+memory
// requests/limits, both probes, and FallbackToLogsOnError.
const hardenedDeployment = `{
  "apiVersion": "apps/v1", "kind": "Deployment",
  "metadata": {"name": "web", "namespace": "shop"},
  "spec": {
    "replicas": 3,
    "template": {
      "metadata": {"labels": {"app": "web"}},
      "spec": {
        "automountServiceAccountToken": false,
        "securityContext": {"runAsNonRoot": true, "seccompProfile": {"type": "RuntimeDefault"}},
        "affinity": {"podAntiAffinity": {"preferredDuringSchedulingIgnoredDuringExecution": [{"weight": 1}]}},
        "containers": [{
          "name": "web",
          "securityContext": {
            "readOnlyRootFilesystem": true,
            "allowPrivilegeEscalation": false,
            "privileged": false,
            "capabilities": {"drop": ["ALL"]}
          },
          "resources": {
            "requests": {"cpu": "100m", "memory": "128Mi"},
            "limits": {"cpu": "500m", "memory": "256Mi"}
          },
          "livenessProbe": {"httpGet": {"path": "/healthz", "port": 8080}},
          "readinessProbe": {"httpGet": {"path": "/ready", "port": 8080}},
          "terminationMessagePolicy": "FallbackToLogsOnError"
        }]
      }
    }
  }
}`

// matchingPDB covers the hardened Deployment's pods (app=web) with a loose
// minAvailable so it doesn't block all evictions.
const matchingPDB = `{
  "apiVersion": "policy/v1", "kind": "PodDisruptionBudget",
  "metadata": {"name": "web-pdb", "namespace": "shop"},
  "spec": {"minAvailable": 1, "selector": {"matchLabels": {"app": "web"}}}
}`

func TestScore_Hardened_AllPass(t *testing.T) {
	// Availability needs the cluster's PDBs, so score the whole fleet (Deployment
	// + matching PDB) and pick out the Deployment.
	clusters := BuildFleet([]FleetResource{
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: parseDoc(t, hardenedDeployment)},
		{Origin: ResourceOrigin{Cluster: "c1"}, Doc: parseDoc(t, matchingPDB)},
	})
	scores := ScoreFleet(clusters, nil)
	if len(scores) != 1 {
		t.Fatalf("want 1 workload, got %d", len(scores))
	}
	if scores[0].Overall != StatusPass {
		t.Fatalf("overall: want pass, got %s", scores[0].Overall)
	}
	for _, d := range scores[0].Dimensions {
		if d.Status != StatusPass {
			t.Errorf("dimension %s: want pass, got %s (%v)", d.Dimension, d.Status, d.Issues)
		}
	}
}

func TestScore_Dimensions(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     map[string]Status
	}{
		{
			name: "root container with no limits/probes fails security+resources+probes",
			manifest: `{
              "apiVersion": "apps/v1", "kind": "Deployment",
              "metadata": {"name": "bad", "namespace": "x"},
              "spec": {"replicas": 1, "template": {"spec": {"containers": [{"name": "c"}]}}}
            }`,
			want: map[string]Status{
				DimSecurity:  StatusFail, // no runAsNonRoot, no allowPrivEsc:false
				DimResources: StatusFail, // no memory limit
				DimProbes:    StatusFail, // no readiness probe
				DimHygiene:   StatusWarn, // no terminationMessagePolicy
			},
		},
		{
			name: "missing only cpu limit + requests warns resources (memory limit set)",
			manifest: `{
              "apiVersion": "apps/v1", "kind": "Deployment",
              "metadata": {"name": "w", "namespace": "x"},
              "spec": {"replicas": 1, "template": {"spec": {"containers": [{
                "name": "c",
                "resources": {"limits": {"memory": "256Mi"}}
              }]}}}
            }`,
			want: map[string]Status{DimResources: StatusWarn},
		},
		{
			name: "privileged container fails security",
			manifest: `{
              "apiVersion": "apps/v1", "kind": "Deployment",
              "metadata": {"name": "p", "namespace": "x"},
              "spec": {"replicas": 1, "template": {"spec": {"containers": [{
                "name": "c",
                "securityContext": {"privileged": true, "runAsNonRoot": true,
                  "allowPrivilegeEscalation": false, "readOnlyRootFilesystem": true,
                  "capabilities": {"drop": ["ALL"]}, "seccompProfile": {"type": "RuntimeDefault"}}
              }]}}}
            }`,
			want: map[string]Status{DimSecurity: StatusFail},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scoreOf(t, tc.manifest)
			for dim, want := range tc.want {
				if got[dim] != want {
					t.Errorf("dimension %s: want %s, got %s", dim, want, got[dim])
				}
			}
		})
	}
}

// Probes don't apply to Jobs; the probes dimension should pass regardless.
func TestScore_ProbesSkippedForJob(t *testing.T) {
	manifest := `{
      "apiVersion": "batch/v1", "kind": "Job",
      "metadata": {"name": "j", "namespace": "x"},
      "spec": {"template": {"spec": {"containers": [{"name": "c"}]}}}
    }`
	got := scoreOf(t, manifest)
	if got[DimProbes] != StatusPass {
		t.Errorf("probes for Job: want pass (n/a), got %s", got[DimProbes])
	}
}
