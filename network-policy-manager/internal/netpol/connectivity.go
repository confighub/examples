// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

// connectivity models native NetworkPolicy reachability within one cluster.
//
// Semantics implemented:
//   - A pod is ingress/egress isolated once any policy with the corresponding
//     policyType selects it; otherwise that direction is open.
//   - Traffic A→B is allowed iff A's egress permits B (A not egress-isolated, or
//     an egress rule's `to` matches B) AND B's ingress permits A.
//   - Peer matching honors podSelector + namespaceSelector (with the implicit
//     kubernetes.io/metadata.name namespace label); a peer with no
//     namespaceSelector is scoped to the policy's own namespace.
//
// Limitations (v1): ipBlock peers never match pod-to-pod traffic (they describe
// external CIDRs); port constraints are not considered in the reachability
// boolean (a flow allowed on any port counts as reachable).

const namespaceNameLabel = "kubernetes.io/metadata.name"

// clusterIndex is the per-cluster lookup structure the reachability queries and
// the asymmetry analyzer share.
type clusterIndex struct {
	cluster      string
	policiesByNS map[string][]*NetworkPolicyEntity
	workloads    []*WorkloadEntity
	nsLabels     map[string]map[string]string
}

func newClusterIndex(c *ClusterNetpol) *clusterIndex {
	idx := &clusterIndex{
		cluster:      c.Cluster,
		policiesByNS: map[string][]*NetworkPolicyEntity{},
		workloads:    c.Workloads,
		nsLabels:     map[string]map[string]string{},
	}
	for _, p := range c.NetworkPolicies {
		ns := NamespaceOf(p.Namespace)
		idx.policiesByNS[ns] = append(idx.policiesByNS[ns], p)
	}
	for _, n := range c.Namespaces {
		labels := map[string]string{}
		for k, v := range n.Labels {
			labels[k] = v
		}
		labels[namespaceNameLabel] = n.Name
		idx.nsLabels[n.Name] = labels
	}
	// Ensure every namespace referenced by a workload or policy has at least the
	// implicit name label, so namespaceSelector matching works without an
	// explicit Namespace resource.
	ensure := func(ns string) {
		if _, ok := idx.nsLabels[ns]; !ok {
			idx.nsLabels[ns] = map[string]string{namespaceNameLabel: ns}
		}
	}
	for _, w := range c.Workloads {
		ensure(NamespaceOf(w.Namespace))
	}
	for ns := range idx.policiesByNS {
		ensure(ns)
	}
	return idx
}

func (idx *clusterIndex) ingressIsolated(w *WorkloadEntity) bool {
	for _, p := range idx.policiesByNS[NamespaceOf(w.Namespace)] {
		if p.IsolatesIngress() && p.PodSelector.Matches(w.PodLabels) {
			return true
		}
	}
	return false
}

func (idx *clusterIndex) egressIsolated(w *WorkloadEntity) bool {
	for _, p := range idx.policiesByNS[NamespaceOf(w.Namespace)] {
		if p.IsolatesEgress() && p.PodSelector.Matches(w.PodLabels) {
			return true
		}
	}
	return false
}

// ingressAllows reports whether some ingress rule on a policy selecting dst
// admits src.
func (idx *clusterIndex) ingressAllows(dst, src *WorkloadEntity) bool {
	for _, p := range idx.policiesByNS[NamespaceOf(dst.Namespace)] {
		if !p.IsolatesIngress() || !p.PodSelector.Matches(dst.PodLabels) {
			continue
		}
		for _, rule := range p.Ingress {
			if idx.ruleAllows(rule, src, NamespaceOf(p.Namespace)) {
				return true
			}
		}
	}
	return false
}

// egressAllows reports whether some egress rule on a policy selecting src admits
// dst.
func (idx *clusterIndex) egressAllows(src, dst *WorkloadEntity) bool {
	for _, p := range idx.policiesByNS[NamespaceOf(src.Namespace)] {
		if !p.IsolatesEgress() || !p.PodSelector.Matches(src.PodLabels) {
			continue
		}
		for _, rule := range p.Egress {
			if idx.ruleAllows(rule, dst, NamespaceOf(p.Namespace)) {
				return true
			}
		}
	}
	return false
}

// ingressAllowsSpecific is like ingressAllows but ignores allow-all (empty-peer)
// rules, so it reflects only an explicit intent to admit src — not a blanket
// open rule. The asymmetry analyzer uses this so an allow-all destination isn't
// read as expressing intent toward every source.
func (idx *clusterIndex) ingressAllowsSpecific(dst, src *WorkloadEntity) bool {
	for _, p := range idx.policiesByNS[NamespaceOf(dst.Namespace)] {
		if !p.IsolatesIngress() || !p.PodSelector.Matches(dst.PodLabels) {
			continue
		}
		for _, rule := range p.Ingress {
			if len(rule.Peers) == 0 {
				continue
			}
			if idx.ruleAllows(rule, src, NamespaceOf(p.Namespace)) {
				return true
			}
		}
	}
	return false
}

// egressAllowsSpecific is the egress counterpart of ingressAllowsSpecific.
func (idx *clusterIndex) egressAllowsSpecific(src, dst *WorkloadEntity) bool {
	for _, p := range idx.policiesByNS[NamespaceOf(src.Namespace)] {
		if !p.IsolatesEgress() || !p.PodSelector.Matches(src.PodLabels) {
			continue
		}
		for _, rule := range p.Egress {
			if len(rule.Peers) == 0 {
				continue
			}
			if idx.ruleAllows(rule, dst, NamespaceOf(p.Namespace)) {
				return true
			}
		}
	}
	return false
}

// ruleAllows reports whether a rule admits the target workload. An empty peer
// list means allow-all.
func (idx *clusterIndex) ruleAllows(rule NetworkPolicyRule, target *WorkloadEntity, policyNS string) bool {
	if len(rule.Peers) == 0 {
		return true
	}
	for _, peer := range rule.Peers {
		if idx.peerMatches(peer, target, policyNS) {
			return true
		}
	}
	return false
}

func (idx *clusterIndex) peerMatches(peer NetworkPolicyPeer, target *WorkloadEntity, policyNS string) bool {
	// An ipBlock-only peer describes external CIDRs and never matches a pod.
	if peer.PodSelector == nil && peer.NamespaceSelector == nil && peer.IPBlock != nil {
		return false
	}
	targetNS := NamespaceOf(target.Namespace)
	if peer.NamespaceSelector != nil {
		if !peer.NamespaceSelector.Matches(idx.nsLabels[targetNS]) {
			return false
		}
	} else if targetNS != policyNS {
		// No namespaceSelector: the peer is scoped to the policy's namespace.
		return false
	}
	if peer.PodSelector != nil {
		return peer.PodSelector.Matches(target.PodLabels)
	}
	return true
}

// CanReach reports whether src is allowed to send traffic to dst under the
// cluster's policies.
func (idx *clusterIndex) CanReach(src, dst *WorkloadEntity) bool {
	egressOK := !idx.egressIsolated(src) || idx.egressAllows(src, dst)
	ingressOK := !idx.ingressIsolated(dst) || idx.ingressAllows(dst, src)
	return egressOK && ingressOK
}

// WhoCanReach returns the workloads in the cluster allowed to send traffic to
// dst (dst must be one of the cluster's workloads).
func WhoCanReach(c *ClusterNetpol, dst *WorkloadEntity) []*WorkloadEntity {
	idx := newClusterIndex(c)
	var out []*WorkloadEntity
	for _, src := range idx.workloads {
		if src == dst {
			continue
		}
		if idx.CanReach(src, dst) {
			out = append(out, src)
		}
	}
	return out
}

// ReachableFrom returns the workloads in the cluster that src is allowed to send
// traffic to (src must be one of the cluster's workloads).
func ReachableFrom(c *ClusterNetpol, src *WorkloadEntity) []*WorkloadEntity {
	idx := newClusterIndex(c)
	var out []*WorkloadEntity
	for _, dst := range idx.workloads {
		if dst == src {
			continue
		}
		if idx.CanReach(src, dst) {
			out = append(out, dst)
		}
	}
	return out
}
