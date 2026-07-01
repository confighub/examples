// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import "sort"

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

// ComponentConsistency is the cross-variant verdict for one component: whether
// its namespace name and pod-security level are identical across every variant
// Space. This is the fleet-wide read prior art (per-cluster controllers,
// per-resource validators) cannot do.
type ComponentConsistency struct {
	Component             string             `json:"component"`
	Variants              []VariantNamespace `json:"variants"`
	Namespaces            []string           `json:"namespaces"`
	PodSecurityLevels     []string           `json:"podSecurityLevels,omitempty"`
	NamespaceConsistent   bool               `json:"namespaceConsistent"`
	PodSecurityConsistent bool               `json:"podSecurityConsistent"`
	Consistent            bool               `json:"consistent"`
	Issues                []string           `json:"issues,omitempty"`
}

// spaceAgg accumulates one variant Space's observations while analyzing.
type spaceAgg struct {
	space      string
	cluster    string
	namespaces map[string]bool
	psa        map[string]bool
}

// AnalyzeConsistency groups the fleet's namespaces by their Space's Component
// label and reports, per component, whether the namespace name and pod-security
// level are identical across its variant Spaces. Resources whose Space has no
// Component label are skipped (they can't be grouped). Canonical Spaces are
// already excluded upstream.
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
			a = &spaceAgg{space: space, cluster: cluster, namespaces: map[string]bool{}, psa: map[string]bool{}}
			spaces[space] = a
		}
		return a
	}

	componentOf := func(o ResourceOrigin) string { return o.SpaceLabels[ComponentLabel] }
	addNamespace := func(a *spaceAgg, ns string) {
		if ns != "" && ns != PlaceholderNamespace {
			a.namespaces[ns] = true
		}
	}

	for _, c := range clusters {
		for _, n := range c.Namespaces {
			comp := componentOf(n.Origin)
			if comp == "" {
				continue
			}
			a := aggFor(comp, n.Origin.Space, n.Origin.Cluster)
			addNamespace(a, n.Name)
			if lvl := n.PodSecurityEnforce(); lvl != "" {
				a.psa[lvl] = true
			}
		}
		for _, w := range c.Workloads {
			if comp := componentOf(w.Origin); comp != "" {
				addNamespace(aggFor(comp, w.Origin.Space, w.Origin.Cluster), w.Namespace)
			}
		}
		for _, r := range c.RBAC {
			if comp := componentOf(r.Origin); comp != "" {
				addNamespace(aggFor(comp, r.Origin.Space, r.Origin.Cluster), r.Namespace)
			}
		}
		for _, np := range c.NetworkPolicies {
			if comp := componentOf(np.Origin); comp != "" {
				addNamespace(aggFor(comp, np.Origin.Space, np.Origin.Cluster), np.Namespace)
			}
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
		cc.NamespaceConsistent = len(cc.Namespaces) <= 1
		cc.PodSecurityConsistent = len(cc.PodSecurityLevels) <= 1
		cc.Consistent = cc.NamespaceConsistent && cc.PodSecurityConsistent
		if !cc.NamespaceConsistent {
			cc.Issues = append(cc.Issues, "namespace name differs across variant Spaces: "+joinQuoted(cc.Namespaces))
		}
		if !cc.PodSecurityConsistent {
			cc.Issues = append(cc.Issues, "pod-security enforce level differs across variant Spaces: "+joinQuoted(cc.PodSecurityLevels))
		}
		out = append(out, cc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Component < out[j].Component })
	return out
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
