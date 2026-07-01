// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package scheduling is the placement analysis engine: it parses the placement
// fields of pod-bearing workloads (nodeSelector, tolerations, node affinity) into
// a typed model and reports where each workload is allowed to land, plus
// placement anti-patterns across the fleet.
//
// Placement is "which node a pod lands on". Spreading a workload's own replicas
// (pod anti-affinity, topology spread) is an availability concern owned by
// cub-workload, and is deliberately out of scope here.
//
// Parsing is lenient: malformed documents are skipped, never errored on.
package scheduling

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

// Toleration is a parsed pod toleration.
type Toleration struct {
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"`
}

// WorkloadPlacement is a pod-bearing workload reduced to its placement fields.
type WorkloadPlacement struct {
	Kind                    string            `json:"kind"`
	Name                    string            `json:"name"`
	Namespace               string            `json:"namespace"`
	NodeSelector            map[string]string `json:"nodeSelector,omitempty"`
	Tolerations             []Toleration      `json:"tolerations,omitempty"`
	HasNodeAffinity         bool              `json:"hasNodeAffinity"`
	HasRequiredNodeAffinity bool              `json:"hasRequiredNodeAffinity"`
	Origin                  ResourceOrigin    `json:"origin"`
}

// HasNodeSelector reports whether the workload pins to nodes by label.
func (w *WorkloadPlacement) HasNodeSelector() bool { return len(w.NodeSelector) > 0 }

// HasTolerations reports whether the workload tolerates any taint.
func (w *WorkloadPlacement) HasTolerations() bool { return len(w.Tolerations) > 0 }

// Constrained reports whether the workload actually restricts which nodes it lands
// on: a nodeSelector or a required node affinity. Tolerations alone do NOT
// constrain placement — they only permit scheduling onto tainted nodes.
func (w *WorkloadPlacement) Constrained() bool {
	return w.HasNodeSelector() || w.HasRequiredNodeAffinity
}

// TolerationKeys returns the distinct toleration keys (for reporting).
func (w *WorkloadPlacement) TolerationKeys() []string {
	var keys []string
	seen := map[string]bool{}
	for _, t := range w.Tolerations {
		k := t.Key
		if k == "" {
			k = "*" // an empty key with Exists tolerates everything
		}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

// ClusterWorkloads holds the placement entities of one cluster.
type ClusterWorkloads struct {
	Cluster   string               `json:"cluster"`
	Workloads []*WorkloadPlacement `json:"workloads"`
}

var workloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"ReplicaSet":  true,
	"Job":         true,
	"CronJob":     true,
	"Pod":         true,
}

// IsWorkloadKind reports whether kind is a pod-bearing workload this tool scores.
func IsWorkloadKind(kind string) bool { return workloadKinds[kind] }

// BuildFleet indexes parsed fleet resources into per-cluster placement sets.
func BuildFleet(resources []FleetResource) map[string]*ClusterWorkloads {
	clusters := make(map[string]*ClusterWorkloads)
	forCluster := func(name string) *ClusterWorkloads {
		c, ok := clusters[name]
		if !ok {
			c = &ClusterWorkloads{Cluster: name}
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
		metadata, _ := asRecord(rec["metadata"])
		name, hasName := asString(metadata["name"])
		if !hasKind || !hasName || !workloadKinds[kind] {
			continue
		}
		namespace, _ := asString(metadata["namespace"])
		spec, _ := asRecord(rec["spec"])
		podSpec := podSpecOf(kind, spec)
		w := &WorkloadPlacement{Kind: kind, Name: name, Namespace: namespace, Origin: fr.Origin}
		if podSpec != nil {
			w.NodeSelector = asStringMap(podSpec["nodeSelector"])
			w.Tolerations = parseTolerations(podSpec["tolerations"])
			w.HasNodeAffinity, w.HasRequiredNodeAffinity = parseNodeAffinity(podSpec["affinity"])
		}
		forCluster(fr.Origin.Cluster).Workloads = append(forCluster(fr.Origin.Cluster).Workloads, w)
	}
	return clusters
}

func parseTolerations(v any) []Toleration {
	arr := asArray(v)
	if len(arr) == 0 {
		return nil
	}
	out := make([]Toleration, 0, len(arr))
	for _, e := range arr {
		er, ok := asRecord(e)
		if !ok {
			continue
		}
		key, _ := asString(er["key"])
		op, _ := asString(er["operator"])
		val, _ := asString(er["value"])
		effect, _ := asString(er["effect"])
		out = append(out, Toleration{Key: key, Operator: op, Value: val, Effect: effect})
	}
	return out
}

func parseNodeAffinity(v any) (has, hasRequired bool) {
	affinity, ok := asRecord(v)
	if !ok {
		return false, false
	}
	na, ok := asRecord(affinity["nodeAffinity"])
	if !ok {
		return false, false
	}
	_, hasRequired = asRecord(na["requiredDuringSchedulingIgnoredDuringExecution"])
	preferred := asArray(na["preferredDuringSchedulingIgnoredDuringExecution"])
	return hasRequired || len(preferred) > 0, hasRequired
}

// podSpecOf returns the pod spec for a workload kind, descending through the
// kind-specific template path.
func podSpecOf(kind string, spec map[string]any) map[string]any {
	switch kind {
	case "Pod":
		return spec
	case "CronJob":
		jobTemplate, _ := asRecord(spec["jobTemplate"])
		jobSpec, _ := asRecord(jobTemplate["spec"])
		template, _ := asRecord(jobSpec["template"])
		podSpec, _ := asRecord(template["spec"])
		return podSpec
	default:
		template, _ := asRecord(spec["template"])
		podSpec, _ := asRecord(template["spec"])
		return podSpec
	}
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

func asArray(v any) []any {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	return a
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
