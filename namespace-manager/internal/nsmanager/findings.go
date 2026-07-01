// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"fmt"
	"sort"
)

// Severity levels for findings.
const (
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// Finding is one namespace-governance result attributed to a namespace (and,
// for consistency, a component).
type Finding struct {
	Analyzer  string `json:"analyzer"`
	Severity  string `json:"severity"`
	Cluster   string `json:"cluster,omitempty"`
	Component string `json:"component,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Message   string `json:"message"`
}

// AnalyzeFindings runs the v1 analyzer set over the fleet:
//   - missing-default-deny   (occupied namespace with no default-deny NetworkPolicy)
//   - missing-pod-security   (Namespace object without a pod-security enforce label)
//   - missing-baseline-rbac  (occupied namespace with no RoleBinding)
//   - missing-namespace-object (occupied namespace with no v1/Namespace Unit)
//   - duplicate-namespace    (two Namespace objects colliding on name + Target)
//   - namespace-name-inconsistent (a component's namespace name varies across variants)
//   - pod-security-inconsistent   (a component's pod-security level varies across variants)
func AnalyzeFindings(clusters map[string]*ClusterNamespaces) []Finding {
	var fs []Finding

	// Per-namespace envelope completeness.
	for _, c := range clusters {
		for _, e := range AnalyzeCluster(c) {
			if e.WorkloadCount > 0 && !e.HasNamespaceObject {
				fs = append(fs, Finding{
					Analyzer: "missing-namespace-object", Severity: SeverityMedium,
					Cluster: e.Cluster, Namespace: e.Namespace,
					Message: fmt.Sprintf("namespace %q has %d workload(s) but no v1/Namespace Unit", e.Namespace, e.WorkloadCount),
				})
			}
			if e.HasNamespaceObject && e.PodSecurityEnforce == "" {
				fs = append(fs, Finding{
					Analyzer: "missing-pod-security", Severity: SeverityMedium,
					Cluster: e.Cluster, Namespace: e.Namespace,
					Message: fmt.Sprintf("namespace %q has no pod-security.kubernetes.io/enforce label", e.Namespace),
				})
			}
			if e.WorkloadCount > 0 && !e.HasDefaultDeny {
				fs = append(fs, Finding{
					Analyzer: "missing-default-deny", Severity: SeverityHigh,
					Cluster: e.Cluster, Namespace: e.Namespace,
					Message: fmt.Sprintf("namespace %q has %d workload(s) but no default-deny NetworkPolicy", e.Namespace, e.WorkloadCount),
				})
			}
			if e.WorkloadCount > 0 && !e.HasBaselineRBAC {
				fs = append(fs, Finding{
					Analyzer: "missing-baseline-rbac", Severity: SeverityLow,
					Cluster: e.Cluster, Namespace: e.Namespace,
					Message: fmt.Sprintf("namespace %q has no baseline RBAC (RoleBinding)", e.Namespace),
				})
			}
		}
	}

	// Fleet-level: duplicate namespace name on the same Target.
	for _, d := range DuplicateNamespaces(clusters) {
		fs = append(fs, Finding{
			Analyzer: "duplicate-namespace", Severity: SeverityHigh,
			Cluster: d.Target, Namespace: d.Namespace,
			Message: fmt.Sprintf("namespace %q is defined by %d Units on target %q — a collision", d.Namespace, d.Count, d.Target),
		})
	}

	// Fleet-level: cross-variant consistency.
	for _, cc := range AnalyzeConsistency(clusters) {
		if !cc.NamespaceConsistent {
			fs = append(fs, Finding{
				Analyzer: "namespace-name-inconsistent", Severity: SeverityMedium,
				Component: cc.Component,
				Message:   fmt.Sprintf("component %q uses different namespace names across its variant Spaces: %s", cc.Component, joinQuoted(cc.Namespaces)),
			})
		}
		if !cc.PodSecurityConsistent {
			fs = append(fs, Finding{
				Analyzer: "pod-security-inconsistent", Severity: SeverityLow,
				Component: cc.Component,
				Message:   fmt.Sprintf("component %q uses different pod-security levels across its variant Spaces: %s", cc.Component, joinQuoted(cc.PodSecurityLevels)),
			})
		}
	}

	sort.SliceStable(fs, func(i, j int) bool {
		if severityRank(fs[i].Severity) != severityRank(fs[j].Severity) {
			return severityRank(fs[i].Severity) < severityRank(fs[j].Severity)
		}
		if fs[i].Analyzer != fs[j].Analyzer {
			return fs[i].Analyzer < fs[j].Analyzer
		}
		if fs[i].Cluster != fs[j].Cluster {
			return fs[i].Cluster < fs[j].Cluster
		}
		if fs[i].Component != fs[j].Component {
			return fs[i].Component < fs[j].Component
		}
		return fs[i].Namespace < fs[j].Namespace
	})
	return fs
}

func severityRank(s string) int {
	switch s {
	case SeverityHigh:
		return 0
	case SeverityMedium:
		return 1
	case SeverityLow:
		return 2
	}
	return 3
}
