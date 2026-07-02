// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package autoscale

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Empty reports whether the selector has no terms.
func (s LabelSelector) Empty() bool {
	return len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0
}

// Matches reports whether labels satisfy the selector (matchLabels + In/NotIn/
// Exists/DoesNotExist).
func (s LabelSelector) Matches(labels map[string]string) bool {
	for k, v := range s.MatchLabels {
		if labels[k] != v {
			return false
		}
	}
	for _, r := range s.MatchExpressions {
		val, present := labels[r.Key]
		switch r.Operator {
		case "In":
			if !present || !contains(r.Values, val) {
				return false
			}
		case "NotIn":
			if present && contains(r.Values, val) {
				return false
			}
		case "Exists":
			if !present {
				return false
			}
		case "DoesNotExist":
			if present {
				return false
			}
		}
	}
	return true
}

func contains(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

// Severity ranks a finding for triage.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// AtLeast reports whether s is at least as severe as threshold.
func AtLeast(s, threshold Severity) bool {
	return severityRank(s) >= severityRank(threshold)
}

func severityRank(s Severity) int {
	switch s {
	case SeverityHigh:
		return 2
	case SeverityMedium:
		return 1
	default:
		return 0
	}
}

// Finding is one autoscaling issue.
type Finding struct {
	Severity  Severity `json:"severity"`
	Analyzer  string   `json:"analyzer"`
	Cluster   string   `json:"cluster"`
	Namespace string   `json:"namespace,omitempty"`
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Space     string   `json:"space"`
	UnitSlug  string   `json:"unitSlug"`
	Message   string   `json:"message"`
}

// autoscalerForWorkload returns the autoscaler targeting a workload (by
// scaleTargetRef.name in the same namespace), or nil.
func autoscalerForWorkload(w *WorkloadEntity, autoscalers []*Autoscaler) *Autoscaler {
	for _, a := range autoscalers {
		if a.Namespace == w.Namespace && a.TargetName == w.Name {
			return a
		}
	}
	return nil
}

// Findings computes autoscaling findings across the fleet, most-severe first:
//   - autoscaler-pinned (medium): an HPA/ScaledObject with min == max (can't scale)
//   - pdb-blocks-min-scale (medium): a PDB whose minAvailable >= the autoscaler's
//     minReplicas, so at minimum scale no pod may be voluntarily evicted (the
//     cross-resource HPA-vs-PDB check)
//   - no-autoscaler (low): a Deployment/StatefulSet not targeted by any autoscaler
func Findings(clusters map[string]*ClusterAutoscale) []Finding {
	var out []Finding
	for _, c := range clusters {
		for _, a := range c.Autoscalers {
			if a.Pinned() {
				out = append(out, Finding{
					Severity: SeverityMedium, Analyzer: "autoscaler-pinned",
					Cluster: a.Origin.Cluster, Namespace: a.Namespace, Kind: string(a.Kind), Name: a.Name,
					Space: a.Origin.Space, UnitSlug: a.Origin.UnitSlug,
					Message: fmt.Sprintf("min == max (%d) — the autoscaler can't actually scale", *a.Min),
				})
			}
		}
		for _, w := range c.Workloads {
			a := autoscalerForWorkload(w, c.Autoscalers)
			if a == nil {
				out = append(out, Finding{
					Severity: SeverityLow, Analyzer: "no-autoscaler",
					Cluster: w.Origin.Cluster, Namespace: w.Namespace, Kind: w.Kind, Name: w.Name,
					Space: w.Origin.Space, UnitSlug: w.Origin.UnitSlug,
					Message: "no HorizontalPodAutoscaler or ScaledObject targets this workload",
				})
				continue
			}
			// Cross-resource: does a PDB block voluntary disruption at minimum scale?
			if a.Min == nil {
				continue
			}
			for _, p := range c.PDBs {
				if p.Namespace != w.Namespace || !p.Selector.Matches(w.PodLabels) {
					continue
				}
				if pdbBlocksAtScale(p, *a.Min) {
					out = append(out, Finding{
						Severity: SeverityMedium, Analyzer: "pdb-blocks-min-scale",
						Cluster: w.Origin.Cluster, Namespace: w.Namespace, Kind: w.Kind, Name: w.Name,
						Space: w.Origin.Space, UnitSlug: w.Origin.UnitSlug,
						Message: fmt.Sprintf("PDB %q minAvailable (%s) >= %s minReplicas (%d) — at minimum scale no pod may be voluntarily evicted",
							p.Name, p.MinAvailable, a.Kind, *a.Min),
					})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Analyzer < out[j].Analyzer
	})
	return out
}

// pdbBlocksAtScale reports whether a PDB's minAvailable prevents all voluntary
// disruption when the workload is at minReplicas: minAvailable of "100%", or an
// integer minAvailable >= minReplicas.
func pdbBlocksAtScale(p *PDBEntity, minReplicas int64) bool {
	if p.MinAvailable == "" {
		return false
	}
	if strings.TrimSpace(p.MinAvailable) == "100%" {
		return true
	}
	if n, err := strconv.ParseInt(p.MinAvailable, 10, 64); err == nil {
		return n >= minReplicas
	}
	return false
}
