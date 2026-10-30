// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"fmt"
	"sort"

	"github.com/confighub/sdk/configkit/k8skit"
)

// The labels besides enforce that a namespace's Pod Security Admission reads.
const (
	PodSecurityEnforceVersionLabel = "pod-security.kubernetes.io/enforce-version"
	PodSecurityWarnLabel           = "pod-security.kubernetes.io/warn"
	PodSecurityWarnVersionLabel    = "pod-security.kubernetes.io/warn-version"
)

// PodSecurityApply is what applying a Pod Security Standard level does to one
// namespace: the level it will enforce, or why it is left alone.
type PodSecurityApply struct {
	Cluster    string `json:"cluster"`
	Namespace  string `json:"namespace"`
	Workloads  int    `json:"workloads"`
	Enforce    string `json:"enforce,omitempty"`
	Achievable string `json:"achievable,omitempty"`
	// Level is the level the namespace enforces once applied, which its
	// workloads are labeled with; empty when Skip says why nothing is done.
	Level string `json:"level,omitempty"`
	// Kept says the Namespace's own label is left as it is -- it already enforces
	// Level, or a stricter one than was asked for -- and only copied onto the
	// workloads.
	Kept bool   `json:"kept,omitempty"`
	Note string `json:"note,omitempty"`
	Skip string `json:"skip,omitempty"`
}

// LabelEdit sets one label on the resources WhereResource selects in a Unit.
type LabelEdit struct {
	WhereResource string `json:"whereResource"`
	Key           string `json:"key"`
	Value         string `json:"value"`
}

// UnitLabelEdits are the label edits to make in one Unit.
type UnitLabelEdits struct {
	UnitID   string      `json:"unitId"`
	Space    string      `json:"space"`
	UnitSlug string      `json:"unitSlug"`
	Edits    []LabelEdit `json:"edits"`
}

// PlanPodSecurity decides, for each namespace a Namespace Unit in clusters
// defines and selected accepts, the Pod Security Standard level to apply: the
// most restrictive one all its workloads meet, or level when one is asked for,
// at version. The level goes on the Namespace, which is what admission enforces,
// and on each of its workloads, so vet-pod-security-standard can hold them to it
// in ConfigHub. It returns the per-namespace decisions and the label edits that
// carry them out, grouped by Unit; a label already at its value is not edited.
//
// It never lowers a namespace's level. A Namespace that already enforces the
// level, or a stricter one, keeps its labels, version included, and its
// workloads get copies -- which is what lets the guardrail flag the workloads a
// stricter level rejects.
//
// A namespace is left alone, with the reason, when it is one the cluster owns
// (kube-system and the like); when its workloads could not all be checked; when
// it has none in ConfigHub and no level was asked for, since there is nothing to
// judge by; and when its workloads do not meet the level asked for.
func PlanPodSecurity(clusters map[string]*ClusterNamespaces, postures []NamespacePodSecurity, level, version string, selected func(*NamespaceEntity) bool) ([]PodSecurityApply, []UnitLabelEdits) {
	posture := map[[2]string]NamespacePodSecurity{}
	for _, p := range postures {
		posture[[2]string{p.Cluster, p.Namespace}] = p
	}

	type nsKey struct{ cluster, name string }
	objects := map[nsKey][]*NamespaceEntity{}
	workloads := map[nsKey][]*WorkloadEntity{}
	for _, c := range clusters {
		for _, n := range c.Namespaces {
			if !c.skipsNamespace(n.Name) && selected(n) {
				k := nsKey{c.Cluster, n.Name}
				objects[k] = append(objects[k], n)
			}
		}
		for _, w := range c.Workloads {
			k := nsKey{c.Cluster, w.Namespace}
			workloads[k] = append(workloads[k], w)
		}
	}

	editsByUnit := map[string]*UnitLabelEdits{}
	addEdit := func(o ResourceOrigin, whereResource, key, value string) {
		u, ok := editsByUnit[o.UnitID]
		if !ok {
			u = &UnitLabelEdits{UnitID: o.UnitID, Space: o.Space, UnitSlug: o.UnitSlug}
			editsByUnit[o.UnitID] = u
		}
		u.Edits = append(u.Edits, LabelEdit{WhereResource: whereResource, Key: key, Value: value})
	}

	var plans []PodSecurityApply
	for k, nsObjects := range objects {
		p, hasPosture := posture[[2]string{k.cluster, k.name}]
		plan := PodSecurityApply{Cluster: k.cluster, Namespace: k.name, Workloads: len(workloads[k]), Enforce: enforceOf(nsObjects)}
		if hasPosture {
			plan.Achievable = p.Achievable
		}
		target := level
		switch {
		case k8skit.IsBuiltInNamespace(k.name):
			plan.Skip = "the cluster owns this namespace"
		case hasPosture && p.Unchecked > 0:
			plan.Skip = fmt.Sprintf("%d workload(s) could not be checked", p.Unchecked)
		case !hasPosture && level == "":
			plan.Skip = "no workloads in ConfigHub to judge a level by; pass --level to set one"
		case hasPosture && level != "" && podSecurityRank[level] > podSecurityRank[p.Achievable]:
			plan.Skip = fmt.Sprintf("its workloads meet only %s", p.Achievable)
		}
		if plan.Skip != "" {
			plans = append(plans, plan)
			continue
		}
		if target == "" {
			target = p.Achievable
		}

		// The labels the workloads copy: the Namespace's own when it keeps them,
		// else the new level at version.
		labels := map[string]string{PodSecurityEnforceLabel: target, PodSecurityEnforceVersionLabel: version}
		if plan.Enforce != "" && podSecurityRank[plan.Enforce] >= podSecurityRank[target] {
			if plan.Enforce != target {
				plan.Note = fmt.Sprintf("already enforces the stricter %s, which its workloads do not all meet; kept, and copied onto them so the guardrail flags the ones it rejects", plan.Enforce)
			}
			target, plan.Kept = plan.Enforce, true
			labels = map[string]string{PodSecurityEnforceLabel: target, PodSecurityEnforceVersionLabel: versionOf(nsObjects, target)}
		}
		plan.Level = target

		if !plan.Kept {
			for _, n := range nsObjects {
				where := fmt.Sprintf("kind = 'Namespace' AND metadata.name = '%s'", n.Name)
				for _, key := range []string{PodSecurityEnforceLabel, PodSecurityEnforceVersionLabel} {
					if n.Labels[key] != labels[key] {
						addEdit(n.Origin, where, key, labels[key])
					}
				}
				// Warn at restricted below it, so what still stands in the way shows
				// up as warnings, unless someone has chosen a warn level already.
				if target != LevelRestricted && n.Labels[PodSecurityWarnLabel] == "" {
					addEdit(n.Origin, where, PodSecurityWarnLabel, LevelRestricted)
					addEdit(n.Origin, where, PodSecurityWarnVersionLabel, version)
				}
			}
		}
		for _, w := range workloads[k] {
			where := fmt.Sprintf("kind = '%s' AND metadata.name = '%s' AND metadata.namespace = '%s'", w.Kind, w.Name, w.Namespace)
			for _, key := range []string{PodSecurityEnforceLabel, PodSecurityEnforceVersionLabel} {
				if labels[key] != "" && w.Labels[key] != labels[key] {
					addEdit(w.Origin, where, key, labels[key])
				}
			}
		}
		plans = append(plans, plan)
	}

	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Cluster != plans[j].Cluster {
			return plans[i].Cluster < plans[j].Cluster
		}
		return plans[i].Namespace < plans[j].Namespace
	})
	edits := make([]UnitLabelEdits, 0, len(editsByUnit))
	for _, u := range editsByUnit {
		edits = append(edits, *u)
	}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].Space != edits[j].Space {
			return edits[i].Space < edits[j].Space
		}
		return edits[i].UnitSlug < edits[j].UnitSlug
	})
	return plans, edits
}

// versionOf is the enforce-version on the Namespace object that enforces level,
// or "" when it sets none and admission uses the latest.
func versionOf(objects []*NamespaceEntity, level string) string {
	for _, n := range objects {
		if n.PodSecurityEnforce() == level {
			return n.Labels[PodSecurityEnforceVersionLabel]
		}
	}
	return ""
}

// enforceOf is the strictest enforce level among a namespace's Namespace
// objects -- usually one, but a namespace defined twice may disagree with
// itself, and the strictest is the one not to undercut.
func enforceOf(objects []*NamespaceEntity) string {
	enforce := ""
	for _, n := range objects {
		if lvl := n.PodSecurityEnforce(); lvl != "" && (enforce == "" || podSecurityRank[lvl] > podSecurityRank[enforce]) {
			enforce = lvl
		}
	}
	return enforce
}
