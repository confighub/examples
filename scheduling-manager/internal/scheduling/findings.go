// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package scheduling

import (
	"fmt"
	"sort"
	"strings"
)

// Severity ranks a finding for triage.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

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

// Finding is one placement issue on one workload.
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

// Analyzers.
const analyzerPlacement = "placement"

// Findings computes placement anti-patterns across the fleet, most-severe first.
//
// v1 flags the tractable, cluster-fact-free anti-pattern: a workload that
// tolerates a taint but does not constrain where it lands (no nodeSelector and no
// required node affinity), so it may schedule onto general nodes — usually not the
// intent of adding a toleration. (Checks that need cluster node-pool / taint facts
// — e.g. a nodeSelector for a pool no cluster advertises, or a nodeSelector onto a
// tainted pool without a matching toleration — are deferred until those Target
// facts exist.)
func Findings(clusters map[string]*ClusterWorkloads) []Finding {
	var out []Finding
	for _, c := range clusters {
		for _, w := range c.Workloads {
			if w.HasTolerations() && !w.Constrained() {
				out = append(out, Finding{
					Severity:  SeverityMedium,
					Analyzer:  analyzerPlacement,
					Cluster:   w.Origin.Cluster,
					Namespace: w.Namespace,
					Kind:      w.Kind,
					Name:      w.Name,
					Space:     w.Origin.Space,
					UnitSlug:  w.Origin.UnitSlug,
					Message: fmt.Sprintf("tolerates taint(s) [%s] but has no nodeSelector or required node affinity — may schedule onto general nodes",
						strings.Join(w.TolerationKeys(), ", ")),
				})
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
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}
