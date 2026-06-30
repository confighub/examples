// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import "sort"

// NamespaceOf normalizes an empty namespace to "default", matching the
// Kubernetes default for namespaced resources whose namespace is unset.
func NamespaceOf(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

// WorkloadCoverage records whether a single workload is selected by any ingress
// and any egress NetworkPolicy in its namespace — i.e. whether it is isolated
// (and therefore governed) on each direction.
type WorkloadCoverage struct {
	Cluster         string         `json:"cluster"`
	Namespace       string         `json:"namespace"`
	Kind            string         `json:"kind"`
	Name            string         `json:"name"`
	IngressCovered  bool           `json:"ingressCovered"`
	EgressCovered   bool           `json:"egressCovered"`
	IngressPolicies []string       `json:"ingressPolicies,omitempty"`
	EgressPolicies  []string       `json:"egressPolicies,omitempty"`
	Origin          ResourceOrigin `json:"origin"`
}

// NamespaceCoverage summarizes NetworkPolicy coverage for one namespace.
type NamespaceCoverage struct {
	Cluster            string   `json:"cluster"`
	Namespace          string   `json:"namespace"`
	HasPolicy          bool     `json:"hasPolicy"`
	DefaultDenyIngress bool     `json:"defaultDenyIngress"`
	DefaultDenyEgress  bool     `json:"defaultDenyEgress"`
	Workloads          int      `json:"workloads"`
	IngressCovered     int      `json:"ingressCovered"`
	EgressCovered      int      `json:"egressCovered"`
	UncoveredIngress   []string `json:"uncoveredIngress,omitempty"`
	UncoveredEgress    []string `json:"uncoveredEgress,omitempty"`
}

// Coverage computes per-namespace and per-workload NetworkPolicy coverage for
// the cluster. A workload is "covered" on a direction if at least one policy in
// its namespace isolates that direction and its podSelector matches the
// workload's pod labels. A namespace has a "default-deny" on a direction if some
// policy there isolates that direction with an empty podSelector (selecting all
// pods). Namespaces are derived from both policies and workloads, so a namespace
// with workloads but no policy still appears (as fully uncovered).
func (c *ClusterNetpol) Coverage() (namespaces []NamespaceCoverage, workloads []WorkloadCoverage) {
	policiesByNS := map[string][]*NetworkPolicyEntity{}
	for _, p := range c.NetworkPolicies {
		ns := NamespaceOf(p.Namespace)
		policiesByNS[ns] = append(policiesByNS[ns], p)
	}

	nsSet := map[string]bool{}
	for ns := range policiesByNS {
		nsSet[ns] = true
	}

	workloadsByNS := map[string][]WorkloadCoverage{}
	for _, w := range c.Workloads {
		ns := NamespaceOf(w.Namespace)
		nsSet[ns] = true
		wc := WorkloadCoverage{
			Cluster: c.Cluster, Namespace: ns, Kind: w.Kind, Name: w.Name, Origin: w.Origin,
		}
		for _, p := range policiesByNS[ns] {
			if !p.PodSelector.Matches(w.PodLabels) {
				continue
			}
			if p.IsolatesIngress() {
				wc.IngressCovered = true
				wc.IngressPolicies = append(wc.IngressPolicies, p.Name)
			}
			if p.IsolatesEgress() {
				wc.EgressCovered = true
				wc.EgressPolicies = append(wc.EgressPolicies, p.Name)
			}
		}
		workloads = append(workloads, wc)
		workloadsByNS[ns] = append(workloadsByNS[ns], wc)
	}

	for ns := range nsSet {
		nc := NamespaceCoverage{Cluster: c.Cluster, Namespace: ns}
		pols := policiesByNS[ns]
		nc.HasPolicy = len(pols) > 0
		for _, p := range pols {
			if !p.PodSelector.Empty() {
				continue
			}
			if p.IsolatesIngress() {
				nc.DefaultDenyIngress = true
			}
			if p.IsolatesEgress() {
				nc.DefaultDenyEgress = true
			}
		}
		for _, wc := range workloadsByNS[ns] {
			nc.Workloads++
			if wc.IngressCovered {
				nc.IngressCovered++
			} else {
				nc.UncoveredIngress = append(nc.UncoveredIngress, wc.Name)
			}
			if wc.EgressCovered {
				nc.EgressCovered++
			} else {
				nc.UncoveredEgress = append(nc.UncoveredEgress, wc.Name)
			}
		}
		namespaces = append(namespaces, nc)
	}

	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].Namespace < namespaces[j].Namespace })
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].Namespace != workloads[j].Namespace {
			return workloads[i].Namespace < workloads[j].Namespace
		}
		return workloads[i].Name < workloads[j].Name
	})
	return namespaces, workloads
}
