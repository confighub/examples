// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package autoscale is the autoscaling analysis engine: it parses
// HorizontalPodAutoscalers, KEDA ScaledObjects, pod-bearing workloads, and
// PodDisruptionBudgets into a typed model and computes autoscaling findings —
// including the cross-resource HPA-minReplicas vs PDB-minAvailable check.
//
// Parsing is lenient: malformed documents are skipped, never errored on.
package autoscale

import "strings"

// ResourceOrigin records where a resource came from in ConfigHub.
type ResourceOrigin struct {
	Cluster      string            `json:"cluster"`
	Target       string            `json:"target,omitempty"`
	Space        string            `json:"space"`
	SpaceID      string            `json:"spaceId"`
	SpaceLabels  map[string]string `json:"spaceLabels,omitempty"`
	UnitID       string            `json:"unitId"`
	UnitSlug     string            `json:"unitSlug"`
	ResourceName string            `json:"resourceName"`
	Canonical    bool              `json:"canonical,omitempty"`
}

// FleetResource is a parsed resource document plus its ConfigHub origin.
type FleetResource struct {
	Origin ResourceOrigin
	Doc    any
}

// LabelSelector is a parsed Kubernetes label selector.
type LabelSelector struct {
	MatchLabels      map[string]string     `json:"matchLabels,omitempty"`
	MatchExpressions []SelectorRequirement `json:"matchExpressions,omitempty"`
}

// SelectorRequirement is one matchExpressions entry.
type SelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// AutoscalerKind is HPA or ScaledObject.
type AutoscalerKind string

const (
	KindHPA          AutoscalerKind = "HorizontalPodAutoscaler"
	KindScaledObject AutoscalerKind = "ScaledObject"
)

// Autoscaler is the common shape of an HPA or a KEDA ScaledObject.
type Autoscaler struct {
	Kind       AutoscalerKind `json:"kind"`
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace"`
	TargetKind string         `json:"targetKind,omitempty"`
	TargetName string         `json:"targetName"`
	Min        *int64         `json:"min,omitempty"`
	Max        *int64         `json:"max,omitempty"`
	Origin     ResourceOrigin `json:"origin"`
}

// Pinned reports whether the autoscaler can't actually scale (min == max, both set).
func (a *Autoscaler) Pinned() bool {
	return a.Min != nil && a.Max != nil && *a.Min == *a.Max
}

// WorkloadEntity is a scalable workload (Deployment / StatefulSet), reduced to its
// replica count and pod-template labels.
type WorkloadEntity struct {
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Replicas  *int64            `json:"replicas,omitempty"`
	PodLabels map[string]string `json:"podLabels,omitempty"`
	Origin    ResourceOrigin    `json:"origin"`
}

// PDBEntity is a parsed policy/v1 PodDisruptionBudget.
type PDBEntity struct {
	Name         string         `json:"name"`
	Namespace    string         `json:"namespace"`
	MinAvailable string         `json:"minAvailable,omitempty"`
	Selector     LabelSelector  `json:"selector"`
	Origin       ResourceOrigin `json:"origin"`
}

// ClusterAutoscale holds the autoscaling entities of one cluster.
type ClusterAutoscale struct {
	Cluster     string            `json:"cluster"`
	Autoscalers []*Autoscaler     `json:"autoscalers"`
	Workloads   []*WorkloadEntity `json:"workloads"`
	PDBs        []*PDBEntity      `json:"pdbs"`
}

// scalableKinds are the workload kinds that can be autoscaled by an HPA/ScaledObject.
var scalableKinds = map[string]bool{"Deployment": true, "StatefulSet": true}

// IsScalableKind reports whether kind is a Deployment or StatefulSet.
func IsScalableKind(kind string) bool { return scalableKinds[kind] }

// BuildFleet indexes parsed fleet resources into per-cluster entity sets.
func BuildFleet(resources []FleetResource) map[string]*ClusterAutoscale {
	clusters := make(map[string]*ClusterAutoscale)
	forCluster := func(name string) *ClusterAutoscale {
		c, ok := clusters[name]
		if !ok {
			c = &ClusterAutoscale{Cluster: name}
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
		spec, _ := asRecord(rec["spec"])
		c := forCluster(fr.Origin.Cluster)

		switch {
		case kind == "HorizontalPodAutoscaler" && strings.HasPrefix(apiVersion, "autoscaling/"):
			ref, _ := asRecord(spec["scaleTargetRef"])
			tName, _ := asString(ref["name"])
			tKind, _ := asString(ref["kind"])
			c.Autoscalers = append(c.Autoscalers, &Autoscaler{
				Kind: KindHPA, Name: name, Namespace: namespace,
				TargetKind: tKind, TargetName: tName,
				Min: intPtr(spec["minReplicas"]), Max: intPtr(spec["maxReplicas"]),
				Origin: fr.Origin,
			})
		case kind == "ScaledObject" && strings.HasPrefix(apiVersion, "keda.sh/"):
			ref, _ := asRecord(spec["scaleTargetRef"])
			tName, _ := asString(ref["name"])
			tKind, _ := asString(ref["kind"])
			c.Autoscalers = append(c.Autoscalers, &Autoscaler{
				Kind: KindScaledObject, Name: name, Namespace: namespace,
				TargetKind: tKind, TargetName: tName,
				Min: intPtr(spec["minReplicaCount"]), Max: intPtr(spec["maxReplicaCount"]),
				Origin: fr.Origin,
			})
		case scalableKinds[kind]:
			w := &WorkloadEntity{Kind: kind, Name: name, Namespace: namespace, Replicas: intPtr(spec["replicas"]), Origin: fr.Origin}
			if tmpl, ok := asRecord(spec["template"]); ok {
				if md, ok := asRecord(tmpl["metadata"]); ok {
					w.PodLabels = asStringMap(md["labels"])
				}
			}
			c.Workloads = append(c.Workloads, w)
		case kind == "PodDisruptionBudget" && strings.HasPrefix(apiVersion, "policy/"):
			c.PDBs = append(c.PDBs, &PDBEntity{
				Name: name, Namespace: namespace,
				MinAvailable: asScalarString(spec["minAvailable"]),
				Selector:     parseSelector(spec["selector"]),
				Origin:       fr.Origin,
			})
		}
	}
	return clusters
}

func parseSelector(v any) LabelSelector {
	sel, ok := asRecord(v)
	if !ok {
		return LabelSelector{}
	}
	out := LabelSelector{MatchLabels: asStringMap(sel["matchLabels"])}
	for _, e := range asArray(sel["matchExpressions"]) {
		er, ok := asRecord(e)
		if !ok {
			continue
		}
		key, _ := asString(er["key"])
		op, _ := asString(er["operator"])
		out.MatchExpressions = append(out.MatchExpressions, SelectorRequirement{Key: key, Operator: op, Values: asStringArray(er["values"])})
	}
	return out
}

// ResourceMeta extracts kind, name, namespace from a decoded resource document.
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

func intPtr(v any) *int64 {
	switch n := v.(type) {
	case float64:
		i := int64(n)
		return &i
	case int64:
		return &n
	case int:
		i := int64(n)
		return &i
	}
	return nil
}

func asScalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return itoa(int64(t))
		}
	case int64:
		return itoa(t)
	case int:
		return itoa(int64(t))
	}
	return ""
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func asArray(v any) []any {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	return a
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
		return nil
	}
	out := make(map[string]string, len(rec))
	for k, val := range rec {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}
