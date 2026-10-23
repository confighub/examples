// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import "testing"

// podRes builds a workload or Namespace FleetResource in one Unit, keyed the way
// ConfigHub names resources (namespace/name), so it can be matched to results.
func podRes(cluster, unitID, resourceName string, doc map[string]any) FleetResource {
	return FleetResource{
		Origin: ResourceOrigin{
			Cluster: cluster, Target: cluster, Space: cluster + "-space", SpaceID: "space-" + cluster,
			UnitID: unitID, UnitSlug: unitID, ResourceName: resourceName,
		},
		Doc: doc,
	}
}

func findPodSecurity(ps []NamespacePodSecurity, cluster, ns string) *NamespacePodSecurity {
	for i := range ps {
		if ps[i].Cluster == cluster && ps[i].Namespace == ns {
			return &ps[i]
		}
	}
	return nil
}

func TestAnalyzePodSecurity(t *testing.T) {
	enforce := func(level string) map[string]any { return map[string]any{PodSecurityEnforceLabel: level} }
	resources := []FleetResource{
		// web: one workload passes restricted, one only baseline; labeled baseline.
		podRes("prod", "web-ns", "/web", nsDoc("web", enforce(LevelBaseline))),
		podRes("prod", "web-api", "web/api", workloadDoc("Deployment", "api", "web")),
		podRes("prod", "web-worker", "web/worker", workloadDoc("Deployment", "worker", "web")),
		// agent: a DaemonSet fails baseline, but the label enforces restricted.
		podRes("prod", "agent-ns", "/agent", nsDoc("agent", enforce(LevelRestricted))),
		podRes("prod", "agent", "agent/node-agent", workloadDoc("DaemonSet", "node-agent", "agent")),
		// clean: everything passes restricted, labeled baseline.
		podRes("prod", "clean-ns", "/clean", nsDoc("clean", enforce(LevelBaseline))),
		podRes("prod", "clean", "clean/app", workloadDoc("Deployment", "app", "clean")),
		// bare: no Namespace Unit at all.
		podRes("prod", "bare", "bare/job", workloadDoc("Job", "job", "bare")),
		// strict: restricted, labeled restricted.
		podRes("prod", "strict-ns", "/strict", nsDoc("strict", enforce(LevelRestricted))),
		podRes("prod", "strict", "strict/app", workloadDoc("Deployment", "app", "strict")),
		// An unrendered base is not a namespace.
		podRes("prod", "base", "confighubplaceholder/app", workloadDoc("Deployment", "app", PlaceholderNamespace)),
		// A Namespace with no workloads has no posture.
		podRes("prod", "empty-ns", "/empty", nsDoc("empty", nil)),
	}
	escalation := []PodSecurityCheck{{Reason: "allowPrivilegeEscalation != false", Message: "violates PodSecurity \"restricted:latest\": allowPrivilegeEscalation != false"}}
	privileged := []PodSecurityCheck{{Reason: "privileged", Message: "violates PodSecurity \"baseline:latest\": privileged"}}
	results := PodSecurityResults{
		Baseline: map[WorkloadKey][]PodSecurityCheck{
			{UnitID: "agent", ResourceName: "agent/node-agent"}: privileged,
		},
		Restricted: map[WorkloadKey][]PodSecurityCheck{
			{UnitID: "web-worker", ResourceName: "web/worker"}:  escalation,
			{UnitID: "agent", ResourceName: "agent/node-agent"}: append(privileged, escalation...),
			{UnitID: "bare", ResourceName: "bare/job"}:          escalation,
		},
	}
	got := AnalyzePodSecurity(BuildFleet(resources), results)

	for _, tc := range []struct {
		ns, achievable, status, blockedLevel string
		blockers                             []string
	}{
		{"web", LevelBaseline, PodSecurityAtLevel, LevelRestricted, []string{"web-worker"}},
		{"agent", LevelPrivileged, PodSecurityViolating, LevelBaseline, []string{"agent"}},
		{"clean", LevelRestricted, PodSecurityTightenable, "", nil},
		{"bare", LevelBaseline, PodSecurityUnlabeled, LevelRestricted, []string{"bare"}},
		{"strict", LevelRestricted, PodSecurityAtLevel, "", nil},
	} {
		p := findPodSecurity(got, "prod", tc.ns)
		if p == nil {
			t.Errorf("%s: no posture reported", tc.ns)
			continue
		}
		if p.Achievable != tc.achievable || p.Status != tc.status || p.BlockedLevel != tc.blockedLevel {
			t.Errorf("%s: achievable=%s status=%s blocked=%s, want %s %s %s",
				tc.ns, p.Achievable, p.Status, p.BlockedLevel, tc.achievable, tc.status, tc.blockedLevel)
		}
		var units []string
		for _, b := range p.Blockers {
			units = append(units, b.UnitSlug)
		}
		if len(units) != len(tc.blockers) || (len(units) > 0 && units[0] != tc.blockers[0]) {
			t.Errorf("%s: blockers %v, want %v", tc.ns, units, tc.blockers)
		}
	}

	// The blockers of a privileged namespace are what it fails at baseline, not
	// everything it fails at restricted.
	agent := findPodSecurity(got, "prod", "agent")
	if agent == nil || len(agent.Blockers) != 1 {
		t.Fatalf("agent = %+v, want one blocker", agent)
	}
	if len(agent.Blockers[0].Checks) != 1 || agent.Blockers[0].Checks[0].Reason != "privileged" {
		t.Errorf("agent blocker checks = %+v, want only the baseline failure", agent.Blockers[0].Checks)
	}
	if b := agent.Blockers[0]; b.Kind != "DaemonSet" || b.Name != "node-agent" || b.Space != "prod-space" {
		t.Errorf("agent blocker = %+v, want DaemonSet node-agent in prod-space", b)
	}

	for _, ns := range []string{PlaceholderNamespace, "empty"} {
		if findPodSecurity(got, "prod", ns) != nil {
			t.Errorf("%s should have no posture", ns)
		}
	}
	if len(got) != 5 {
		t.Errorf("got %d namespaces, want 5", len(got))
	}
}

// A workload whose Unit could not be checked is counted, not taken to pass or
// to block.
func TestAnalyzePodSecurityUnchecked(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		podRes("prod", "ok", "web/api", workloadDoc("Deployment", "api", "web")),
		podRes("prod", "broken", "web/worker", workloadDoc("Deployment", "worker", "web")),
	})
	got := AnalyzePodSecurity(clusters, PodSecurityResults{Unchecked: map[string]string{"broken": "could not read the pod spec"}})
	p := findPodSecurity(got, "prod", "web")
	if p == nil || p.Unchecked != 1 || p.Workloads != 2 || p.Achievable != LevelRestricted || len(p.Blockers) != 0 {
		t.Errorf("web = %+v, want 2 workloads, 1 unchecked, restricted among the rest", p)
	}
}

// Only Units holding workloads in a real namespace are worth checking.
func TestPodSecurityUnitIDs(t *testing.T) {
	clusters := BuildFleet([]FleetResource{
		podRes("prod", "u2", "web/api", workloadDoc("Deployment", "api", "web")),
		podRes("prod", "u1", "web/worker", workloadDoc("Deployment", "worker", "web")),
		podRes("prod", "u1", "web/cron", workloadDoc("CronJob", "cron", "web")),
		podRes("prod", "ns", "/web", nsDoc("web", nil)),
		podRes("prod", "base", "confighubplaceholder/app", workloadDoc("Deployment", "app", PlaceholderNamespace)),
	})
	got := PodSecurityUnitIDs(clusters)
	if len(got) != 2 || got[0] != "u1" || got[1] != "u2" {
		t.Errorf("PodSecurityUnitIDs = %v, want [u1 u2]", got)
	}
}
