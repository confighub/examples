// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package convert turns the Kubernetes resources held in a ConfigHub Space into
// Score workload specifications.
//
// The mapping is deliberately lossy in one direction only: everything it emits
// is faithful to the source config, and everything it cannot express is
// reported rather than dropped. Deployments and StatefulSets become Score
// workloads; the Services, Ingresses, ConfigMaps, Secrets and PVCs around them
// are folded into those workloads as service ports, routes, files and volumes.
package convert

import (
	"fmt"
	"sort"

	scoretypes "github.com/score-spec/score-go/types"

	"github.com/confighub/examples/k8s-to-score/internal/k8s"
)

// ScoreAPIVersion is the Score specification version this tool emits.
const ScoreAPIVersion = "score.dev/v1b1"

// WorkloadKindAnnotation tells score-k8s which Kubernetes kind to render a
// workload back as. Score itself has no notion of Deployment vs StatefulSet.
const WorkloadKindAnnotation = "k8s.score.dev/kind"

// Result is a completed conversion: the Score workloads to write, plus
// everything the converter could not represent.
type Result struct {
	Workloads []NamedWorkload `json:"workloads"`
	Warnings  []Warning       `json:"warnings"`
	Skipped   []Skipped       `json:"skipped"`
}

// NamedWorkload is one Score workload and the file basename it should be
// written under.
type NamedWorkload struct {
	Name     string              `json:"name"`
	UnitSlug string              `json:"unit"`
	Kind     string              `json:"kind"`
	Workload scoretypes.Workload `json:"-"`
}

// Warning is a piece of source config the converter could not fully represent.
type Warning struct {
	Workload string `json:"workload"`
	Unit     string `json:"unit"`
	Message  string `json:"message"`
}

// Skipped is a Kubernetes resource with no Score representation at all.
type Skipped struct {
	Unit string `json:"unit"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type converter struct {
	inv      *inventory
	warnings []Warning
}

// Convert decodes every Unit's config data and converts it. It returns a Result
// even when there are warnings; an error means the input could not be parsed at
// all.
func Convert(units []UnitData) (*Result, error) {
	var objs []k8s.Object
	for _, u := range units {
		decoded, err := k8s.Decode(u.Slug, u.Data)
		if err != nil {
			return nil, err
		}
		objs = append(objs, decoded...)
	}

	inv, err := buildInventory(objs)
	if err != nil {
		return nil, err
	}

	c := &converter{inv: inv}
	res := &Result{}

	for _, w := range inv.workloads {
		res.Workloads = append(res.Workloads, NamedWorkload{
			Name:     w.name,
			UnitSlug: w.src.UnitSlug,
			Kind:     w.kind,
			Workload: c.convertWorkload(w),
		})
	}
	sort.Slice(res.Workloads, func(i, j int) bool { return res.Workloads[i].Name < res.Workloads[j].Name })

	for _, o := range inv.unsupported {
		res.Skipped = append(res.Skipped, Skipped{Unit: o.UnitSlug, Kind: o.Kind, Name: o.Name})
	}
	sort.Slice(res.Skipped, func(i, j int) bool {
		if res.Skipped[i].Kind != res.Skipped[j].Kind {
			return res.Skipped[i].Kind < res.Skipped[j].Kind
		}
		return res.Skipped[i].Name < res.Skipped[j].Name
	})

	res.Warnings = c.warnings
	return res, nil
}

// UnitData is the input to Convert: one Unit's slug and its config data. It
// mirrors cub.Unit without importing it, so the converter stays testable
// against files with no ConfigHub session.
type UnitData struct {
	Slug string
	Data []byte
}

func (c *converter) convertWorkload(w *workload) scoretypes.Workload {
	out := scoretypes.Workload{
		ApiVersion: ScoreAPIVersion,
		Metadata:   scoretypes.WorkloadMetadata{"name": w.name},
		Containers: scoretypes.WorkloadContainers{},
	}

	// Score has no kind field; score-k8s reads this annotation to render a
	// StatefulSet back as a StatefulSet rather than defaulting to a Deployment.
	if w.kind == "StatefulSet" {
		out.Metadata["annotations"] = map[string]string{WorkloadKindAnnotation: w.kind}
	}

	res := scoretypes.WorkloadResources{}

	for _, k8sc := range w.pod.Containers {
		out.Containers[sanitizeName(k8sc.Name)] = c.convertContainer(w, k8sc, res)
	}
	for _, k8sc := range w.pod.InitContainers {
		c.warnf(w, "init container %q has no Score equivalent and was dropped; express start-up ordering with a container `before` dependency instead", k8sc.Name)
	}

	svc := c.convertService(w)
	if svc != nil {
		out.Service = svc
	}
	c.convertIngresses(w, svc, res)

	if w.replicas != nil && *w.replicas != 1 {
		c.warnf(w, "replicas=%d is not part of the Score spec and was dropped; set it in the platform's provisioning layer", *w.replicas)
	}

	if len(res) > 0 {
		out.Resources = res
	}
	return out
}

func (c *converter) warnf(w *workload, format string, args ...any) {
	c.warnings = append(c.warnings, Warning{
		Workload: w.name,
		Unit:     w.src.UnitSlug,
		Message:  fmt.Sprintf(format, args...),
	})
}
