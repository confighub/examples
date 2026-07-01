// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package nsmanager is the namespace-envelope analysis engine: it parses
// Namespaces, default-deny NetworkPolicies, baseline RBAC (ServiceAccount /
// Role / RoleBinding), and pod-bearing workloads drawn from ConfigHub Units into
// a typed domain model, then computes per-namespace envelope completeness and
// fleet-wide consistency checks over it.
//
// "Envelope" is the policy bundle a namespace should carry: pod-security labels
// on the Namespace object, a default-deny NetworkPolicy, and baseline RBAC. The
// manager reports which namespaces are missing members — the fleet-wide read a
// per-resource validator or a runtime tenancy controller (Capsule, HNC) cannot
// do.
//
// Parsing is lenient: malformed documents are skipped, never errored on — a bad
// resource in one Unit must not take down fleet-wide analysis.
package nsmanager

import "strings"

// ResourceOrigin records where a resource came from in ConfigHub. Clusters are
// Targets: a Unit's Target identifies the cluster it deploys to, and Units from
// many Spaces can share one cluster Target. Cluster is the Target slug when the
// Unit is bound, falling back to the Space slug for unbound ("paper cluster")
// Units; Target is set only when actually bound.
type ResourceOrigin struct {
	Cluster      string            `json:"cluster"`
	Target       string            `json:"target,omitempty"`
	Space        string            `json:"space"`
	SpaceID      string            `json:"spaceId"`
	SpaceLabels  map[string]string `json:"spaceLabels,omitempty"`
	UnitID       string            `json:"unitId"`
	UnitSlug     string            `json:"unitSlug"`
	ResourceName string            `json:"resourceName"`
	// Canonical is true for definitions in base/policy Spaces that aren't
	// deployed anywhere — shown in the explorer but excluded from cluster
	// analysis (envelope completeness, duplicates).
	Canonical bool `json:"canonical,omitempty"`
}

// FleetResource is a parsed resource document plus its ConfigHub origin. Doc is
// the decoded JSON body (typically a map[string]any).
type FleetResource struct {
	Origin ResourceOrigin
	Doc    any
}

// NamespaceEntity is a parsed v1 Namespace. Its labels carry the pod-security
// admission level (pod-security.kubernetes.io/enforce).
type NamespaceEntity struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Origin ResourceOrigin    `json:"origin"`
}

// PodSecurityEnforceLabel is the Pod Security Admission enforce-level label.
const PodSecurityEnforceLabel = "pod-security.kubernetes.io/enforce"

// PodSecurityEnforce returns the namespace's enforced Pod Security level
// ("baseline", "restricted", "privileged"), or "" if none is set.
func (n *NamespaceEntity) PodSecurityEnforce() string {
	return n.Labels[PodSecurityEnforceLabel]
}

// NetworkPolicyEntity is a parsed networking.k8s.io/v1 NetworkPolicy, reduced to
// what envelope analysis needs: whether it is a namespace-wide default-deny.
type NetworkPolicyEntity struct {
	Name             string         `json:"name"`
	Namespace        string         `json:"namespace"`
	PodSelectorEmpty bool           `json:"podSelectorEmpty"`
	PolicyTypes      []string       `json:"policyTypes,omitempty"`
	HasEgressRules   bool           `json:"hasEgressRules,omitempty"`
	Origin           ResourceOrigin `json:"origin"`
}

// isolates reports which directions this policy isolates, applying the
// Kubernetes default: when policyTypes is set it is authoritative; when absent,
// Ingress is always implied and Egress only if egress rules exist.
func (np *NetworkPolicyEntity) isolates() (ingress, egress bool) {
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
	return true, np.HasEgressRules
}

// IsDefaultDenyIngress reports whether this policy is a namespace-wide
// default-deny on ingress: an empty podSelector (selects every pod) that
// isolates ingress. This is the envelope's default-deny member.
func (np *NetworkPolicyEntity) IsDefaultDenyIngress() bool {
	ingress, _ := np.isolates()
	return np.PodSelectorEmpty && ingress
}

// RBACEntity is a parsed namespaced RBAC object (ServiceAccount, Role, or
// RoleBinding).
type RBACEntity struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Origin    ResourceOrigin `json:"origin"`
}

// WorkloadEntity is a parsed pod-bearing resource (Deployment, StatefulSet,
// DaemonSet, ReplicaSet, Job, CronJob, or bare Pod) — used to identify which
// namespaces are occupied and therefore want an envelope.
type WorkloadEntity struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Origin    ResourceOrigin `json:"origin"`
}

// ClusterNamespaces holds the envelope-relevant entities of one cluster.
type ClusterNamespaces struct {
	Cluster         string                 `json:"cluster"`
	Namespaces      []*NamespaceEntity     `json:"namespaces"`
	NetworkPolicies []*NetworkPolicyEntity `json:"networkPolicies"`
	RBAC            []*RBACEntity          `json:"rbac"`
	Workloads       []*WorkloadEntity      `json:"workloads"`
}

// workloadKinds is the set of pod-bearing kinds whose presence marks a namespace
// as occupied.
var workloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"ReplicaSet":  true,
	"Job":         true,
	"CronJob":     true,
	"Pod":         true,
}

// rbacKinds is the set of namespaced RBAC kinds that make up baseline RBAC.
var rbacKinds = map[string]bool{
	"ServiceAccount": true,
	"Role":           true,
	"RoleBinding":    true,
}

// BuildFleet indexes parsed fleet resources into per-cluster entity sets.
// Unrecognized kinds and unparseable docs are ignored. Entities within a cluster
// preserve input order.
func BuildFleet(resources []FleetResource) map[string]*ClusterNamespaces {
	clusters := make(map[string]*ClusterNamespaces)
	forCluster := func(name string) *ClusterNamespaces {
		c, ok := clusters[name]
		if !ok {
			c = &ClusterNamespaces{Cluster: name}
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
		case kind == "Namespace" && apiVersion == "v1":
			cluster.Namespaces = append(cluster.Namespaces, &NamespaceEntity{
				Name: name, Labels: labels, Origin: fr.Origin,
			})
		case kind == "NetworkPolicy" && strings.HasPrefix(apiVersion, "networking.k8s.io/"):
			podSelector, hasPodSelector := asRecord(spec["podSelector"])
			egress, _ := spec["egress"].([]any)
			cluster.NetworkPolicies = append(cluster.NetworkPolicies, &NetworkPolicyEntity{
				Name:             name,
				Namespace:        namespace,
				PodSelectorEmpty: hasPodSelector && len(podSelector) == 0,
				PolicyTypes:      asStringArray(spec["policyTypes"]),
				HasEgressRules:   len(egress) > 0,
				Origin:           fr.Origin,
			})
		case rbacKinds[kind] && (apiVersion == "v1" || strings.HasPrefix(apiVersion, "rbac.authorization.k8s.io/")):
			cluster.RBAC = append(cluster.RBAC, &RBACEntity{
				Kind: kind, Name: name, Namespace: namespace, Origin: fr.Origin,
			})
		case workloadKinds[kind]:
			cluster.Workloads = append(cluster.Workloads, &WorkloadEntity{
				Kind: kind, Name: name, Namespace: namespace, Origin: fr.Origin,
			})
		}
	}
	return clusters
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

// --- lenient decoding helpers ---

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
