// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import (
	"strings"
	"testing"
)

func labeledWorkload(kind, name, namespace string, labels map[string]any) map[string]any {
	doc := workloadDoc(kind, name, namespace)
	if labels != nil {
		doc["metadata"].(map[string]any)["labels"] = labels
	}
	return doc
}

func findApply(plans []PodSecurityApply, ns string) *PodSecurityApply {
	for i := range plans {
		if plans[i].Namespace == ns {
			return &plans[i]
		}
	}
	return nil
}

func unitEdits(edits []UnitLabelEdits, unit string) []LabelEdit {
	for _, u := range edits {
		if u.UnitSlug == unit {
			return u.Edits
		}
	}
	return nil
}

func TestPlanPodSecurity(t *testing.T) {
	enforce := func(level string) map[string]any { return map[string]any{PodSecurityEnforceLabel: level} }
	resources := []FleetResource{
		// web: workloads meet restricted, unlabeled -> restricted on both.
		podRes("prod", "web-ns", "/web", nsDoc("web", nil)),
		podRes("prod", "web-api", "web/api", workloadDoc("Deployment", "api", "web")),
		// shop: meets baseline, labeled baseline already, one workload unlabeled.
		podRes("prod", "shop-ns", "/shop", nsDoc("shop", map[string]any{
			PodSecurityEnforceLabel: "baseline", PodSecurityEnforceVersionLabel: "latest", PodSecurityWarnLabel: "restricted",
		})),
		podRes("prod", "shop-web", "shop/web", labeledWorkload("Deployment", "web", "shop", map[string]any{
			PodSecurityEnforceLabel: "baseline", PodSecurityEnforceVersionLabel: "latest",
		})),
		podRes("prod", "shop-cron", "shop/report", workloadDoc("CronJob", "report", "shop")),
		// agent: enforces restricted, but its workload meets only privileged.
		podRes("prod", "agent-ns", "/agent", nsDoc("agent", enforce("restricted"))),
		podRes("prod", "agent", "agent/node", workloadDoc("DaemonSet", "node", "agent")),
		// empty: a Namespace with no workloads.
		podRes("prod", "empty-ns", "/empty", nsDoc("empty", nil)),
		// kube-system is the cluster's.
		podRes("prod", "ks-ns", "/kube-system", nsDoc("kube-system", nil)),
		podRes("prod", "dns", "kube-system/coredns", workloadDoc("Deployment", "coredns", "kube-system")),
		// odd: a workload could not be checked.
		podRes("prod", "odd-ns", "/odd", nsDoc("odd", nil)),
		podRes("prod", "odd", "odd/x", workloadDoc("Deployment", "x", "odd")),
	}
	clusters := BuildFleet(resources)
	results := PodSecurityResults{
		Baseline: map[WorkloadKey][]PodSecurityCheck{
			{UnitID: "agent", ResourceName: "agent/node"}:        {{Reason: "privileged"}},
			{UnitID: "dns", ResourceName: "kube-system/coredns"}: {{Reason: "privileged"}},
		},
		Restricted: map[WorkloadKey][]PodSecurityCheck{
			{UnitID: "shop-web", ResourceName: "shop/web"}: {{Reason: "seccompProfile"}},
			{UnitID: "agent", ResourceName: "agent/node"}:  {{Reason: "privileged"}},
		},
		Unchecked: map[string]string{"odd": "could not read the pod spec"},
	}
	all := func(*NamespaceEntity) bool { return true }
	plans, edits := PlanPodSecurity(clusters, AnalyzePodSecurity(clusters, results), "", "latest", all)

	for _, tc := range []struct{ ns, level, skip string }{
		{"web", LevelRestricted, ""},
		{"shop", LevelBaseline, ""},
		{"agent", LevelRestricted, ""},
		{"empty", "", "no workloads in ConfigHub"},
		{"kube-system", "", "cluster owns"},
		{"odd", "", "could not be checked"},
	} {
		p := findApply(plans, tc.ns)
		if p == nil {
			t.Errorf("%s: no plan", tc.ns)
			continue
		}
		if p.Level != tc.level || (tc.skip == "") != (p.Skip == "") || !strings.Contains(p.Skip, tc.skip) {
			t.Errorf("%s: level=%q skip=%q, want level=%q skip containing %q", tc.ns, p.Level, p.Skip, tc.level, tc.skip)
		}
	}

	// web: restricted on the Namespace (no warn below restricted) and its workload.
	webNS := unitEdits(edits, "web-ns")
	if len(webNS) != 2 || webNS[0].Key != PodSecurityEnforceLabel || webNS[0].Value != LevelRestricted ||
		webNS[0].WhereResource != "kind = 'Namespace' AND metadata.name = 'web'" {
		t.Errorf("web-ns edits = %+v", webNS)
	}
	if api := unitEdits(edits, "web-api"); len(api) != 2 ||
		api[0].WhereResource != "kind = 'Deployment' AND metadata.name = 'api' AND metadata.namespace = 'web'" {
		t.Errorf("web-api edits = %+v", api)
	}
	// shop: Namespace and the labeled workload already there; only the CronJob changes.
	if e := unitEdits(edits, "shop-ns"); len(e) != 0 {
		t.Errorf("shop-ns edits = %+v, want none", e)
	}
	if e := unitEdits(edits, "shop-web"); len(e) != 0 {
		t.Errorf("shop-web edits = %+v, want none", e)
	}
	if e := unitEdits(edits, "shop-cron"); len(e) != 2 || e[0].Value != LevelBaseline {
		t.Errorf("shop-cron edits = %+v", e)
	}
	// agent keeps its stricter restricted label, unversioned, and copies it onto its
	// workload, which the guardrail will then flag.
	if p := findApply(plans, "agent"); p == nil || !p.Kept || !strings.Contains(p.Note, "stricter restricted") {
		t.Errorf("agent = %+v, want kept with a note", p)
	}
	if e := unitEdits(edits, "agent"); len(e) != 1 || e[0].Key != PodSecurityEnforceLabel || e[0].Value != LevelRestricted {
		t.Errorf("agent edits = %+v, want only the enforce label", e)
	}
	for _, unit := range []string{"agent-ns", "empty-ns", "ks-ns", "dns", "odd-ns", "odd"} {
		if e := unitEdits(edits, unit); len(e) != 0 {
			t.Errorf("%s edits = %+v, want none for a skipped namespace", unit, e)
		}
	}
}

func TestPlanPodSecurityRequestedLevel(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		podRes("prod", "web-ns", "/web", nsDoc("web", nil)),
		podRes("prod", "web", "web/web", workloadDoc("Deployment", "web", "web")),
		podRes("prod", "empty-ns", "/empty", nsDoc("empty", nil)),
	})
	results := PodSecurityResults{Restricted: map[WorkloadKey][]PodSecurityCheck{
		{UnitID: "web", ResourceName: "web/web"}: {{Reason: "seccompProfile"}},
	}}
	postures := AnalyzePodSecurity(clusters, results)
	all := func(*NamespaceEntity) bool { return true }

	plans, _ := PlanPodSecurity(clusters, postures, LevelRestricted, "latest", all)
	if p := findApply(plans, "web"); p == nil || p.Level != "" || !strings.Contains(p.Skip, "meet only baseline") {
		t.Errorf("web at restricted = %+v, want refused", p)
	}
	// A level asked for applies to a namespace with no workloads to judge by.
	if p := findApply(plans, "empty"); p == nil || p.Level != LevelRestricted {
		t.Errorf("empty at restricted = %+v", p)
	}

	plans, edits := PlanPodSecurity(clusters, postures, LevelBaseline, "v1.30", all)
	if p := findApply(plans, "web"); p == nil || p.Level != LevelBaseline {
		t.Errorf("web at baseline = %+v", p)
	}
	// Below restricted, the Namespace also warns at restricted, at the same version.
	e := unitEdits(edits, "web-ns")
	got := map[string]string{}
	for _, edit := range e {
		got[edit.Key] = edit.Value
	}
	if got[PodSecurityEnforceLabel] != LevelBaseline || got[PodSecurityEnforceVersionLabel] != "v1.30" ||
		got[PodSecurityWarnLabel] != LevelRestricted || got[PodSecurityWarnVersionLabel] != "v1.30" {
		t.Errorf("web-ns labels = %v", got)
	}

	// Only selected namespaces are planned.
	plans, _ = PlanPodSecurity(clusters, postures, "", "latest", func(n *NamespaceEntity) bool { return n.Name == "web" })
	if len(plans) != 1 || plans[0].Namespace != "web" {
		t.Errorf("selected plans = %+v", plans)
	}
}

// The posture counts the workloads that do not carry their namespace's level.
func TestAnalyzePodSecurityUnlabeled(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		podRes("prod", "ns", "/web", nsDoc("web", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		podRes("prod", "a", "web/a", labeledWorkload("Deployment", "a", "web", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		podRes("prod", "b", "web/b", workloadDoc("Deployment", "b", "web")),
	})
	p := findPodSecurity(AnalyzePodSecurity(clusters, PodSecurityResults{}), "prod", "web")
	if p == nil || p.Unlabeled != 1 {
		t.Errorf("web = %+v, want 1 unlabeled workload", p)
	}
}
