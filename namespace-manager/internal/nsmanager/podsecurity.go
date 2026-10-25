// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"sort"
)

// Pod Security Standard levels, from least to most restrictive
// (https://kubernetes.io/docs/concepts/security/pod-security-standards/).
const (
	LevelPrivileged = "privileged"
	LevelBaseline   = "baseline"
	LevelRestricted = "restricted"
)

// podSecurityRank orders the levels; an unknown or empty label ranks as
// privileged, which is what admission enforces without one.
var podSecurityRank = map[string]int{LevelPrivileged: 0, LevelBaseline: 1, LevelRestricted: 2}

// WorkloadKey identifies one workload resource in one Unit, the way ConfigHub
// names it: the Unit's ID and the resource's scoped name (namespace/name).
type WorkloadKey struct {
	UnitID       string
	ResourceName string
}

// PodSecurityCheck is what vet-pod-security-standard found wrong with one
// workload at one level: each failed check's reason and full message.
type PodSecurityCheck struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// PodSecurityResults holds the workloads that fail each level, as reported by
// vet-pod-security-standard. A workload absent from a level's map passes it,
// unless its Unit is in Unchecked: the function could not check that Unit, for
// the reason given.
type PodSecurityResults struct {
	Baseline   map[WorkloadKey][]PodSecurityCheck
	Restricted map[WorkloadKey][]PodSecurityCheck
	Unchecked  map[string]string
}

// PodSecurityBlocker is a workload that keeps its namespace from the next more
// restrictive level, and the checks it fails there.
type PodSecurityBlocker struct {
	Space    string             `json:"space"`
	SpaceID  string             `json:"spaceId"`
	UnitSlug string             `json:"unitSlug"`
	Kind     string             `json:"kind"`
	Name     string             `json:"name"`
	Checks   []PodSecurityCheck `json:"checks"`
}

// Pod-security statuses, comparing a namespace's enforce label with the level
// its workloads meet.
const (
	// PodSecurityViolating: the label enforces a level some workload fails, so
	// admission rejects that workload's pods.
	PodSecurityViolating = "violating"
	// PodSecurityUnlabeled: the Namespace has no enforce label, or there is no
	// Namespace Unit to carry one.
	PodSecurityUnlabeled = "unlabeled"
	// PodSecurityTightenable: the workloads meet a stricter level than the label
	// enforces.
	PodSecurityTightenable = "tightenable"
	// PodSecurityAtLevel: the label enforces exactly the level the workloads meet.
	PodSecurityAtLevel = "at-level"
)

// NamespacePodSecurity is one namespace's posture: the level its label
// enforces, the most restrictive level all its workloads meet, and the
// workloads that keep it from the next one.
type NamespacePodSecurity struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	// Enforce is the Namespace's pod-security.kubernetes.io/enforce label, or ""
	// when there is none.
	Enforce string `json:"enforce,omitempty"`
	// Achievable is the most restrictive level every workload in the namespace
	// meets.
	Achievable string `json:"achievable"`
	Status     string `json:"status"`
	Workloads  int    `json:"workloads"`
	// Unchecked counts the workloads whose Unit could not be checked. Achievable
	// covers only the others, so with any unchecked it is an upper bound.
	Unchecked int `json:"unchecked,omitempty"`
	// Unlabeled counts, when the namespace enforces a level, the workloads that do
	// not carry that level in their own enforce label. vet-pod-security-standard
	// checks a workload at its own label, so these are the ones it does not hold
	// to the namespace's level; apply-envelope labels them.
	Unlabeled int `json:"unlabeled,omitempty"`
	// BlockedLevel is the level one step stricter than Achievable, which the
	// Blockers fail; empty when Achievable is restricted.
	BlockedLevel string               `json:"blockedLevel,omitempty"`
	Blockers     []PodSecurityBlocker `json:"blockers,omitempty"`
}

// AnalyzePodSecurity computes each occupied namespace's pod-security posture
// from the workloads in clusters and the checks vet-pod-security-standard
// reported for them. Namespaces with no workloads have no posture to report,
// and the placeholder namespace of an unrendered base is skipped.
func AnalyzePodSecurity(clusters map[string]*ClusterNamespaces, results PodSecurityResults) []NamespacePodSecurity {
	var out []NamespacePodSecurity
	for _, c := range clusters {
		enforce := map[string]string{}
		for _, n := range c.Namespaces {
			if lvl := n.PodSecurityEnforce(); lvl != "" {
				enforce[n.Name] = lvl
			}
		}
		byNamespace := map[string][]*WorkloadEntity{}
		for _, w := range c.Workloads {
			if w.Namespace == "" || w.Namespace == PlaceholderNamespace {
				continue
			}
			byNamespace[w.Namespace] = append(byNamespace[w.Namespace], w)
		}
		for ns, workloads := range byNamespace {
			p := NamespacePodSecurity{Cluster: c.Cluster, Namespace: ns, Enforce: enforce[ns], Workloads: len(workloads)}
			for _, w := range workloads {
				if _, ok := results.Unchecked[w.Origin.UnitID]; ok {
					p.Unchecked++
				}
				if p.Enforce != "" && w.PodSecurityEnforce() != p.Enforce {
					p.Unlabeled++
				}
			}
			baselineBlockers := blockersFor(workloads, results.Baseline)
			restrictedBlockers := blockersFor(workloads, results.Restricted)
			switch {
			case len(baselineBlockers) > 0:
				p.Achievable, p.BlockedLevel, p.Blockers = LevelPrivileged, LevelBaseline, baselineBlockers
			case len(restrictedBlockers) > 0:
				p.Achievable, p.BlockedLevel, p.Blockers = LevelBaseline, LevelRestricted, restrictedBlockers
			default:
				p.Achievable = LevelRestricted
			}
			p.Status = podSecurityStatus(p.Enforce, p.Achievable)
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func podSecurityStatus(enforce, achievable string) string {
	if enforce == "" {
		return PodSecurityUnlabeled
	}
	switch enforcing, meets := podSecurityRank[enforce], podSecurityRank[achievable]; {
	case enforcing > meets:
		return PodSecurityViolating
	case enforcing < meets:
		return PodSecurityTightenable
	}
	return PodSecurityAtLevel
}

// blockersFor returns the workloads among ws that fail a level, sorted by
// Space, Unit and resource.
func blockersFor(ws []*WorkloadEntity, failing map[WorkloadKey][]PodSecurityCheck) []PodSecurityBlocker {
	var out []PodSecurityBlocker
	for _, w := range ws {
		checks, ok := failing[WorkloadKey{UnitID: w.Origin.UnitID, ResourceName: w.Origin.ResourceName}]
		if !ok {
			continue
		}
		out = append(out, PodSecurityBlocker{
			Space: w.Origin.Space, SpaceID: w.Origin.SpaceID, UnitSlug: w.Origin.UnitSlug,
			Kind: w.Kind, Name: w.Name, Checks: checks,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Space != b.Space {
			return a.Space < b.Space
		}
		if a.UnitSlug != b.UnitSlug {
			return a.UnitSlug < b.UnitSlug
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return out
}

// PodSecurityUnitIDs returns the IDs of the Units holding workloads that
// AnalyzePodSecurity would consider, sorted: the only Units worth checking.
func PodSecurityUnitIDs(clusters map[string]*ClusterNamespaces) []string {
	seen := map[string]bool{}
	for _, c := range clusters {
		for _, w := range c.Workloads {
			if w.Namespace == "" || w.Namespace == PlaceholderNamespace || w.Origin.UnitID == "" {
				continue
			}
			seen[w.Origin.UnitID] = true
		}
	}
	return sortedKeys(seen)
}
