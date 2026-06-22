// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// RBAC hygiene analyzers, mirroring the published fleet-audit findings:
// wildcards, dangerous verbs, risky grants, cluster-admin bindings, orphaned
// bindings, and unbound ServiceAccounts. Pure functions over the snapshot;
// enforcement (gates) is server-side via Triggers — these are the analysis-only
// complement. Faithful Go port of the web app's findings.ts.

package rbac

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Severity ranks a finding's importance.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// Finding is one hygiene issue discovered by an analyzer.
type Finding struct {
	ID           string         `json:"id"` // stable id for rendering and dedup
	Analyzer     string         `json:"analyzer"`
	Severity     Severity       `json:"severity"`
	Cluster      string         `json:"cluster"`
	Origin       ResourceOrigin `json:"origin"`
	ResourceKind string         `json:"resourceKind"`
	ResourceName string         `json:"resourceName"`
	Namespace    string         `json:"namespace,omitempty"`
	Message      string         `json:"message"`
}

var escalationVerbs = map[string]bool{"escalate": true, "bind": true, "impersonate": true}

func newFinding(analyzer string, severity Severity, origin ResourceOrigin, resourceKind, resourceName, message, namespace string) Finding {
	return Finding{
		ID:           fmt.Sprintf("%s:%s:%s:%s:%s", analyzer, origin.Cluster, resourceKind, namespace, resourceName),
		Analyzer:     analyzer,
		Severity:     severity,
		Cluster:      origin.Cluster,
		Origin:       origin,
		ResourceKind: resourceKind,
		ResourceName: resourceName,
		Namespace:    namespace,
		Message:      message,
	}
}

func wildcardFindings(cluster *ClusterRbac) []Finding {
	var out []Finding
	for _, role := range cluster.Roles {
		var parts []string
		for i, rule := range role.Rules {
			if slices.Contains(rule.Verbs, "*") {
				parts = append(parts, fmt.Sprintf("rule %d: wildcard verbs", i))
			}
			if slices.Contains(rule.Resources, "*") {
				parts = append(parts, fmt.Sprintf("rule %d: wildcard resources", i))
			}
			if slices.Contains(rule.APIGroups, "*") {
				parts = append(parts, fmt.Sprintf("rule %d: wildcard apiGroups", i))
			}
		}
		if len(parts) > 0 {
			out = append(out, newFinding("wildcard-rules", SeverityHigh, role.Origin, role.Kind, role.Name,
				fmt.Sprintf("Wildcard permissions (%s). Enumerate the specific verbs/resources needed.", strings.Join(parts, "; ")),
				role.Namespace))
		}
	}
	return out
}

func escalationFindings(cluster *ClusterRbac) []Finding {
	var out []Finding
	for _, role := range cluster.Roles {
		var verbs []string
		seen := map[string]bool{}
		for _, r := range role.Rules {
			for _, v := range r.Verbs {
				if escalationVerbs[v] && !seen[v] {
					seen[v] = true
					verbs = append(verbs, v)
				}
			}
		}
		if len(verbs) > 0 {
			out = append(out, newFinding("privilege-escalation-verbs", SeverityHigh, role.Origin, role.Kind, role.Name,
				fmt.Sprintf("Grants privilege-escalation verb(s): %s.", strings.Join(verbs, ", ")),
				role.Namespace))
		}
	}
	return out
}

var writeVerbs = map[string]bool{
	"create": true, "update": true, "patch": true, "delete": true, "deletecollection": true, "*": true,
}

// riskyGrantFindings flags grants that aren't wildcards but deserve eyes:
// secrets, exec, webhooks, CRDs.
func riskyGrantFindings(cluster *ClusterRbac) []Finding {
	var out []Finding
	for _, role := range cluster.Roles {
		var risks []string
		seen := map[string]bool{}
		add := func(r string) {
			if !seen[r] {
				seen[r] = true
				risks = append(risks, r)
			}
		}
		for _, rule := range role.Rules {
			writes := false
			for _, v := range rule.Verbs {
				if writeVerbs[v] {
					writes = true
					break
				}
			}
			if slices.Contains(rule.Resources, "secrets") {
				if writes {
					add("secrets write")
				} else {
					add("secrets read")
				}
			}
			if slices.Contains(rule.Resources, "pods/exec") || slices.Contains(rule.Resources, "pods/attach") {
				add("pod exec/attach")
			}
			if writes && (slices.Contains(rule.APIGroups, "admissionregistration.k8s.io") || slices.Contains(rule.APIGroups, "apiextensions.k8s.io")) {
				add("webhook/CRD write")
			}
		}
		if len(risks) > 0 {
			out = append(out, newFinding("risky-grants", SeverityMedium, role.Origin, role.Kind, role.Name,
				fmt.Sprintf("Sensitive access: %s.", strings.Join(risks, ", ")),
				role.Namespace))
		}
	}
	return out
}

func clusterAdminFindings(cluster *ClusterRbac) []Finding {
	var out []Finding
	for _, binding := range cluster.Bindings {
		isSuperuser := binding.RoleRef.Name == "cluster-admin"
		if !isSuperuser {
			if role := ResolveRoleRef(binding, cluster); role != nil {
				for _, r := range EffectiveRules(role, cluster) {
					if slices.Contains(r.Verbs, "*") && slices.Contains(r.Resources, "*") && slices.Contains(r.APIGroups, "*") {
						isSuperuser = true
						break
					}
				}
			}
		}
		if !isSuperuser {
			continue
		}
		subjects := "(no subjects)"
		if len(binding.Subjects) > 0 {
			keys := make([]string, len(binding.Subjects))
			for i, s := range binding.Subjects {
				keys[i] = SubjectKey(s)
			}
			subjects = strings.Join(keys, ", ")
		}
		severity := SeverityMedium
		if binding.Kind == "ClusterRoleBinding" {
			severity = SeverityHigh
		}
		out = append(out, newFinding("cluster-admin-bindings", severity, binding.Origin, binding.Kind, binding.Name,
			fmt.Sprintf("Grants superuser (%s) to: %s.", binding.RoleRef.Name, subjects),
			binding.Namespace))
	}
	return out
}

func orphanedBindingFindings(cluster *ClusterRbac) []Finding {
	var out []Finding
	for _, binding := range cluster.Bindings {
		if ResolveRoleRef(binding, cluster) != nil {
			continue
		}
		if IsBuiltinRoleName(binding.RoleRef.Name) {
			continue
		}
		out = append(out, newFinding("orphaned-bindings", SeverityMedium, binding.Origin, binding.Kind, binding.Name,
			fmt.Sprintf("References %s %q, which does not exist on this cluster. Remove the binding or restore the role.", binding.RoleRef.Kind, binding.RoleRef.Name),
			binding.Namespace))
	}
	return out
}

func unboundServiceAccountFindings(cluster *ClusterRbac) []Finding {
	bound := map[string]bool{}
	for _, binding := range cluster.Bindings {
		for _, s := range binding.Subjects {
			if s.Kind == "ServiceAccount" {
				bound[s.Namespace+"/"+s.Name] = true
			}
		}
	}
	var out []Finding
	for _, sa := range cluster.ServiceAccounts {
		if bound[sa.Namespace+"/"+sa.Name] {
			continue
		}
		out = append(out, newFinding("unbound-service-accounts", SeverityLow, sa.Origin, "ServiceAccount", sa.Name,
			"ServiceAccount has no role bindings in this snapshot — possibly unused.",
			sa.Namespace))
	}
	return out
}

var analyzers = []func(*ClusterRbac) []Finding{
	wildcardFindings,
	escalationFindings,
	riskyGrantFindings,
	clusterAdminFindings,
	orphanedBindingFindings,
	unboundServiceAccountFindings,
}

var severityOrder = map[Severity]int{SeverityHigh: 0, SeverityMedium: 1, SeverityLow: 2}

// AnalyzeFleet runs every analyzer over every cluster, sorted by severity, then
// cluster, then id.
func AnalyzeFleet(clusters map[string]*ClusterRbac) []Finding {
	var out []Finding
	for _, cluster := range clusters {
		for _, analyzer := range analyzers {
			out = append(out, analyzer(cluster)...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if severityOrder[out[i].Severity] != severityOrder[out[j].Severity] {
			return severityOrder[out[i].Severity] < severityOrder[out[j].Severity]
		}
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		return out[i].ID < out[j].ID
	})
	return out
}
