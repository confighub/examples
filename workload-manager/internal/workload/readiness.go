// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package workload

import (
	"fmt"
	"sort"
)

// Status is a per-dimension verdict, worst-of over the dimension's checks.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// rank orders statuses so we can take the worst.
func rank(s Status) int {
	switch s {
	case StatusFail:
		return 2
	case StatusWarn:
		return 1
	default:
		return 0
	}
}

func worst(a, b Status) Status {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// Readiness dimensions.
const (
	DimSecurity     = "security"
	DimResources    = "resources"
	DimProbes       = "probes"
	DimHygiene      = "hygiene"
	DimAvailability = "availability"
)

// AllDimensions is the scorecard's dimension set, in display order.
var AllDimensions = []string{DimSecurity, DimResources, DimProbes, DimHygiene, DimAvailability}

// Issue is one scored check within a dimension, carrying its own status so
// findings can map per-issue severity (a dimension can hold both warns and fails).
type Issue struct {
	Status  Status `json:"status"`
	Message string `json:"message"`
}

// DimensionResult is a workload's verdict on one dimension.
type DimensionResult struct {
	Dimension string  `json:"dimension"`
	Status    Status  `json:"status"`
	Issues    []Issue `json:"issues,omitempty"`
}

// add records an issue and bumps the dimension status to the worst seen.
func (d *DimensionResult) add(st Status, msg string) {
	d.Issues = append(d.Issues, Issue{Status: st, Message: msg})
	d.Status = worst(d.Status, st)
}

// WorkloadScore is the per-workload production-readiness scorecard.
type WorkloadScore struct {
	Cluster    string            `json:"cluster"`
	Namespace  string            `json:"namespace,omitempty"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Space      string            `json:"space"`
	UnitSlug   string            `json:"unitSlug"`
	Replicas   *int64            `json:"replicas,omitempty"`
	Overall    Status            `json:"overall"`
	Dimensions []DimensionResult `json:"dimensions"`
}

// dimensionSet turns a possibly-empty selection into a lookup; empty means all.
func dimensionSet(dims []string) map[string]bool {
	if len(dims) == 0 {
		dims = AllDimensions
	}
	set := make(map[string]bool, len(dims))
	for _, d := range dims {
		set[d] = true
	}
	return set
}

// ScoreWorkload computes the readiness scorecard for one workload, restricted to
// the requested dimensions (empty = all). pdbs are the workload's cluster's
// PodDisruptionBudgets, needed for the availability (PDB coverage) dimension;
// nil is fine when availability is not requested.
func ScoreWorkload(w *WorkloadEntity, pdbs []*PDBEntity, dims []string) WorkloadScore {
	want := dimensionSet(dims)
	score := WorkloadScore{
		Cluster:   w.Origin.Cluster,
		Namespace: w.Namespace,
		Kind:      w.Kind,
		Name:      w.Name,
		Space:     w.Origin.Space,
		UnitSlug:  w.Origin.UnitSlug,
		Replicas:  w.Replicas,
		Overall:   StatusPass,
	}
	add := func(d DimensionResult) {
		score.Dimensions = append(score.Dimensions, d)
		score.Overall = worst(score.Overall, d.Status)
	}
	if want[DimSecurity] {
		add(scoreSecurity(w))
	}
	if want[DimResources] {
		add(scoreResources(w))
	}
	if want[DimProbes] {
		add(scoreProbes(w))
	}
	if want[DimHygiene] {
		add(scoreHygiene(w))
	}
	if want[DimAvailability] {
		add(scoreAvailability(w, pdbs))
	}
	return score
}

// ScoreFleet scores every workload across all clusters, sorted by
// (cluster, namespace, kind, name). Each workload's availability is scored
// against its own cluster's PDBs.
func ScoreFleet(clusters map[string]*ClusterWorkloads, dims []string) []WorkloadScore {
	var out []WorkloadScore
	for _, c := range clusters {
		for _, w := range c.Workloads {
			out = append(out, ScoreWorkload(w, c.PDBs, dims))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// scoreSecurity checks the pod/container security context: runAsNonRoot,
// privileged, allowPrivilegeEscalation, readOnlyRootFilesystem, capabilities drop
// ALL, seccomp, and automountServiceAccountToken.
func scoreSecurity(w *WorkloadEntity) DimensionResult {
	d := DimensionResult{Dimension: DimSecurity, Status: StatusPass}
	for _, c := range w.Containers {
		who := containerLabel(c.Name)
		if isTrue(c.Privileged) {
			d.add(StatusFail, who+"runs privileged")
		}
		if !isFalse(c.AllowPrivilegeEscalation) {
			d.add(StatusFail, who+"allowPrivilegeEscalation is not false")
		}
		if !isTrue(c.RunAsNonRoot) {
			d.add(StatusFail, who+"does not set runAsNonRoot: true")
		}
		if !isTrue(c.ReadOnlyRootFilesystem) {
			d.add(StatusWarn, who+"readOnlyRootFilesystem is not true")
		}
		if !c.DropsAll {
			d.add(StatusWarn, who+"does not drop ALL capabilities")
		}
		if !isRuntimeSeccomp(c.SeccompProfileType) {
			d.add(StatusWarn, who+"seccompProfile is not RuntimeDefault")
		}
	}
	// Pod-level automount (best-effort; SA-level setting is not visible here).
	if !isFalse(w.AutomountServiceAccountToken) {
		d.add(StatusWarn, "automountServiceAccountToken is not false")
	}
	return d
}

// scoreResources checks that every container sets cpu+memory requests and limits.
// Missing limits fail (the flagship); missing requests warn.
func scoreResources(w *WorkloadEntity) DimensionResult {
	d := DimensionResult{Dimension: DimResources, Status: StatusPass}
	for _, c := range w.Containers {
		who := containerLabel(c.Name)
		if !c.HasMemoryLimit {
			d.add(StatusFail, who+"no memory limit")
		}
		if !c.HasCPULimit {
			// CPU limits are contentious (throttling); flag as a warning, not a fail.
			d.add(StatusWarn, who+"no cpu limit")
		}
		if !c.HasMemoryRequest {
			d.add(StatusWarn, who+"no memory request")
		}
		if !c.HasCPURequest {
			d.add(StatusWarn, who+"no cpu request")
		}
	}
	return d
}

// scoreProbes checks liveness/readiness probes on long-running controllers. Job,
// CronJob, and bare Pod are skipped (probes don't apply the same way).
func scoreProbes(w *WorkloadEntity) DimensionResult {
	d := DimensionResult{Dimension: DimProbes, Status: StatusPass}
	switch w.Kind {
	case "Job", "CronJob", "Pod":
		return d // not applicable
	}
	for _, c := range w.Containers {
		who := containerLabel(c.Name)
		if !c.HasReadiness {
			d.add(StatusFail, who+"no readiness probe")
		}
		if !c.HasLiveness {
			d.add(StatusWarn, who+"no liveness probe")
		}
	}
	return d
}

// scoreHygiene checks operational hygiene: terminationMessagePolicy should be
// FallbackToLogsOnError so a crash surfaces its last log lines.
func scoreHygiene(w *WorkloadEntity) DimensionResult {
	d := DimensionResult{Dimension: DimHygiene, Status: StatusPass}
	for _, c := range w.Containers {
		who := containerLabel(c.Name)
		if c.TerminationMessagePolicy != "FallbackToLogsOnError" {
			d.add(StatusWarn, who+"terminationMessagePolicy is not FallbackToLogsOnError")
		}
	}
	return d
}

// scoreAvailability checks disruption survival for multi-replica workloads: a
// matching PDB (the cross-Unit selector join), that the PDB doesn't block all
// evictions, and that pod anti-affinity or topology spread is present. Workloads
// that don't want availability (single-replica, DaemonSet, Job, CronJob, Pod)
// pass by default (not applicable).
func scoreAvailability(w *WorkloadEntity, pdbs []*PDBEntity) DimensionResult {
	d := DimensionResult{Dimension: DimAvailability, Status: StatusPass}
	if !w.WantsAvailability() {
		return d // not applicable
	}
	eval := evaluateAvailability(w, pdbs)
	if len(eval.MatchedPDBs) == 0 {
		d.add(StatusFail, "multi-replica workload has no matching PodDisruptionBudget")
	} else if eval.BlocksAllEvictions {
		d.add(StatusFail, fmt.Sprintf("PDB %q blocks all voluntary evictions (minAvailable >= replicas or maxUnavailable: 0)", eval.BlockingPDB))
	}
	if !eval.HasSpread {
		d.add(StatusWarn, "no pod anti-affinity or topology spread (a node/zone loss can take all replicas)")
	}
	return d
}

func containerLabel(name string) string {
	if name == "" {
		return "a container "
	}
	return fmt.Sprintf("container %q ", name)
}

func isTrue(b *bool) bool  { return b != nil && *b }
func isFalse(b *bool) bool { return b != nil && !*b }

func isRuntimeSeccomp(t string) bool {
	return t == "RuntimeDefault" || t == "Localhost"
}
