// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package observability is the observability-posture analysis engine: it parses
// Prometheus ServiceMonitors, Services, and pod-bearing workloads into a typed
// model and computes ServiceMonitor coverage of metrics-exposing Services (a
// cross-Unit selector join) and telemetry-sidecar presence on workloads.
//
// Parsing is lenient: malformed documents are skipped, never errored on.
package observability

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

// ServiceMonitorEntity is a parsed monitoring.coreos.com ServiceMonitor.
type ServiceMonitorEntity struct {
	Name          string         `json:"name"`
	Namespace     string         `json:"namespace"`
	Selector      LabelSelector  `json:"selector"`
	EndpointPorts []string       `json:"endpointPorts,omitempty"`
	Origin        ResourceOrigin `json:"origin"`
}

// ServicePort is a parsed Service port.
type ServicePort struct {
	Name string `json:"name,omitempty"`
	Port int64  `json:"port,omitempty"`
}

// ServiceEntity is a parsed v1 Service.
type ServiceEntity struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Ports       []ServicePort     `json:"ports,omitempty"`
	Origin      ResourceOrigin    `json:"origin"`
}

// metricsPortNames are Service/container port names that conventionally expose
// Prometheus metrics.
var metricsPortNames = map[string]bool{
	"metrics": true, "http-metrics": true, "monitoring": true, "prometheus": true, "telemetry": true,
}

// ExposesMetrics reports whether the Service looks like a metrics scrape target:
// a port named like a metrics port, or a prometheus.io/scrape=true annotation.
func (s *ServiceEntity) ExposesMetrics() bool {
	if s.Annotations["prometheus.io/scrape"] == "true" {
		return true
	}
	for _, p := range s.Ports {
		if metricsPortNames[strings.ToLower(p.Name)] {
			return true
		}
	}
	return false
}

// WorkloadEntity is a parsed pod-bearing workload, reduced to its container names
// (for telemetry-sidecar presence).
type WorkloadEntity struct {
	Kind           string         `json:"kind"`
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	ContainerNames []string       `json:"containerNames,omitempty"`
	Origin         ResourceOrigin `json:"origin"`
}

// HasContainer reports whether the workload has a container with the given name.
func (w *WorkloadEntity) HasContainer(name string) bool {
	for _, c := range w.ContainerNames {
		if c == name {
			return true
		}
	}
	return false
}

// ClusterObservability holds the observability entities of one cluster.
type ClusterObservability struct {
	Cluster         string                  `json:"cluster"`
	ServiceMonitors []*ServiceMonitorEntity `json:"serviceMonitors"`
	Services        []*ServiceEntity        `json:"services"`
	Workloads       []*WorkloadEntity       `json:"workloads"`
}

var workloadKinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true,
	"ReplicaSet": true, "Job": true, "CronJob": true, "Pod": true,
}

// IsWorkloadKind reports whether kind is a pod-bearing workload.
func IsWorkloadKind(kind string) bool { return workloadKinds[kind] }

// BuildFleet indexes parsed fleet resources into per-cluster entity sets.
func BuildFleet(resources []FleetResource) map[string]*ClusterObservability {
	clusters := make(map[string]*ClusterObservability)
	forCluster := func(name string) *ClusterObservability {
		c, ok := clusters[name]
		if !ok {
			c = &ClusterObservability{Cluster: name}
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
		case kind == "ServiceMonitor" && strings.HasPrefix(apiVersion, "monitoring.coreos.com/"):
			c.ServiceMonitors = append(c.ServiceMonitors, &ServiceMonitorEntity{
				Name:          name,
				Namespace:     namespace,
				Selector:      parseSelector(spec["selector"]),
				EndpointPorts: endpointPorts(spec["endpoints"]),
				Origin:        fr.Origin,
			})
		case kind == "Service" && apiVersion == "v1":
			c.Services = append(c.Services, &ServiceEntity{
				Name:        name,
				Namespace:   namespace,
				Labels:      asStringMap(metadata["labels"]),
				Annotations: asStringMap(metadata["annotations"]),
				Ports:       servicePorts(spec["ports"]),
				Origin:      fr.Origin,
			})
		case workloadKinds[kind]:
			c.Workloads = append(c.Workloads, &WorkloadEntity{
				Kind: kind, Name: name, Namespace: namespace,
				ContainerNames: containerNames(kind, spec),
				Origin:         fr.Origin,
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

func endpointPorts(v any) []string {
	var out []string
	for _, e := range asArray(v) {
		er, ok := asRecord(e)
		if !ok {
			continue
		}
		if p, ok := asString(er["port"]); ok && p != "" {
			out = append(out, p)
		}
	}
	return out
}

func servicePorts(v any) []ServicePort {
	var out []ServicePort
	for _, e := range asArray(v) {
		er, ok := asRecord(e)
		if !ok {
			continue
		}
		name, _ := asString(er["name"])
		port, _ := asInt(er["port"])
		out = append(out, ServicePort{Name: name, Port: port})
	}
	return out
}

func containerNames(kind string, spec map[string]any) []string {
	podSpec := podSpecOf(kind, spec)
	if podSpec == nil {
		return nil
	}
	var out []string
	for _, c := range asArray(podSpec["containers"]) {
		cr, ok := asRecord(c)
		if !ok {
			continue
		}
		if n, ok := asString(cr["name"]); ok {
			out = append(out, n)
		}
	}
	return out
}

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
