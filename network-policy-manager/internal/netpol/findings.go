// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import (
	"fmt"
	"net"
	"sort"
)

// Severity levels for findings.
const (
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// metadataIP is the cloud instance-metadata endpoint that egress policies should
// not expose (AWS/GCP/Azure link-local address).
const metadataIP = "169.254.169.254"

// Finding is one hygiene/anti-pattern result attributed to a resource (or a
// namespace, when no single resource owns it).
type Finding struct {
	Analyzer  string         `json:"analyzer"`
	Severity  string         `json:"severity"`
	Cluster   string         `json:"cluster"`
	Namespace string         `json:"namespace,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	Resource  string         `json:"resource,omitempty"`
	Message   string         `json:"message"`
	Origin    ResourceOrigin `json:"origin,omitempty"`
}

// AnalyzeFindings runs the v1 analyzer set over one cluster:
//   - missing-default-deny-ingress (namespace with workloads, no default-deny)
//   - uncovered-ingress (workload selected by no ingress policy)
//   - allow-all (a policy with an empty-peer ingress/egress rule)
//   - metadata-egress (egress rule whose ipBlock exposes the metadata IP)
//   - ingress-egress-asymmetry (one side allows the flow, the other drops it)
func AnalyzeFindings(c *ClusterNetpol) []Finding {
	idx := newClusterIndex(c)
	namespaces, workloads := c.Coverage()
	var fs []Finding

	for _, nc := range namespaces {
		if nc.Workloads > 0 && !nc.DefaultDenyIngress {
			fs = append(fs, Finding{
				Analyzer: "missing-default-deny-ingress", Severity: SeverityHigh,
				Cluster: c.Cluster, Namespace: nc.Namespace,
				Message: fmt.Sprintf("namespace %q has %d workload(s) but no default-deny ingress NetworkPolicy", nc.Namespace, nc.Workloads),
			})
		}
	}

	for _, wc := range workloads {
		if !wc.IngressCovered {
			fs = append(fs, Finding{
				Analyzer: "uncovered-ingress", Severity: SeverityHigh,
				Cluster: c.Cluster, Namespace: wc.Namespace, Kind: wc.Kind, Resource: wc.Name, Origin: wc.Origin,
				Message: fmt.Sprintf("%s %q in namespace %q is not selected by any ingress NetworkPolicy", wc.Kind, wc.Name, wc.Namespace),
			})
		}
	}

	for _, p := range c.NetworkPolicies {
		if hasAllowAllRule(p.Ingress) {
			fs = append(fs, Finding{
				Analyzer: "allow-all", Severity: SeverityMedium,
				Cluster: c.Cluster, Namespace: NamespaceOf(p.Namespace), Kind: "NetworkPolicy", Resource: p.Name, Origin: p.Origin,
				Message: fmt.Sprintf("NetworkPolicy %q has an allow-all ingress rule (empty `from`), permitting all sources", p.Name),
			})
		}
		if hasAllowAllRule(p.Egress) {
			fs = append(fs, Finding{
				Analyzer: "allow-all", Severity: SeverityMedium,
				Cluster: c.Cluster, Namespace: NamespaceOf(p.Namespace), Kind: "NetworkPolicy", Resource: p.Name, Origin: p.Origin,
				Message: fmt.Sprintf("NetworkPolicy %q has an allow-all egress rule (empty `to`), permitting all destinations", p.Name),
			})
		}
		for _, rule := range p.Egress {
			for _, peer := range rule.Peers {
				if peer.IPBlock != nil && IPBlockExposesMetadata(peer.IPBlock) {
					fs = append(fs, Finding{
						Analyzer: "metadata-egress", Severity: SeverityHigh,
						Cluster: c.Cluster, Namespace: NamespaceOf(p.Namespace), Kind: "NetworkPolicy", Resource: p.Name, Origin: p.Origin,
						Message: fmt.Sprintf("NetworkPolicy %q egress permits cloud metadata IP %s via CIDR %s (exclude it with an `except`)", p.Name, metadataIP, peer.IPBlock.CIDR),
					})
				}
			}
		}
	}

	fs = append(fs, idx.asymmetryFindings()...)

	sort.SliceStable(fs, func(i, j int) bool {
		if severityRank(fs[i].Severity) != severityRank(fs[j].Severity) {
			return severityRank(fs[i].Severity) < severityRank(fs[j].Severity)
		}
		if fs[i].Analyzer != fs[j].Analyzer {
			return fs[i].Analyzer < fs[j].Analyzer
		}
		if fs[i].Namespace != fs[j].Namespace {
			return fs[i].Namespace < fs[j].Namespace
		}
		return fs[i].Resource < fs[j].Resource
	})
	return fs
}

// asymmetryFindings reports flows that one side expresses intent for but the
// other side silently drops. Only pairs where an explicit allow exists on one
// side are considered, so the output is the actionable mismatch set, not every
// blocked pair.
func (idx *clusterIndex) asymmetryFindings() []Finding {
	var fs []Finding
	for _, a := range idx.workloads {
		for _, b := range idx.workloads {
			if a == b {
				continue
			}
			// A's egress explicitly allows reaching B, but B's ingress drops it.
			// "Specific" intent only — an allow-all rule is not a per-peer intent.
			if idx.egressAllowsSpecific(a, b) && idx.ingressIsolated(b) && !idx.ingressAllows(b, a) {
				fs = append(fs, Finding{
					Analyzer: "ingress-egress-asymmetry", Severity: SeverityMedium,
					Cluster: idx.cluster, Namespace: NamespaceOf(a.Namespace), Kind: a.Kind, Resource: a.Name, Origin: a.Origin,
					Message: fmt.Sprintf("%s %q egress allows reaching %s %q, but %q's ingress does not admit it — traffic is dropped at the destination",
						a.Kind, a.Name, b.Kind, b.Name, b.Name),
				})
			}
			// B's ingress explicitly allows A, but A's egress won't permit it.
			if idx.ingressAllowsSpecific(b, a) && idx.egressIsolated(a) && !idx.egressAllows(a, b) {
				fs = append(fs, Finding{
					Analyzer: "ingress-egress-asymmetry", Severity: SeverityMedium,
					Cluster: idx.cluster, Namespace: NamespaceOf(b.Namespace), Kind: b.Kind, Resource: b.Name, Origin: b.Origin,
					Message: fmt.Sprintf("%s %q ingress allows %s %q, but %q's egress does not permit it — traffic is dropped at the source",
						b.Kind, b.Name, a.Kind, a.Name, a.Name),
				})
			}
		}
	}
	return fs
}

// hasAllowAllRule reports whether any rule in the set has no peer constraints
// (an empty `from`/`to`), which admits all peers.
func hasAllowAllRule(rules []NetworkPolicyRule) bool {
	for _, r := range rules {
		if len(r.Peers) == 0 {
			return true
		}
	}
	return false
}

// IPBlockExposesMetadata reports whether an egress ipBlock admits the cloud
// metadata IP — its CIDR contains the address and no `except` excludes it.
func IPBlockExposesMetadata(ib *IPBlock) bool {
	_, cidr, err := net.ParseCIDR(ib.CIDR)
	if err != nil {
		return false
	}
	mip := net.ParseIP(metadataIP)
	if !cidr.Contains(mip) {
		return false
	}
	for _, ex := range ib.Except {
		if _, exNet, err := net.ParseCIDR(ex); err == nil && exNet.Contains(mip) {
			return false
		}
	}
	return true
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
