// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package workload is the workload-posture analysis engine: it parses pod-bearing
// workloads (Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob, bare
// Pod) and PodDisruptionBudgets drawn from ConfigHub Units into a typed domain
// model, then scores each workload's production-readiness posture — security
// context, resource requests/limits, probes, and operational hygiene — over it.
// (PDB coverage, the cross-Unit selector join, lands in a later milestone.)
//
// Parsing is lenient: malformed documents are skipped, never errored on — a bad
// resource in one Unit must not take down fleet-wide analysis.
package workload

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
	Cluster      string            `json:"cluster"`
	Target       string            `json:"target,omitempty"`
	Space        string            `json:"space"`
	SpaceID      string            `json:"spaceId"`
	SpaceLabels  map[string]string `json:"spaceLabels,omitempty"`
	UnitID       string            `json:"unitId"`
	UnitSlug     string            `json:"unitSlug"`
	ResourceName string            `json:"resourceName"`
	// Canonical is true for definitions in base/policy Spaces that aren't deployed
	// anywhere — shown in the explorer but excluded from cluster analysis.
	Canonical bool `json:"canonical,omitempty"`
}

// FleetResource is a parsed resource document plus its ConfigHub origin. Doc is
// the decoded JSON body (typically a map[string]any).
type FleetResource struct {
	Origin ResourceOrigin
	Doc    any
}

// ContainerPosture is the readiness-relevant state of one container, with the pod
// securityContext already folded in where the field defaults down from pod to
// container (runAsNonRoot, seccompProfile).
type ContainerPosture struct {
	Name string `json:"name"`
	// Security (effective values; nil = unset at both container and pod level).
	RunAsNonRoot            *bool  `json:"runAsNonRoot,omitempty"`
	RunAsUser               *int64 `json:"runAsUser,omitempty"`
	ReadOnlyRootFilesystem  *bool  `json:"readOnlyRootFilesystem,omitempty"`
	AllowPrivilegeEscalation *bool `json:"allowPrivilegeEscalation,omitempty"`
	Privileged              *bool  `json:"privileged,omitempty"`
	DropsAll                bool   `json:"dropsAll"`
	SeccompProfileType      string `json:"seccompProfileType,omitempty"`
	// Resources.
	HasCPURequest    bool `json:"hasCpuRequest"`
	HasMemoryRequest bool `json:"hasMemoryRequest"`
	HasCPULimit      bool `json:"hasCpuLimit"`
	HasMemoryLimit   bool `json:"hasMemoryLimit"`
	// Probes.
	HasLiveness  bool `json:"hasLiveness"`
	HasReadiness bool `json:"hasReadiness"`
	HasStartup   bool `json:"hasStartup"`
	// Hygiene.
	TerminationMessagePolicy string `json:"terminationMessagePolicy,omitempty"`
}

// WorkloadEntity is a parsed pod-bearing resource with its pod template reduced
// to the fields the scorers need.
type WorkloadEntity struct {
	Kind                        string             `json:"kind"`
	Name                        string             `json:"name"`
	Namespace                   string             `json:"namespace"`
	Replicas                    *int64             `json:"replicas,omitempty"`
	AutomountServiceAccountToken *bool             `json:"automountServiceAccountToken,omitempty"`
	HasAntiAffinity             bool               `json:"hasAntiAffinity"`
	HasTopologySpread           bool               `json:"hasTopologySpread"`
	PodLabels                   map[string]string  `json:"podLabels,omitempty"`
	Containers                  []ContainerPosture `json:"containers"`
	Origin                      ResourceOrigin     `json:"origin"`
}

// MultiReplica reports whether the workload runs more than one replica (so it
// wants a PDB and pod anti-affinity). Kinds without a replica field (DaemonSet,
// Job, CronJob, bare Pod) are treated as single-instance for this purpose.
func (w *WorkloadEntity) MultiReplica() bool {
	return w.Replicas != nil && *w.Replicas > 1
}

// LabelSelector is a parsed Kubernetes label selector (matchLabels +
// matchExpressions), used by PDB coverage (later milestone).
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

// PDBEntity is a parsed policy/v1 PodDisruptionBudget, reduced to its selector
// and disruption policy.
type PDBEntity struct {
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	Selector       LabelSelector  `json:"selector"`
	MinAvailable   string         `json:"minAvailable,omitempty"`
	MaxUnavailable string         `json:"maxUnavailable,omitempty"`
	Origin         ResourceOrigin `json:"origin"`
}

// ClusterWorkloads holds the workload-posture entities of one cluster.
type ClusterWorkloads struct {
	Cluster   string            `json:"cluster"`
	Workloads []*WorkloadEntity `json:"workloads"`
	PDBs      []*PDBEntity      `json:"pdbs"`
}

// workloadKinds is the set of pod-bearing kinds this manager scores.
var workloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"ReplicaSet":  true,
	"Job":         true,
	"CronJob":     true,
	"Pod":         true,
}

// IsWorkloadKind reports whether kind is a pod-bearing workload this manager
// scores.
func IsWorkloadKind(kind string) bool { return workloadKinds[kind] }

// BuildFleet indexes parsed fleet resources into per-cluster entity sets.
// Unrecognized kinds and unparseable docs are ignored. Entities within a cluster
// preserve input order.
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
		apiVersion, _ := asString(rec["apiVersion"])
		metadata, _ := asRecord(rec["metadata"])
		name, hasName := asString(metadata["name"])
		if !hasKind || !hasName {
			continue
		}
		namespace, _ := asString(metadata["namespace"])
		cluster := forCluster(fr.Origin.Cluster)

		switch {
		case kind == "PodDisruptionBudget" && strings.HasPrefix(apiVersion, "policy/"):
			spec, _ := asRecord(rec["spec"])
			cluster.PDBs = append(cluster.PDBs, &PDBEntity{
				Name:           name,
				Namespace:      namespace,
				Selector:       parseSelector(spec["selector"]),
				MinAvailable:   asScalarString(spec["minAvailable"]),
				MaxUnavailable: asScalarString(spec["maxUnavailable"]),
				Origin:         fr.Origin,
			})
		case workloadKinds[kind]:
			w := parseWorkload(kind, namespace, name, rec, fr.Origin)
			if w != nil {
				cluster.Workloads = append(cluster.Workloads, w)
			}
		}
	}
	return clusters
}

// parseWorkload extracts the readiness-relevant fields from a workload document.
func parseWorkload(kind, namespace, name string, rec map[string]any, origin ResourceOrigin) *WorkloadEntity {
	spec, _ := asRecord(rec["spec"])
	w := &WorkloadEntity{Kind: kind, Name: name, Namespace: namespace, Origin: origin}

	// Replicas (Deployment/StatefulSet/ReplicaSet carry it; others don't).
	switch kind {
	case "Deployment", "StatefulSet", "ReplicaSet":
		if r, ok := asInt(spec["replicas"]); ok {
			w.Replicas = &r
		}
	}

	podSpec := podSpecOf(kind, spec)
	if podSpec == nil {
		return w
	}
	if tmpl := podTemplateMeta(kind, spec); tmpl != nil {
		w.PodLabels = asStringMap(tmpl["labels"])
	}

	if b, ok := asBool(podSpec["automountServiceAccountToken"]); ok {
		w.AutomountServiceAccountToken = &b
	}
	podSec, _ := asRecord(podSpec["securityContext"])
	w.HasAntiAffinity = hasPodAntiAffinity(podSpec)
	w.HasTopologySpread = hasTopologySpread(podSpec)

	for _, c := range asArray(podSpec["containers"]) {
		cr, ok := asRecord(c)
		if !ok {
			continue
		}
		w.Containers = append(w.Containers, parseContainer(cr, podSec))
	}
	return w
}

// parseContainer folds the pod securityContext into the container's effective
// posture where Kubernetes defaults down (runAsNonRoot, runAsUser, seccomp).
func parseContainer(cr, podSec map[string]any) ContainerPosture {
	name, _ := asString(cr["name"])
	c := ContainerPosture{Name: name}
	cSec, _ := asRecord(cr["securityContext"])

	c.RunAsNonRoot = boolFrom(cSec["runAsNonRoot"], podSec["runAsNonRoot"])
	c.RunAsUser = intFrom(cSec["runAsUser"], podSec["runAsUser"])
	if b, ok := asBool(cSec["readOnlyRootFilesystem"]); ok {
		c.ReadOnlyRootFilesystem = &b
	}
	if b, ok := asBool(cSec["allowPrivilegeEscalation"]); ok {
		c.AllowPrivilegeEscalation = &b
	}
	if b, ok := asBool(cSec["privileged"]); ok {
		c.Privileged = &b
	}
	if caps, ok := asRecord(cSec["capabilities"]); ok {
		for _, d := range asStringArray(caps["drop"]) {
			if d == "ALL" || d == "all" {
				c.DropsAll = true
			}
		}
	}
	c.SeccompProfileType = seccompType(cSec, podSec)

	requests, limits := containerResources(cr)
	c.HasCPURequest = requests["cpu"]
	c.HasMemoryRequest = requests["memory"]
	c.HasCPULimit = limits["cpu"]
	c.HasMemoryLimit = limits["memory"]

	_, c.HasLiveness = asRecord(cr["livenessProbe"])
	_, c.HasReadiness = asRecord(cr["readinessProbe"])
	_, c.HasStartup = asRecord(cr["startupProbe"])

	c.TerminationMessagePolicy, _ = asString(cr["terminationMessagePolicy"])
	return c
}

// podSpecOf returns the pod spec for a workload kind, descending through the kind
// -specific template path.
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
	default: // Deployment/StatefulSet/DaemonSet/ReplicaSet/Job
		template, _ := asRecord(spec["template"])
		podSpec, _ := asRecord(template["spec"])
		return podSpec
	}
}

// podTemplateMeta returns the pod template metadata (for pod-template labels).
func podTemplateMeta(kind string, spec map[string]any) map[string]any {
	switch kind {
	case "Pod":
		return nil
	case "CronJob":
		jobTemplate, _ := asRecord(spec["jobTemplate"])
		jobSpec, _ := asRecord(jobTemplate["spec"])
		template, _ := asRecord(jobSpec["template"])
		md, _ := asRecord(template["metadata"])
		return md
	default:
		template, _ := asRecord(spec["template"])
		md, _ := asRecord(template["metadata"])
		return md
	}
}

func containerResources(cr map[string]any) (requests, limits map[string]bool) {
	requests = map[string]bool{}
	limits = map[string]bool{}
	res, ok := asRecord(cr["resources"])
	if !ok {
		return requests, limits
	}
	mark := func(section map[string]any, out map[string]bool) {
		for _, k := range []string{"cpu", "memory"} {
			if v := asScalarString(section[k]); v != "" {
				out[k] = true
			}
		}
	}
	if req, ok := asRecord(res["requests"]); ok {
		mark(req, requests)
	}
	if lim, ok := asRecord(res["limits"]); ok {
		mark(lim, limits)
	}
	return requests, limits
}

func seccompType(cSec, podSec map[string]any) string {
	if sp, ok := asRecord(cSec["seccompProfile"]); ok {
		if t, _ := asString(sp["type"]); t != "" {
			return t
		}
	}
	if sp, ok := asRecord(podSec["seccompProfile"]); ok {
		if t, _ := asString(sp["type"]); t != "" {
			return t
		}
	}
	return ""
}

func hasPodAntiAffinity(podSpec map[string]any) bool {
	affinity, ok := asRecord(podSpec["affinity"])
	if !ok {
		return false
	}
	paa, ok := asRecord(affinity["podAntiAffinity"])
	if !ok {
		return false
	}
	required := asArray(paa["requiredDuringSchedulingIgnoredDuringExecution"])
	preferred := asArray(paa["preferredDuringSchedulingIgnoredDuringExecution"])
	return len(required) > 0 || len(preferred) > 0
}

func hasTopologySpread(podSpec map[string]any) bool {
	return len(asArray(podSpec["topologySpreadConstraints"])) > 0
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
		out.MatchExpressions = append(out.MatchExpressions, SelectorRequirement{
			Key: key, Operator: op, Values: asStringArray(er["values"]),
		})
	}
	return out
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

func asBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

// asInt coerces a JSON number (always float64 from encoding/json) or integer to
// int64.
func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// asScalarString renders a scalar (string or number) as a string, "" otherwise.
// Used for resource quantities ("500m", "256Mi") and PDB min/max (int or "%").
func asScalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// Render whole numbers without a trailing ".0".
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	}
	return ""
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

// boolFrom returns the first set bool among the given values (container then
// pod), or nil if neither is a bool.
func boolFrom(vals ...any) *bool {
	for _, v := range vals {
		if b, ok := asBool(v); ok {
			return &b
		}
	}
	return nil
}

// intFrom returns the first set int among the given values, or nil.
func intFrom(vals ...any) *int64 {
	for _, v := range vals {
		if n, ok := asInt(v); ok {
			return &n
		}
	}
	return nil
}
