// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/confighub/examples/eks-manager/internal/eks"
)

func cluster(version string, ngVersions ...string) *eks.ClusterSet {
	cs := &eks.ClusterSet{
		Cluster: "c",
		Control: &eks.ClusterEntity{
			Name: "c", Version: version, APIVersion: "eks.aws.upbound.io/v1beta2",
			Origin: eks.ResourceOrigin{Space: "eks-c", UnitSlug: "cluster"},
		},
	}
	for i, v := range ngVersions {
		cs.NodeGroups = append(cs.NodeGroups, &eks.NodeGroupEntity{
			Name:    string(rune('a' + i)),
			Version: v,
			Origin:  eks.ResourceOrigin{Space: "eks-c", UnitSlug: "ng-" + string(rune('a'+i))},
		})
	}
	return cs
}

func stagesOf(p upgradePlan, stage string) []upgradeStage {
	var out []upgradeStage
	for _, s := range p.Stages {
		if s.Stage == stage {
			out = append(out, s)
		}
	}
	return out
}

// With every node group level, the control plane goes first.
func TestBuildUpgradePlan_NoCatchupNeeded(t *testing.T) {
	p := buildUpgradePlan(cluster("1.34", "1.34", "1.34"),
		eks.Version{Major: 1, Minor: 34}, eks.Version{Major: 1, Minor: 35}, true)

	if p.Blocked {
		t.Error("plan blocked though no node group is behind")
	}
	if len(stagesOf(p, "nodegroup-catchup")) != 0 {
		t.Error("catch-up stages emitted unnecessarily")
	}
	if p.Stages[0].Stage != "control-plane" {
		t.Errorf("first stage = %q, want control-plane", p.Stages[0].Stage)
	}
	if p.ControlPlaneStage != 1 {
		t.Errorf("ControlPlaneStage = %d, want 1", p.ControlPlaneStage)
	}
	// Both node groups follow, targeting the new version.
	post := stagesOf(p, "nodegroup")
	if len(post) != 2 {
		t.Fatalf("post-upgrade node group stages = %d, want 2", len(post))
	}
	for _, s := range post {
		if s.To != "1.35" {
			t.Errorf("node group target = %q, want 1.35", s.To)
		}
		if s.Order <= p.ControlPlaneStage {
			t.Errorf("node group stage %d runs before the control plane (%d)", s.Order, p.ControlPlaneStage)
		}
	}
}

// A node group behind the CURRENT control plane must be caught up before the
// control plane advances, or it is stranded further behind than EKS allows.
func TestBuildUpgradePlan_CatchupFirst(t *testing.T) {
	p := buildUpgradePlan(cluster("1.35", "1.34", "1.35"),
		eks.Version{Major: 1, Minor: 35}, eks.Version{Major: 1, Minor: 36}, true)

	if !p.Blocked {
		t.Fatal("plan not blocked though a node group is behind the control plane")
	}
	catchup := stagesOf(p, "nodegroup-catchup")
	if len(catchup) != 1 {
		t.Fatalf("catch-up stages = %d, want 1 (only the lagging group)", len(catchup))
	}
	// One minor at a time: 1.34 -> 1.35, never straight to 1.36.
	if catchup[0].To != "1.35" {
		t.Errorf("catch-up target = %q, want 1.35 (one minor at a time)", catchup[0].To)
	}
	// And it must be ordered before the control plane.
	if catchup[0].Order >= p.ControlPlaneStage {
		t.Errorf("catch-up stage %d does not precede the control plane (%d)",
			catchup[0].Order, p.ControlPlaneStage)
	}
	if countCatchup(p) != 1 {
		t.Errorf("countCatchup = %d, want 1", countCatchup(p))
	}
}

// A group several minors behind needs several passes, and the note must say so.
func TestBuildUpgradePlan_MultiPassCatchup(t *testing.T) {
	p := buildUpgradePlan(cluster("1.35", "1.32"),
		eks.Version{Major: 1, Minor: 35}, eks.Version{Major: 1, Minor: 36}, true)

	catchup := stagesOf(p, "nodegroup-catchup")
	if len(catchup) != 1 {
		t.Fatalf("catch-up stages = %d, want 1", len(catchup))
	}
	if catchup[0].To != "1.33" {
		t.Errorf("catch-up target = %q, want 1.33 (one minor from 1.32)", catchup[0].To)
	}
	if !p.Blocked {
		t.Error("plan not blocked")
	}
	// The operator must be told this is not a single step.
	if catchup[0].Note == "" || !contains(catchup[0].Note, "several passes") {
		t.Errorf("note does not warn about multiple passes: %q", catchup[0].Note)
	}
}

// An unpinned node group tracks the control plane, so it is never "behind".
func TestBuildUpgradePlan_UnpinnedNodeGroup(t *testing.T) {
	p := buildUpgradePlan(cluster("1.35", ""),
		eks.Version{Major: 1, Minor: 35}, eks.Version{Major: 1, Minor: 36}, true)

	if p.Blocked {
		t.Error("an unpinned node group blocked the upgrade")
	}
	if len(stagesOf(p, "nodegroup-catchup")) != 0 {
		t.Error("catch-up emitted for an unpinned node group")
	}
	post := stagesOf(p, "nodegroup")
	if len(post) != 1 || post[0].From != "(unpinned)" {
		t.Errorf("unpinned node group stage = %+v", post)
	}
}

// A node group already at or beyond the target needs no stage at all.
func TestBuildUpgradePlan_AlreadyCurrent(t *testing.T) {
	p := buildUpgradePlan(cluster("1.35", "1.36"),
		eks.Version{Major: 1, Minor: 35}, eks.Version{Major: 1, Minor: 36}, true)
	if len(stagesOf(p, "nodegroup")) != 0 {
		t.Error("a node group already at the target got an upgrade stage")
	}
	if p.Blocked {
		t.Error("a node group ahead of the control plane blocked the upgrade")
	}
}

func TestBuildUpgradePlan_AddonsBetween(t *testing.T) {
	cs := cluster("1.34", "1.34")
	cs.Addons = []*eks.AddonEntity{{
		Name: "cni", AddonName: "vpc-cni", AddonVersion: "v1.19.5",
		Origin: eks.ResourceOrigin{Space: "eks-c", UnitSlug: "addon-vpc-cni"},
	}}
	p := buildUpgradePlan(cs, eks.Version{Major: 1, Minor: 34}, eks.Version{Major: 1, Minor: 35}, true)

	addons := stagesOf(p, "addon")
	if len(addons) != 1 {
		t.Fatalf("addon stages = %d, want 1", len(addons))
	}
	ngs := stagesOf(p, "nodegroup")
	// Order matters: control plane, then addons, then node groups.
	if !(p.ControlPlaneStage < addons[0].Order && addons[0].Order < ngs[0].Order) {
		t.Errorf("stage order wrong: control=%d addon=%d nodegroup=%d",
			p.ControlPlaneStage, addons[0].Order, ngs[0].Order)
	}
}

func TestGradeEdits(t *testing.T) {
	const rt = "eks.aws.upbound.io/v1beta2/NodeGroup"

	// A pure scaling change is in-place and must not block.
	r := gradeEdits(rt, "NodeGroup", []pathEdit{
		{Path: "spec.forProvider.scalingConfig.maxSize", Value: "12"},
	})
	if r.Blocks || r.MaxDisruption != eks.DisruptionNone {
		t.Errorf("scaling graded %q blocks=%v; want none/false", r.MaxDisruption, r.Blocks)
	}

	// A version change rolls nodes but still applies.
	r = gradeEdits(rt, "NodeGroup", []pathEdit{
		{Path: "spec.forProvider.version", Value: "1.35"},
	})
	if r.Blocks || r.MaxDisruption != eks.DisruptionRolling {
		t.Errorf("version graded %q blocks=%v; want rolling/false", r.MaxDisruption, r.Blocks)
	}

	// An immutable field must block, with remediation.
	r = gradeEdits(rt, "NodeGroup", []pathEdit{
		{Path: "spec.forProvider.instanceTypes.0", Value: "m6i.xlarge"},
	})
	if !r.Blocks {
		t.Fatal("instanceTypes edit did not block")
	}
	if r.MaxScore != "High" {
		t.Errorf("MaxScore = %q, want High", r.MaxScore)
	}
	if r.Remediation == "" {
		t.Error("no remediation on a blocking edit")
	}

	// The worst edit wins over benign ones in the same batch.
	r = gradeEdits(rt, "NodeGroup", []pathEdit{
		{Path: "spec.forProvider.scalingConfig.maxSize", Value: "12"},
		{Path: "spec.forProvider.amiType", Value: "AL2023_ARM_64_STANDARD"},
	})
	if !r.Blocks || r.MaxDisruption != eks.DisruptionReplace {
		t.Errorf("mixed batch graded %q blocks=%v; want replace/true", r.MaxDisruption, r.Blocks)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
