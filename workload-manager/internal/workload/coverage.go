// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package workload

import (
	"sort"
	"strconv"
	"strings"
)

// Empty reports whether the selector has no terms. An empty PodDisruptionBudget
// selector selects every pod in its namespace (Kubernetes semantics), so an empty
// selector matches all label sets.
func (s LabelSelector) Empty() bool {
	return len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0
}

// Matches reports whether the given pod labels satisfy the selector. This is the
// same matchLabels + matchExpressions semantics the netpol manager uses for its
// coverage join (In / NotIn / Exists / DoesNotExist); kept here so the workload
// engine is self-contained until the shared reduce lands.
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

// matchingPDBs returns the PDBs (from the workload's cluster) in the same
// namespace whose selector matches the workload's pod-template labels.
func matchingPDBs(w *WorkloadEntity, pdbs []*PDBEntity) []*PDBEntity {
	var out []*PDBEntity
	for _, p := range pdbs {
		if p.Namespace != w.Namespace {
			continue
		}
		if p.Selector.Matches(w.PodLabels) {
			out = append(out, p)
		}
	}
	return out
}

// pdbBlocksAllEvictions reports whether a PDB is so tight that no pod may ever be
// voluntarily evicted (node drains hang forever): maxUnavailable of 0, or
// minAvailable that meets or exceeds the replica count (including "100%").
func pdbBlocksAllEvictions(p *PDBEntity, replicas *int64) bool {
	if p.MaxUnavailable == "0" {
		return true
	}
	if p.MinAvailable != "" {
		if strings.TrimSpace(p.MinAvailable) == "100%" {
			return true
		}
		if n, err := strconv.ParseInt(p.MinAvailable, 10, 64); err == nil && replicas != nil && n >= *replicas {
			return true
		}
	}
	return false
}

// WantsAvailability reports whether disruption-survival (a PDB, anti-affinity /
// spread) is meaningful for this workload: a multi-replica controller. A
// single-replica workload gains nothing from a PDB (evicting its one pod is
// downtime regardless), and DaemonSet / Job / CronJob / bare Pod carry no
// replica count, so they are out of scope for v1 availability scoring.
func (w *WorkloadEntity) WantsAvailability() bool {
	return w.MultiReplica()
}

// AvailabilityEval is the disruption-survival verdict for one workload.
type AvailabilityEval struct {
	MatchedPDBs        []*PDBEntity
	BlocksAllEvictions bool
	BlockingPDB        string
	HasSpread          bool
}

// evaluateAvailability computes the PDB coverage + spread state of a workload
// against its cluster's PDBs.
func evaluateAvailability(w *WorkloadEntity, pdbs []*PDBEntity) AvailabilityEval {
	matched := matchingPDBs(w, pdbs)
	eval := AvailabilityEval{
		MatchedPDBs: matched,
		HasSpread:   w.HasAntiAffinity || w.HasTopologySpread,
	}
	for _, p := range matched {
		if pdbBlocksAllEvictions(p, w.Replicas) {
			eval.BlocksAllEvictions = true
			eval.BlockingPDB = p.Name
			break
		}
	}
	return eval
}

// AvailabilityResult is the per-workload availability report row (the
// `availability` command's output).
type AvailabilityResult struct {
	Cluster           string `json:"cluster"`
	Namespace         string `json:"namespace,omitempty"`
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	Space             string `json:"space"`
	SpaceID           string `json:"spaceId,omitempty"`
	UnitSlug          string `json:"unitSlug"`
	Replicas          *int64 `json:"replicas,omitempty"`
	HasPDB            bool   `json:"hasPdb"`
	PDBName           string `json:"pdbName,omitempty"`
	PDBBlocksEviction bool   `json:"pdbBlocksEviction"`
	HasAntiAffinity   bool   `json:"hasAntiAffinity"`
	HasTopologySpread bool   `json:"hasTopologySpread"`
	Issues            []string `json:"issues,omitempty"`
}

// AnalyzeAvailability reports availability for every multi-replica workload in
// the fleet, sorted by (cluster, namespace, kind, name).
func AnalyzeAvailability(clusters map[string]*ClusterWorkloads) []AvailabilityResult {
	var out []AvailabilityResult
	for _, c := range clusters {
		for _, w := range c.Workloads {
			if !w.WantsAvailability() {
				continue
			}
			eval := evaluateAvailability(w, c.PDBs)
			r := AvailabilityResult{
				Cluster:           w.Origin.Cluster,
				Namespace:         w.Namespace,
				Kind:              w.Kind,
				Name:              w.Name,
				Space:             w.Origin.Space,
				SpaceID:           w.Origin.SpaceID,
				UnitSlug:          w.Origin.UnitSlug,
				Replicas:          w.Replicas,
				HasPDB:            len(eval.MatchedPDBs) > 0,
				PDBBlocksEviction: eval.BlocksAllEvictions,
				HasAntiAffinity:   w.HasAntiAffinity,
				HasTopologySpread: w.HasTopologySpread,
			}
			if len(eval.MatchedPDBs) > 0 {
				r.PDBName = eval.MatchedPDBs[0].Name
			}
			if !r.HasPDB {
				r.Issues = append(r.Issues, "no matching PodDisruptionBudget")
			}
			if eval.BlocksAllEvictions {
				r.Issues = append(r.Issues, "PDB "+eval.BlockingPDB+" blocks all voluntary evictions")
			}
			if !eval.HasSpread {
				r.Issues = append(r.Issues, "no pod anti-affinity or topology spread")
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
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}
