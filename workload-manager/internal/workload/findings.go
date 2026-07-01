// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package workload

import "sort"

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

// Finding is one ranked readiness issue on one workload, with enough origin to
// deep-link and (later) annotate the offending Unit.
type Finding struct {
	Severity  Severity `json:"severity"`
	Analyzer  string   `json:"analyzer"` // the dimension: security | resources | probes | hygiene | availability
	Cluster   string   `json:"cluster"`
	Namespace string   `json:"namespace,omitempty"`
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Space     string   `json:"space"`
	UnitSlug  string   `json:"unitSlug"`
	Message   string   `json:"message"`
}

// severityFor maps a (dimension, issue-status) pair to a triage severity. A
// failing security / resources / availability check is high; a failing probe is
// medium; warnings are one step down (hygiene is always low).
func severityFor(dimension string, st Status) Severity {
	switch dimension {
	case DimSecurity, DimResources, DimAvailability:
		if st == StatusFail {
			return SeverityHigh
		}
		return SeverityMedium
	case DimProbes:
		if st == StatusFail {
			return SeverityMedium
		}
		return SeverityLow
	default: // hygiene
		return SeverityLow
	}
}

// Findings flattens the fleet readiness scorecard into severity-ranked findings,
// sorted most-severe first, then by (cluster, namespace, kind, name, analyzer).
func Findings(clusters map[string]*ClusterWorkloads) []Finding {
	var out []Finding
	for _, s := range ScoreFleet(clusters, nil) {
		for _, d := range s.Dimensions {
			for _, issue := range d.Issues {
				out = append(out, Finding{
					Severity:  severityFor(d.Dimension, issue.Status),
					Analyzer:  d.Dimension,
					Cluster:   s.Cluster,
					Namespace: s.Namespace,
					Kind:      s.Kind,
					Name:      s.Name,
					Space:     s.Space,
					UnitSlug:  s.UnitSlug,
					Message:   issue.Message,
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
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Analyzer < out[j].Analyzer
	})
	return out
}
