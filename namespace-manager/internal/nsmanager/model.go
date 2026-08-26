// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package nsmanager is the namespace analysis engine: it parses Namespaces and
// the pod-bearing workloads that occupy them, drawn from ConfigHub Units, into a
// typed domain model, then computes per-namespace envelope completeness and
// fleet-wide consistency checks over it.
//
// "Envelope" is what a namespace should carry to be governed at all: the
// Namespace object itself and its pod-security labels. The manager reports which
// namespaces are missing members, which names collide on one cluster, and which
// components let a namespace name drift across variants — the fleet-wide read a
// per-resource validator or a runtime tenancy controller (Capsule, HNC) cannot
// do.
//
// It deliberately stops there. Whether a namespace has the NetworkPolicy
// coverage it needs is network-policy-manager's subject, and whether its RBAC is
// sound is rbac-manager's; each reasons about its own resources far more
// carefully than a completeness check here could.
//
// Parsing is lenient: malformed documents are skipped, never errored on — a bad
// resource in one Unit must not take down fleet-wide analysis.
package nsmanager

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
	Cluster    string             `json:"cluster"`
	Namespaces []*NamespaceEntity `json:"namespaces"`
	Workloads  []*WorkloadEntity  `json:"workloads"`
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

		switch {
		case kind == "Namespace" && apiVersion == "v1":
			cluster.Namespaces = append(cluster.Namespaces, &NamespaceEntity{
				Name: name, Labels: labels, Origin: fr.Origin,
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
