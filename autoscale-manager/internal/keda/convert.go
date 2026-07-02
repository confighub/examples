// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package keda holds the convert-hpa-to-keda function: a mutating ConfigHub
// function that rewrites a HorizontalPodAutoscaler into a KEDA ScaledObject,
// preserving min/max replicas and cpu/memory metrics. It is written as a normal
// ConfigHub function (a handler.FunctionImplementation) so it can run in the
// embedded executor today (see internal/localexec) and move into the SDK
// unchanged later.
package keda

import (
	"fmt"
	"strings"

	api "github.com/confighub/sdk/core/function/api"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/third_party/gaby"
)

// FunctionName is the registered name of the conversion function.
const FunctionName = "convert-hpa-to-keda"

// Signature describes the convert-hpa-to-keda function.
func Signature() api.FunctionSignature {
	return api.FunctionSignature{
		FunctionName:          FunctionName,
		Mutating:              true,
		Hermetic:              true,
		Idempotent:            true, // a second run finds no HPA and is a no-op
		Description:           "Convert a HorizontalPodAutoscaler into a KEDA ScaledObject, preserving min/max replicas and cpu/memory metrics",
		FunctionType:          api.FunctionTypeCustom,
		AffectedResourceTypes: []api.ResourceType{api.ResourceTypeAny},
	}
}

// Convert is the function implementation: it replaces every HorizontalPodAutoscaler
// document in the Unit's data with an equivalent KEDA ScaledObject.
func Convert(fArgs handler.FunctionImplementationArguments) (gaby.Container, any, error) {
	data := fArgs.ParsedData
	for i, doc := range data {
		if doc == nil {
			continue
		}
		if gstr(doc, "kind") != "HorizontalPodAutoscaler" || !strings.HasPrefix(gstr(doc, "apiVersion"), "autoscaling/") {
			continue
		}
		so, err := scaledObjectFromHPA(doc)
		if err != nil {
			return data, nil, err
		}
		data[i] = so
	}
	return data, nil, nil
}

type trigger struct {
	kind       string // cpu | memory
	metricType string // Utilization | AverageValue
	value      string
}

// scaledObjectFromHPA builds a KEDA ScaledObject document from an HPA document.
func scaledObjectFromHPA(hpa *gaby.YamlDoc) (*gaby.YamlDoc, error) {
	name := gstr(hpa, "metadata", "name")
	targetName := gstr(hpa, "spec", "scaleTargetRef", "name")
	if name == "" || targetName == "" {
		return nil, fmt.Errorf("HorizontalPodAutoscaler missing metadata.name or spec.scaleTargetRef.name")
	}
	namespace := gstr(hpa, "metadata", "namespace")
	targetKind := gstr(hpa, "spec", "scaleTargetRef", "kind")
	min := gstr(hpa, "spec", "minReplicas")
	max := gstr(hpa, "spec", "maxReplicas")

	triggers := triggersFromMetrics(hpa)
	if len(triggers) == 0 {
		return nil, fmt.Errorf("HorizontalPodAutoscaler %q has no convertible cpu/memory Resource metrics — KEDA requires at least one trigger (Pods/Object/External metrics are not converted)", name)
	}

	var b strings.Builder
	b.WriteString("apiVersion: keda.sh/v1alpha1\n")
	b.WriteString("kind: ScaledObject\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	if namespace != "" {
		b.WriteString("  namespace: " + namespace + "\n")
	}
	b.WriteString("spec:\n")
	b.WriteString("  scaleTargetRef:\n")
	b.WriteString("    name: " + targetName + "\n")
	if targetKind != "" && targetKind != "Deployment" {
		b.WriteString("    kind: " + targetKind + "\n")
	}
	if min != "" {
		b.WriteString("  minReplicaCount: " + min + "\n")
	}
	if max != "" {
		b.WriteString("  maxReplicaCount: " + max + "\n")
	}
	b.WriteString("  triggers:\n")
	for _, t := range triggers {
		b.WriteString("  - type: " + t.kind + "\n")
		b.WriteString("    metricType: " + t.metricType + "\n")
		b.WriteString("    metadata:\n")
		b.WriteString("      value: \"" + t.value + "\"\n")
	}

	doc, err := gaby.ParseYAML([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("build ScaledObject for %q: %w", name, err)
	}
	return doc, nil
}

// triggersFromMetrics maps HPA autoscaling/v2 Resource metrics (cpu/memory) to
// KEDA cpu/memory triggers. Non-Resource metrics (Pods/Object/External) are
// skipped — they have no direct cpu/memory-scaler equivalent.
func triggersFromMetrics(hpa *gaby.YamlDoc) []trigger {
	metrics := hpa.S("spec", "metrics")
	if metrics == nil {
		return nil
	}
	var out []trigger
	for _, m := range metrics.Children() {
		if gstr(m, "type") != "Resource" {
			continue
		}
		res := gstr(m, "resource", "name")
		if res != "cpu" && res != "memory" {
			continue
		}
		switch gstr(m, "resource", "target", "type") {
		case "Utilization":
			if v := gstr(m, "resource", "target", "averageUtilization"); v != "" {
				out = append(out, trigger{kind: res, metricType: "Utilization", value: v})
			}
		case "AverageValue":
			if v := gstr(m, "resource", "target", "averageValue"); v != "" {
				out = append(out, trigger{kind: res, metricType: "AverageValue", value: v})
			}
		}
	}
	return out
}

// gstr reads a nested scalar as a string, "" if absent.
func gstr(d *gaby.YamlDoc, keys ...string) string {
	n := d.S(keys...)
	if n == nil {
		return ""
	}
	v := n.Data()
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
