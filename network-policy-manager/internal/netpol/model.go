// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package netpol is the Kubernetes NetworkPolicy analysis engine: it parses
// NetworkPolicies, Namespaces, pod-bearing workloads, and Services drawn from
// ConfigHub Units into a typed domain model. Later milestones add the coverage
// and connectivity analysis over that model; M1 provides the model and the
// fleet inventory built on it.
//
// Parsing is lenient: malformed documents are skipped, never errored on — a bad
// resource in one Unit must not take down fleet-wide analysis.
package netpol

import (
	"strconv"
	"strings"
)

// ResourceOrigin records where a resource came from in ConfigHub. Clusters are
// Targets: a Unit's Target identifies the cluster it deploys to, and Units from
// many Spaces can share one cluster Target. Cluster is the Target slug when the
// Unit is bound, falling back to the Space slug for unbound ("paper cluster")
// Units; Target is set only when actually bound.
type ResourceOrigin struct {
	Cluster      string `json:"cluster"`
	Target       string `json:"target,omitempty"`
	Space        string `json:"space"`
	SpaceID      string `json:"spaceId"`
	UnitID       string `json:"unitId"`
	UnitSlug     string `json:"unitSlug"`
	ResourceName string `json:"resourceName"`
	// Canonical is true for definitions in base/policy Spaces that aren't
	// deployed anywhere — shown in the explorer but excluded from cluster
	// analysis (coverage, connectivity, findings).
	Canonical bool `json:"canonical,omitempty"`
}

// FleetResource is a parsed resource document plus its ConfigHub origin. Doc is
// the decoded JSON body (typically a map[string]any).
type FleetResource struct {
	Origin ResourceOrigin
	Doc    any
}

// LabelSelector is a parsed Kubernetes label selector (matchLabels +
// matchExpressions). Present distinguishes an explicitly-set selector — even the
// empty selector `{}`, which matches every pod in scope — from an absent one.
type LabelSelector struct {
	Present          bool                       `json:"present"`
	MatchLabels      map[string]string          `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement is one matchExpressions clause.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// Empty reports whether the selector is present but constrains nothing (`{}`),
// i.e. it selects all pods in its scope.
func (s LabelSelector) Empty() bool {
	return s.Present && len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0
}

// NetworkPolicyEntity is a parsed networking.k8s.io/v1 NetworkPolicy, including
// its ingress/egress rules and the policy-type isolation it implies.
type NetworkPolicyEntity struct {
	Name        string              `json:"name"`
	Namespace   string              `json:"namespace"`
	PodSelector LabelSelector       `json:"podSelector"`
	PolicyTypes []string            `json:"policyTypes,omitempty"`
	Ingress     []NetworkPolicyRule `json:"ingress,omitempty"`
	Egress      []NetworkPolicyRule `json:"egress,omitempty"`
	Origin      ResourceOrigin      `json:"origin"`
}

// NetworkPolicyRule is one ingress or egress rule. Peers are the rule's `from`
// (ingress) or `to` (egress) entries; an empty Peers list means "all peers"
// (an allow-all rule). Ports empty means "all ports".
type NetworkPolicyRule struct {
	Peers []NetworkPolicyPeer `json:"peers,omitempty"`
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// NetworkPolicyPeer is one entry in a rule's from/to list. Exactly one of
// PodSelector / NamespaceSelector / IPBlock is typically set (podSelector and
// namespaceSelector may be combined to mean "pods matching X in namespaces
// matching Y"). A nil selector means that dimension is unconstrained per the
// Kubernetes peer semantics.
type NetworkPolicyPeer struct {
	PodSelector       *LabelSelector `json:"podSelector,omitempty"`
	NamespaceSelector *LabelSelector `json:"namespaceSelector,omitempty"`
	IPBlock           *IPBlock       `json:"ipBlock,omitempty"`
}

// IPBlock is a CIDR peer with optional exceptions.
type IPBlock struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except,omitempty"`
}

// NetworkPolicyPort is one port entry. Port is the numeric or named port as a
// string ("" = all ports); EndPort, when non-zero, makes [Port, EndPort] a range.
type NetworkPolicyPort struct {
	Protocol string `json:"protocol"`
	Port     string `json:"port,omitempty"`
	EndPort  int    `json:"endPort,omitempty"`
}

// Isolates reports which directions this policy isolates, applying the
// Kubernetes default: when policyTypes is set it is authoritative; when absent,
// Ingress is always implied and Egress is implied only if egress rules exist.
func (np *NetworkPolicyEntity) Isolates() (ingress, egress bool) {
	if len(np.PolicyTypes) > 0 {
		for _, t := range np.PolicyTypes {
			switch t {
			case "Ingress":
				ingress = true
			case "Egress":
				egress = true
			}
		}
		return ingress, egress
	}
	return true, len(np.Egress) > 0
}

// IsolatesIngress reports whether this policy makes its selected pods
// ingress-isolated (only explicitly-allowed ingress permitted).
func (np *NetworkPolicyEntity) IsolatesIngress() bool {
	ingress, _ := np.Isolates()
	return ingress
}

// IsolatesEgress reports whether this policy makes its selected pods
// egress-isolated.
func (np *NetworkPolicyEntity) IsolatesEgress() bool {
	_, egress := np.Isolates()
	return egress
}

// AllowsDNSEgress reports whether any egress rule already permits DNS (port 53).
func (np *NetworkPolicyEntity) AllowsDNSEgress() bool {
	for _, rule := range np.Egress {
		for _, p := range rule.Ports {
			if p.Port == "53" {
				return true
			}
		}
	}
	return false
}

// ExposesMetadataEgress reports whether any egress ipBlock admits the cloud
// metadata IP without excepting it.
func (np *NetworkPolicyEntity) ExposesMetadataEgress() bool {
	for _, rule := range np.Egress {
		for _, peer := range rule.Peers {
			if peer.IPBlock != nil && IPBlockExposesMetadata(peer.IPBlock) {
				return true
			}
		}
	}
	return false
}

// WorkloadEntity is a parsed pod-bearing resource (Deployment, StatefulSet,
// DaemonSet, ReplicaSet, Job, CronJob, or bare Pod). PodLabels holds the labels
// on the pods it manages — the labels a NetworkPolicy podSelector matches.
type WorkloadEntity struct {
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	PodLabels map[string]string `json:"podLabels,omitempty"`
	Origin    ResourceOrigin    `json:"origin"`
}

// NamespaceEntity is a parsed v1 Namespace.
type NamespaceEntity struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Origin ResourceOrigin    `json:"origin"`
}

// ServiceEntity is a parsed v1 Service. Selector is the equality-based pod
// selector (a plain string map, unlike a NetworkPolicy's LabelSelector).
type ServiceEntity struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Selector  map[string]string `json:"selector,omitempty"`
	Origin    ResourceOrigin    `json:"origin"`
}

// ClusterNetpol holds the NetworkPolicy-relevant entities of one cluster.
type ClusterNetpol struct {
	Cluster         string                 `json:"cluster"`
	NetworkPolicies []*NetworkPolicyEntity `json:"networkPolicies"`
	Workloads       []*WorkloadEntity      `json:"workloads"`
	Namespaces      []*NamespaceEntity     `json:"namespaces"`
	Services        []*ServiceEntity       `json:"services"`
}

// workloadKinds is the set of pod-bearing kinds whose pod-template labels a
// NetworkPolicy can select.
var workloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"ReplicaSet":  true,
	"Job":         true,
	"CronJob":     true,
	"Pod":         true,
}

// BuildFleet indexes parsed fleet resources into per-cluster entity sets.
// Unrecognized kinds and unparseable docs are ignored. Entities within a cluster
// preserve input order.
func BuildFleet(resources []FleetResource) map[string]*ClusterNetpol {
	clusters := make(map[string]*ClusterNetpol)
	forCluster := func(name string) *ClusterNetpol {
		c, ok := clusters[name]
		if !ok {
			c = &ClusterNetpol{Cluster: name}
			clusters[name] = c
		}
		return c
	}

	for _, fr := range resources {
		rec, ok := asRecord(fr.Doc)
		if !ok {
			continue
		}
		kind, hasKind := asString(rec["kind"])
		apiVersion, _ := asString(rec["apiVersion"])
		metadata, _ := asRecord(rec["metadata"])
		name, hasName := asString(metadata["name"])
		if !hasKind || !hasName {
			continue
		}
		namespace, _ := asString(metadata["namespace"])
		labels := asStringMap(metadata["labels"])
		cluster := forCluster(fr.Origin.Cluster)
		spec, _ := asRecord(rec["spec"])

		switch {
		case kind == "NetworkPolicy" && strings.HasPrefix(apiVersion, "networking.k8s.io/"):
			cluster.NetworkPolicies = append(cluster.NetworkPolicies, &NetworkPolicyEntity{
				Name:        name,
				Namespace:   namespace,
				PodSelector: parseLabelSelector(spec["podSelector"]),
				PolicyTypes: asStringArray(spec["policyTypes"]),
				Ingress:     parseNetpolRules(spec["ingress"], "from"),
				Egress:      parseNetpolRules(spec["egress"], "to"),
				Origin:      fr.Origin,
			})
		case kind == "Namespace" && apiVersion == "v1":
			cluster.Namespaces = append(cluster.Namespaces, &NamespaceEntity{
				Name: name, Labels: labels, Origin: fr.Origin,
			})
		case kind == "Service" && apiVersion == "v1":
			cluster.Services = append(cluster.Services, &ServiceEntity{
				Name: name, Namespace: namespace,
				Selector: asStringMap(spec["selector"]), Origin: fr.Origin,
			})
		case workloadKinds[kind]:
			cluster.Workloads = append(cluster.Workloads, &WorkloadEntity{
				Kind: kind, Name: name, Namespace: namespace,
				PodLabels: podTemplateLabels(kind, rec), Origin: fr.Origin,
			})
		}
	}
	return clusters
}

// podTemplateLabels extracts the labels on the pods a workload manages, from the
// pod-template path appropriate to its kind.
func podTemplateLabels(kind string, rec map[string]any) map[string]string {
	switch kind {
	case "Pod":
		md := nestedRecord(rec, "metadata")
		return asStringMap(md["labels"])
	case "CronJob":
		md := nestedRecord(rec, "spec", "jobTemplate", "spec", "template", "metadata")
		return asStringMap(md["labels"])
	default: // Deployment, StatefulSet, DaemonSet, ReplicaSet, Job
		md := nestedRecord(rec, "spec", "template", "metadata")
		return asStringMap(md["labels"])
	}
}

// ResourceMeta extracts the kind, name, and namespace from a decoded resource
// document. ok is false when the doc is not an object or lacks a kind/name.
func ResourceMeta(doc any) (kind, name, namespace string, ok bool) {
	rec, isRec := asRecord(doc)
	if !isRec {
		return "", "", "", false
	}
	kind, _ = asString(rec["kind"])
	md, _ := asRecord(rec["metadata"])
	name, _ = asString(md["name"])
	namespace, _ = asString(md["namespace"])
	return kind, name, namespace, kind != "" && name != ""
}

// --- lenient decoding helpers (shared with later analysis files) ---

func asRecord(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asStringArray(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func asStringMap(v any) map[string]string {
	rec, ok := asRecord(v)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(rec))
	for k, val := range rec {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

// nestedRecord walks a chain of map keys, returning the nested object or a nil
// map if any segment is missing or not an object. Reading from the returned nil
// map is safe and yields zero values.
func nestedRecord(rec map[string]any, keys ...string) map[string]any {
	cur := rec
	for _, k := range keys {
		next, ok := asRecord(cur[k])
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func firstString(v any) string {
	s, _ := asString(v)
	return s
}

// scalarString renders a string or number as a string. Port values arrive as a
// string (named port) or a number — float64 from JSON (get-resources) or int
// from YAML — so all numeric kinds are handled.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	}
	return ""
}

func intOf(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	}
	return 0
}

// parseNetpolRules parses a spec.ingress or spec.egress array. peerKey is "from"
// for ingress, "to" for egress. A null or non-object entry still counts as a
// rule with no constraints (allow-all).
func parseNetpolRules(v any, peerKey string) []NetworkPolicyRule {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	rules := make([]NetworkPolicyRule, 0, len(arr))
	for _, item := range arr {
		rec, ok := asRecord(item)
		if !ok {
			rules = append(rules, NetworkPolicyRule{})
			continue
		}
		rules = append(rules, NetworkPolicyRule{
			Peers: parsePeers(rec[peerKey]),
			Ports: parsePorts(rec["ports"]),
		})
	}
	return rules
}

func parsePeers(v any) []NetworkPolicyPeer {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	peers := make([]NetworkPolicyPeer, 0, len(arr))
	for _, item := range arr {
		rec, ok := asRecord(item)
		if !ok {
			continue
		}
		var peer NetworkPolicyPeer
		if ps, ok := rec["podSelector"]; ok {
			sel := parseLabelSelector(ps)
			peer.PodSelector = &sel
		}
		if nsSel, ok := rec["namespaceSelector"]; ok {
			sel := parseLabelSelector(nsSel)
			peer.NamespaceSelector = &sel
		}
		if ib, ok := asRecord(rec["ipBlock"]); ok {
			peer.IPBlock = &IPBlock{CIDR: firstString(ib["cidr"]), Except: asStringArray(ib["except"])}
		}
		peers = append(peers, peer)
	}
	return peers
}

func parsePorts(v any) []NetworkPolicyPort {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	ports := make([]NetworkPolicyPort, 0, len(arr))
	for _, item := range arr {
		rec, ok := asRecord(item)
		if !ok {
			continue
		}
		proto := firstString(rec["protocol"])
		if proto == "" {
			proto = "TCP"
		}
		ports = append(ports, NetworkPolicyPort{
			Protocol: proto,
			Port:     scalarString(rec["port"]),
			EndPort:  intOf(rec["endPort"]),
		})
	}
	return ports
}

func parseLabelSelector(v any) LabelSelector {
	rec, ok := asRecord(v)
	if !ok {
		return LabelSelector{}
	}
	sel := LabelSelector{Present: true, MatchLabels: asStringMap(rec["matchLabels"])}
	if exprs, ok := rec["matchExpressions"].([]any); ok {
		for _, e := range exprs {
			er, ok := asRecord(e)
			if !ok {
				continue
			}
			key, _ := asString(er["key"])
			op, _ := asString(er["operator"])
			sel.MatchExpressions = append(sel.MatchExpressions, LabelSelectorRequirement{
				Key: key, Operator: op, Values: asStringArray(er["values"]),
			})
		}
	}
	return sel
}
