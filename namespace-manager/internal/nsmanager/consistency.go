// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"sort"
	"strings"
)

// ComponentLabel is the standard Space label that groups a component's variant
// Spaces (environments / regions / clusters). Consistency is a per-Component
// property across those Spaces.
const ComponentLabel = "Component"

// VariantNamespace is one variant Space's view of a component's namespace: the
// namespace(s) its resources declare and the pod-security level on its Namespace
// object.
type VariantNamespace struct {
	Space              string   `json:"space"`
	Cluster            string   `json:"cluster"`
	Namespaces         []string `json:"namespaces"`
	PodSecurityEnforce string   `json:"podSecurityEnforce,omitempty"`
}

// NamespaceMismatch is a workload a component runs on more than one cluster,
// in a different namespace on each.
type NamespaceMismatch struct {
	// Workload is Kind/name.
	Workload string `json:"workload"`
	// Namespaces maps each compared variant Space running the workload to the
	// namespace(s) it runs in there.
	Namespaces map[string][]string `json:"namespaces"`
}

// ComponentConsistency is the cross-variant verdict for one component: whether
// each of its workloads runs in the same namespace on every cluster, and its
// pod-security level is the same in every variant Space. This is the
// fleet-wide read prior art (per-cluster controllers, per-resource validators)
// cannot do.
//
// Namespaces lists every name the component's variants use, and is no verdict
// by itself: a component may span several namespaces, a variant may run parts
// another does not, and variants sharing a cluster (staging beside prod, a
// preview per pull request) have to use different namespaces.
type ComponentConsistency struct {
	Component             string              `json:"component"`
	Variants              []VariantNamespace  `json:"variants"`
	Namespaces            []string            `json:"namespaces"`
	PodSecurityLevels     []string            `json:"podSecurityLevels,omitempty"`
	NamespaceMismatches   []NamespaceMismatch `json:"namespaceMismatches,omitempty"`
	NamespaceConsistent   bool                `json:"namespaceConsistent"`
	PodSecurityConsistent bool                `json:"podSecurityConsistent"`
	Consistent            bool                `json:"consistent"`
	Issues                []string            `json:"issues,omitempty"`
}

// spaceAgg accumulates one variant Space's observations while analyzing.
type spaceAgg struct {
	space      string
	cluster    string
	namespaces map[string]bool
	psa        map[string]bool
	// workloads maps Kind/name to the namespaces a workload of that name runs in.
	workloads map[string]map[string]bool
}

// AnalyzeConsistency groups the fleet's namespaces by their Space's Component
// label and reports, per component, whether its workloads run in the same
// namespaces across clusters and its pod-security level is the same across
// variant Spaces. Resources whose Space has no Component label are skipped
// (they can't be grouped). Canonical Spaces are already excluded upstream.
func AnalyzeConsistency(clusters map[string]*ClusterNamespaces) []ComponentConsistency {
	// component -> space slug -> aggregate
	byComponent := map[string]map[string]*spaceAgg{}
	aggFor := func(component, space, cluster string) *spaceAgg {
		spaces, ok := byComponent[component]
		if !ok {
			spaces = map[string]*spaceAgg{}
			byComponent[component] = spaces
		}
		a, ok := spaces[space]
		if !ok {
			a = &spaceAgg{space: space, cluster: cluster, namespaces: map[string]bool{}, psa: map[string]bool{}, workloads: map[string]map[string]bool{}}
			spaces[space] = a
		}
		return a
	}

	componentOf := func(o ResourceOrigin) string { return o.SpaceLabels[ComponentLabel] }
	usable := func(ns string) bool { return ns != "" && ns != PlaceholderNamespace }

	for _, c := range clusters {
		for _, n := range c.Namespaces {
			comp := componentOf(n.Origin)
			if comp == "" {
				continue
			}
			a := aggFor(comp, n.Origin.Space, n.Origin.Cluster)
			if usable(n.Name) {
				a.namespaces[n.Name] = true
			}
			if lvl := n.PodSecurityEnforce(); lvl != "" {
				a.psa[lvl] = true
			}
		}
		for _, w := range c.Workloads {
			comp := componentOf(w.Origin)
			if comp == "" || !usable(w.Namespace) {
				continue
			}
			a := aggFor(comp, w.Origin.Space, w.Origin.Cluster)
			a.namespaces[w.Namespace] = true
			key := w.Kind + "/" + w.Name
			if a.workloads[key] == nil {
				a.workloads[key] = map[string]bool{}
			}
			a.workloads[key][w.Namespace] = true
		}
	}

	out := make([]ComponentConsistency, 0, len(byComponent))
	for comp, spaces := range byComponent {
		cc := ComponentConsistency{Component: comp}
		nsSet := map[string]bool{}
		psaSet := map[string]bool{}
		for _, a := range spaces {
			v := VariantNamespace{Space: a.space, Cluster: a.cluster, Namespaces: sortedKeys(a.namespaces)}
			for ns := range a.namespaces {
				nsSet[ns] = true
			}
			psa := sortedKeys(a.psa)
			if len(psa) == 1 {
				v.PodSecurityEnforce = psa[0]
			}
			for lvl := range a.psa {
				psaSet[lvl] = true
			}
			cc.Variants = append(cc.Variants, v)
		}
		sort.Slice(cc.Variants, func(i, j int) bool { return cc.Variants[i].Space < cc.Variants[j].Space })
		cc.Namespaces = sortedKeys(nsSet)
		cc.PodSecurityLevels = sortedKeys(psaSet)
		cc.NamespaceMismatches = namespaceMismatches(spaces)
		cc.NamespaceConsistent = len(cc.NamespaceMismatches) == 0
		cc.PodSecurityConsistent = len(cc.PodSecurityLevels) <= 1
		cc.Consistent = cc.NamespaceConsistent && cc.PodSecurityConsistent
		if !cc.NamespaceConsistent {
			cc.Issues = append(cc.Issues, "workloads run in different namespaces on different clusters: "+describeMismatches(cc.NamespaceMismatches))
		}
		if !cc.PodSecurityConsistent {
			cc.Issues = append(cc.Issues, "pod-security enforce level differs across variant Spaces: "+joinQuoted(cc.PodSecurityLevels))
		}
		out = append(out, cc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Component < out[j].Component })
	return out
}

// namespaceMismatches compares, workload by workload, the variants that are a
// component's only variant on their cluster. Variants sharing a cluster must
// use different namespaces from each other, so they set no expectation for
// the rest; two that do collide are duplicate-namespace's finding. Comparing
// workloads rather than whole sets of namespaces lets a variant run parts
// another does not.
func namespaceMismatches(spaces map[string]*spaceAgg) []NamespaceMismatch {
	perCluster := map[string]int{}
	for _, a := range spaces {
		if len(a.workloads) > 0 {
			perCluster[a.cluster]++
		}
	}
	// Kind/name -> variant Space -> namespaces
	byWorkload := map[string]map[string][]string{}
	for _, a := range spaces {
		if perCluster[a.cluster] != 1 {
			continue
		}
		for key, nss := range a.workloads {
			if byWorkload[key] == nil {
				byWorkload[key] = map[string][]string{}
			}
			byWorkload[key][a.space] = sortedKeys(nss)
		}
	}
	var out []NamespaceMismatch
	for key, bySpace := range byWorkload {
		distinct := map[string]bool{}
		for _, nss := range bySpace {
			distinct[strings.Join(nss, ",")] = true
		}
		if len(distinct) > 1 {
			out = append(out, NamespaceMismatch{Workload: key, Namespaces: bySpace})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Workload < out[j].Workload })
	return out
}

// describeMismatches names each mismatched workload with the namespaces each
// variant Space runs it in.
func describeMismatches(ms []NamespaceMismatch) string {
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		spaces := make([]string, 0, len(m.Namespaces))
		for s := range m.Namespaces {
			spaces = append(spaces, s)
		}
		sort.Strings(spaces)
		where := make([]string, 0, len(spaces))
		for _, s := range spaces {
			where = append(where, joinQuoted(m.Namespaces[s])+" in "+s)
		}
		parts = append(parts, m.Workload+" is in "+strings.Join(where, ", "))
	}
	return strings.Join(parts, "; ")
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func joinQuoted(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += "\"" + s + "\""
	}
	return out
}
