// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import "sort"

// PlaceholderNamespace is the base-template placeholder value (set-namespace
// replaces it at render). Resources still carrying it are unrendered bases that
// vet-placeholders gates from deploying, so envelope and duplicate analysis skip
// them.
const PlaceholderNamespace = "confighubplaceholder"

// Envelope member identifiers — the policy bundle every occupied namespace
// should carry.
const (
	MemberNamespaceObject = "namespace-object"
	MemberPodSecurity     = "pod-security"
	MemberDefaultDeny     = "default-deny"
	MemberBaselineRBAC    = "baseline-rbac"
)

// NamespaceEnvelope is the per-namespace completeness verdict: which envelope
// members are present, and which are missing.
type NamespaceEnvelope struct {
	Cluster            string   `json:"cluster"`
	Namespace          string   `json:"namespace"`
	HasNamespaceObject bool     `json:"hasNamespaceObject"`
	PodSecurityEnforce string   `json:"podSecurityEnforce,omitempty"`
	HasDefaultDeny     bool     `json:"hasDefaultDeny"`
	HasBaselineRBAC    bool     `json:"hasBaselineRBAC"`
	WorkloadCount      int      `json:"workloadCount"`
	Missing            []string `json:"missing,omitempty"`
	Complete           bool     `json:"complete"`
	// SpaceID and UnitSlug identify the v1/Namespace Unit (when one exists), so a
	// finding on this namespace can be annotated onto its Unit.
	SpaceID  string `json:"spaceId,omitempty"`
	UnitSlug string `json:"unitSlug,omitempty"`
}

// AnalyzeCluster computes the envelope verdict for every namespace in one
// cluster. A namespace is analyzed if it has a Namespace object or any occupant
// (workload, NetworkPolicy, or RBAC object). The placeholder namespace is
// skipped.
func AnalyzeCluster(c *ClusterNamespaces) []NamespaceEnvelope {
	// Index by namespace name.
	nsObjects := map[string]*NamespaceEntity{}
	for _, n := range c.Namespaces {
		nsObjects[n.Name] = n
	}
	defaultDeny := map[string]bool{}
	for _, np := range c.NetworkPolicies {
		if np.IsDefaultDenyIngress() {
			defaultDeny[np.Namespace] = true
		}
	}
	// Baseline RBAC is present when the namespace has a RoleBinding (the
	// operative grant). ServiceAccount/Role alone don't bind anything.
	baselineRBAC := map[string]bool{}
	for _, r := range c.RBAC {
		if r.Kind == "RoleBinding" {
			baselineRBAC[r.Namespace] = true
		}
	}
	workloads := map[string]int{}
	for _, w := range c.Workloads {
		workloads[w.Namespace]++
	}

	// The set of namespaces to analyze: every name that appears as an object or
	// as the namespace of some occupant.
	names := map[string]bool{}
	add := func(ns string) {
		if ns != "" && ns != PlaceholderNamespace {
			names[ns] = true
		}
	}
	for name := range nsObjects {
		add(name)
	}
	for _, np := range c.NetworkPolicies {
		add(np.Namespace)
	}
	for _, r := range c.RBAC {
		add(r.Namespace)
	}
	for _, w := range c.Workloads {
		add(w.Namespace)
	}

	out := make([]NamespaceEnvelope, 0, len(names))
	for ns := range names {
		obj, hasObj := nsObjects[ns]
		e := NamespaceEnvelope{
			Cluster:            c.Cluster,
			Namespace:          ns,
			HasNamespaceObject: hasObj,
			HasDefaultDeny:     defaultDeny[ns],
			HasBaselineRBAC:    baselineRBAC[ns],
			WorkloadCount:      workloads[ns],
		}
		if hasObj {
			e.PodSecurityEnforce = obj.PodSecurityEnforce()
			e.SpaceID = obj.Origin.SpaceID
			e.UnitSlug = obj.Origin.UnitSlug
		}
		if !e.HasNamespaceObject {
			e.Missing = append(e.Missing, MemberNamespaceObject)
		}
		if e.PodSecurityEnforce == "" {
			e.Missing = append(e.Missing, MemberPodSecurity)
		}
		if !e.HasDefaultDeny {
			e.Missing = append(e.Missing, MemberDefaultDeny)
		}
		if !e.HasBaselineRBAC {
			e.Missing = append(e.Missing, MemberBaselineRBAC)
		}
		e.Complete = len(e.Missing) == 0
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace < out[j].Namespace })
	return out
}

// AnalyzeFleet computes envelope verdicts across every cluster, sorted by
// (cluster, namespace).
func AnalyzeFleet(clusters map[string]*ClusterNamespaces) []NamespaceEnvelope {
	var out []NamespaceEnvelope
	for _, c := range clusters {
		out = append(out, AnalyzeCluster(c)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

// DuplicateNamespace is a finding: two or more Namespace objects that resolve to
// the same name and are bound to the same Target (would deploy to the same
// cluster) — a collision.
type DuplicateNamespace struct {
	Target    string   `json:"target"`
	Namespace string   `json:"namespace"`
	Count     int      `json:"count"`
	UnitSlugs []string `json:"unitSlugs"`
}

// DuplicateNamespaces flags Namespace objects that collide on (Target, name).
// Only Target-bound Namespaces are considered (an unbound base doesn't deploy);
// the placeholder name is exempt (an unrendered base, gated by vet-placeholders).
func DuplicateNamespaces(clusters map[string]*ClusterNamespaces) []DuplicateNamespace {
	type key struct{ target, name string }
	groups := map[key][]string{}
	for _, c := range clusters {
		for _, n := range c.Namespaces {
			if n.Origin.Target == "" || n.Name == PlaceholderNamespace {
				continue
			}
			k := key{n.Origin.Target, n.Name}
			groups[k] = append(groups[k], n.Origin.UnitSlug)
		}
	}
	var out []DuplicateNamespace
	for k, slugs := range groups {
		if len(slugs) < 2 {
			continue
		}
		sort.Strings(slugs)
		out = append(out, DuplicateNamespace{
			Target: k.target, Namespace: k.name, Count: len(slugs), UnitSlugs: slugs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}
