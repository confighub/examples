// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package observability

import (
	"fmt"
	"sort"
)

// Empty reports whether the selector has no terms. An empty ServiceMonitor
// selector selects every Service in its namespace.
func (s LabelSelector) Empty() bool {
	return len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0
}

// Matches reports whether the given labels satisfy the selector (matchLabels +
// In/NotIn/Exists/DoesNotExist), the same semantics the netpol/PDB coverage joins
// use.
func (s LabelSelector) Matches(labels map[string]string) bool {
	for k, v := range s.MatchLabels {
		if labels[k] != v {
			return false
		}
	}
	for _, r := range s.MatchExpressions {
		val, present := labels[r.Key]
		switch r.Operator {
		case "In":
			if !present || !contains(r.Values, val) {
				return false
			}
		case "NotIn":
			if present && contains(r.Values, val) {
				return false
			}
		case "Exists":
			if !present {
				return false
			}
		case "DoesNotExist":
			if present {
				return false
			}
		}
	}
	return true
}

func contains(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

// coveringMonitors returns the ServiceMonitors (same namespace) whose selector
// matches the Service's labels.
func coveringMonitors(svc *ServiceEntity, sms []*ServiceMonitorEntity) []*ServiceMonitorEntity {
	var out []*ServiceMonitorEntity
	for _, sm := range sms {
		if sm.Namespace != svc.Namespace {
			continue
		}
		if sm.Selector.Matches(svc.Labels) {
			out = append(out, sm)
		}
	}
	return out
}

// CoverageResult is the per-Service ServiceMonitor-coverage verdict (for
// metrics-exposing Services).
type CoverageResult struct {
	Cluster    string   `json:"cluster"`
	Namespace  string   `json:"namespace,omitempty"`
	Service    string   `json:"service"`
	Space      string   `json:"space"`
	UnitSlug   string   `json:"unitSlug"`
	SpaceID    string   `json:"spaceId,omitempty"`
	Covered    bool     `json:"covered"`
	Monitors   []string `json:"monitors,omitempty"`
	MetricPort string   `json:"metricPort,omitempty"`
}

// AnalyzeCoverage reports ServiceMonitor coverage for every metrics-exposing
// Service in the fleet, sorted by (cluster, namespace, service).
func AnalyzeCoverage(clusters map[string]*ClusterObservability) []CoverageResult {
	var out []CoverageResult
	for _, c := range clusters {
		for _, svc := range c.Services {
			if !svc.ExposesMetrics() {
				continue
			}
			ms := coveringMonitors(svc, c.ServiceMonitors)
			r := CoverageResult{
				Cluster:    svc.Origin.Cluster,
				Namespace:  svc.Namespace,
				Service:    svc.Name,
				Space:      svc.Origin.Space,
				UnitSlug:   svc.Origin.UnitSlug,
				SpaceID:    svc.Origin.SpaceID,
				Covered:    len(ms) > 0,
				MetricPort: metricPortName(svc),
			}
			for _, m := range ms {
				r.Monitors = append(r.Monitors, m.Name)
			}
			out = append(out, r)
		}
	}
	sortCoverage(out)
	return out
}

func metricPortName(svc *ServiceEntity) string {
	for _, p := range svc.Ports {
		if metricsPortNames[toLower(p.Name)] {
			return p.Name
		}
	}
	if svc.Annotations["prometheus.io/scrape"] == "true" {
		return "prometheus.io/scrape"
	}
	return ""
}

// DanglingMonitor is a ServiceMonitor that matches no Service in its namespace.
type DanglingMonitor struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Space     string `json:"space"`
	UnitSlug  string `json:"unitSlug"`
}

// DanglingMonitors flags ServiceMonitors that select nothing.
func DanglingMonitors(clusters map[string]*ClusterObservability) []DanglingMonitor {
	var out []DanglingMonitor
	for _, c := range clusters {
		for _, sm := range c.ServiceMonitors {
			matched := false
			for _, svc := range c.Services {
				if sm.Namespace == svc.Namespace && sm.Selector.Matches(svc.Labels) {
					matched = true
					break
				}
			}
			if !matched {
				out = append(out, DanglingMonitor{
					Cluster: sm.Origin.Cluster, Namespace: sm.Namespace, Name: sm.Name,
					Space: sm.Origin.Space, UnitSlug: sm.Origin.UnitSlug,
				})
			}
		}
	}
	return out
}

// SidecarResult reports telemetry-sidecar presence for one workload.
type SidecarResult struct {
	Cluster    string `json:"cluster"`
	Namespace  string `json:"namespace,omitempty"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Space      string `json:"space"`
	UnitSlug   string `json:"unitSlug"`
	HasSidecar bool   `json:"hasSidecar"`
	Sidecar    string `json:"sidecar,omitempty"`
}

// DefaultOtelContainerNames are the container names conventionally used for an
// OpenTelemetry / telemetry collector sidecar.
var DefaultOtelContainerNames = []string{"otel-collector", "otc-container", "opentelemetry-collector", "otel-agent"}

// AnalyzeSidecars reports, per workload, whether it has a container matching one
// of the given sidecar names (defaults to the otel names when empty).
func AnalyzeSidecars(clusters map[string]*ClusterObservability, names []string) []SidecarResult {
	if len(names) == 0 {
		names = DefaultOtelContainerNames
	}
	var out []SidecarResult
	for _, c := range clusters {
		for _, w := range c.Workloads {
			r := SidecarResult{
				Cluster: w.Origin.Cluster, Namespace: w.Namespace, Kind: w.Kind, Name: w.Name,
				Space: w.Origin.Space, UnitSlug: w.Origin.UnitSlug,
			}
			for _, n := range names {
				if w.HasContainer(n) {
					r.HasSidecar = true
					r.Sidecar = n
					break
				}
			}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Severity ranks a finding for triage.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

func severityRank(s Severity) int {
	switch s {
	case SeverityHigh:
		return 2
	case SeverityMedium:
		return 1
	default:
		return 0
	}
}

// Finding is one observability issue.
type Finding struct {
	Severity  Severity `json:"severity"`
	Analyzer  string   `json:"analyzer"`
	Cluster   string   `json:"cluster"`
	Namespace string   `json:"namespace,omitempty"`
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Space     string   `json:"space"`
	UnitSlug  string   `json:"unitSlug"`
	Message   string   `json:"message"`
}

// Findings computes observability findings across the fleet, most-severe first:
// a metrics-exposing Service with no covering ServiceMonitor (the cross-Unit
// coverage join), and a dangling ServiceMonitor (selects nothing).
func Findings(clusters map[string]*ClusterObservability) []Finding {
	var out []Finding
	for _, r := range AnalyzeCoverage(clusters) {
		if r.Covered {
			continue
		}
		out = append(out, Finding{
			Severity: SeverityMedium, Analyzer: "coverage",
			Cluster: r.Cluster, Namespace: r.Namespace, Kind: "Service", Name: r.Service,
			Space: r.Space, UnitSlug: r.UnitSlug,
			Message: fmt.Sprintf("Service exposes metrics (%s) but no ServiceMonitor selects it", r.MetricPort),
		})
	}
	for _, d := range DanglingMonitors(clusters) {
		out = append(out, Finding{
			Severity: SeverityLow, Analyzer: "dangling",
			Cluster: d.Cluster, Namespace: d.Namespace, Kind: "ServiceMonitor", Name: d.Name,
			Space: d.Space, UnitSlug: d.UnitSlug,
			Message: "ServiceMonitor selects no Service in its namespace",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortCoverage(out []CoverageResult) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Service < out[j].Service
	})
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
