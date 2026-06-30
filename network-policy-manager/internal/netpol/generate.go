// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// managedByLabel marks the resources this tool authors, so they are
// distinguishable from hand-written policies.
const managedByLabel = "cub-netpol"

// DefaultDenyYAML renders a default-deny NetworkPolicy for a namespace. By
// default it denies all ingress (podSelector {} + policyTypes [Ingress]); with
// egress=true it also denies egress but allows DNS to kube-dns, since a bare
// egress default-deny breaks name resolution. The returned slug is a stable
// Unit slug derived from the namespace.
func DefaultDenyYAML(namespace string, egress bool) (slug, manifest string) {
	policyTypes := []any{"Ingress"}
	spec := map[string]any{"podSelector": map[string]any{}}
	if egress {
		policyTypes = []any{"Ingress", "Egress"}
		spec["egress"] = []any{dnsEgressRule()}
	}
	spec["policyTypes"] = policyTypes
	doc := networkPolicyDoc("default-deny", namespace, spec)
	return "default-deny-" + namespace, mustYAML(doc)
}

// AllowYAML renders a NetworkPolicy that admits traffic between two workloads.
// Default direction is ingress: a policy in dst's namespace selecting dst that
// admits src. With egress=true it is an egress policy in src's namespace
// selecting src that permits reaching dst. port, when non-empty, restricts the
// rule to that port (numeric or named); protocol defaults to TCP.
func AllowYAML(src, dst *WorkloadEntity, egress bool, port string) (slug, manifest string) {
	if egress {
		peer := workloadPeer(NamespaceOf(dst.Namespace), dst.PodLabels)
		rule := map[string]any{"to": []any{peer}}
		if port != "" {
			rule["ports"] = []any{portEntry(port)}
		}
		spec := map[string]any{
			"podSelector": matchLabels(src.PodLabels),
			"policyTypes": []any{"Egress"},
			"egress":      []any{rule},
		}
		name := "allow-" + src.Name + "-to-" + dst.Name + "-egress"
		return name, mustYAML(networkPolicyDoc(name, NamespaceOf(src.Namespace), spec))
	}
	peer := workloadPeer(NamespaceOf(src.Namespace), src.PodLabels)
	rule := map[string]any{"from": []any{peer}}
	if port != "" {
		rule["ports"] = []any{portEntry(port)}
	}
	spec := map[string]any{
		"podSelector": matchLabels(dst.PodLabels),
		"policyTypes": []any{"Ingress"},
		"ingress":     []any{rule},
	}
	name := "allow-" + src.Name + "-to-" + dst.Name
	return name, mustYAML(networkPolicyDoc(name, NamespaceOf(dst.Namespace), spec))
}

// AllowIngressYAML renders a single ingress NetworkPolicy for one destination
// workload that admits every source in sources — the idiomatic "one policy per
// protected workload, all sources as `from` peers" shape. port, when non-empty,
// restricts the rule to that port. Sources are de-duplicated and ordered for a
// stable manifest.
func AllowIngressYAML(dst *WorkloadEntity, sources []*WorkloadEntity, port string) (slug, manifest string) {
	peers := make([]any, 0, len(sources))
	for _, s := range sortWorkloads(sources) {
		peers = append(peers, workloadPeer(NamespaceOf(s.Namespace), s.PodLabels))
	}
	rule := map[string]any{"from": peers}
	if port != "" {
		rule["ports"] = []any{portEntry(port)}
	}
	spec := map[string]any{
		"podSelector": matchLabels(dst.PodLabels),
		"policyTypes": []any{"Ingress"},
		"ingress":     []any{rule},
	}
	name := "allow-" + dst.Name + "-ingress"
	return name, mustYAML(networkPolicyDoc(name, NamespaceOf(dst.Namespace), spec))
}

// sortWorkloads returns the workloads ordered by namespace then name, with
// duplicates (same namespace+name) removed.
func sortWorkloads(ws []*WorkloadEntity) []*WorkloadEntity {
	seen := map[string]bool{}
	out := make([]*WorkloadEntity, 0, len(ws))
	for _, w := range ws {
		key := NamespaceOf(w.Namespace) + "/" + w.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, nj := NamespaceOf(out[i].Namespace), NamespaceOf(out[j].Namespace)
		if ni != nj {
			return ni < nj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func networkPolicyDoc(name, namespace string, spec map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]any{
				"app.kubernetes.io/name":       name,
				"app.kubernetes.io/managed-by": managedByLabel,
			},
		},
		"spec": spec,
	}
}

// dnsEgressRule allows egress to the cluster DNS service (kube-dns in
// kube-system) on port 53 UDP and TCP — the allowance a default-deny egress
// must include to avoid breaking name resolution.
func dnsEgressRule() map[string]any {
	return map[string]any{
		"to": []any{
			map[string]any{
				"namespaceSelector": map[string]any{
					"matchLabels": map[string]any{namespaceNameLabel: "kube-system"},
				},
				"podSelector": map[string]any{
					"matchLabels": map[string]any{"k8s-app": "kube-dns"},
				},
			},
		},
		"ports": []any{
			map[string]any{"protocol": "UDP", "port": 53},
			map[string]any{"protocol": "TCP", "port": 53},
		},
	}
}

// workloadPeer builds a from/to peer that selects the given pod labels in the
// given namespace (namespaceSelector by the implicit name label + podSelector).
func workloadPeer(namespace string, podLabels map[string]string) map[string]any {
	return map[string]any{
		"namespaceSelector": map[string]any{
			"matchLabels": map[string]any{namespaceNameLabel: namespace},
		},
		"podSelector": matchLabels(podLabels),
	}
}

func matchLabels(labels map[string]string) map[string]any {
	m := make(map[string]any, len(labels))
	for k, v := range labels {
		m[k] = v
	}
	return map[string]any{"matchLabels": m}
}

// portEntry renders a port rule; numeric strings become integer ports, anything
// else is treated as a named port.
func portEntry(port string) map[string]any {
	entry := map[string]any{"protocol": "TCP"}
	if n, err := strconv.Atoi(port); err == nil {
		entry["port"] = n
	} else {
		entry["port"] = port
	}
	return entry
}

// FromPeerJSON renders an ingress from-peer (namespaceSelector + podSelector)
// for a source workload as compact JSON, for splicing into a set-yq expression
// when upserting a source into an existing allow policy.
func FromPeerJSON(src *WorkloadEntity) string {
	b, err := json.Marshal(workloadPeer(NamespaceOf(src.Namespace), src.PodLabels))
	if err != nil {
		panic(err) // fixed-shape map of strings
	}
	return string(b)
}

// CanonicalLabels renders a label map as a stable "k=v,k=v" string for set
// membership comparison (e.g. matching an existing from-peer to a source).
func CanonicalLabels(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ",")
}

func mustYAML(doc map[string]any) string {
	b, err := yaml.Marshal(doc)
	if err != nil {
		// The inputs are fixed-shape maps of strings/ints; marshal cannot fail.
		panic(err)
	}
	return string(b)
}
