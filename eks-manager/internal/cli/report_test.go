// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"testing"

	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

func ptr[T any](v T) *T { return &v }

// fixture builds a small two-cluster fleet: prod on 1.34 with a skewed node
// group, dev on 1.33 pinned under STANDARD support.
func fixture() *snapshot.Snapshot {
	prod := &eks.ClusterSet{
		Cluster: "prod-use1",
		Control: &eks.ClusterEntity{
			Name: "prod-use1", Version: "1.34", Region: "us-east-1",
			UpgradeSupportType: "EXTENDED",
			AutoMode: eks.AutoMode{
				ComputeEnabled: ptr(true), LoadBalancingEnabled: ptr(true), BlockStorageEnabled: ptr(true),
			},
			Origin: eks.ResourceOrigin{
				Cluster: "prod-use1", Space: "eks-prod-use1", UnitSlug: "cluster",
				SpaceLabels: map[string]string{"Environment": "prod", "Cluster": "prod-use1"},
			},
		},
		NodeGroups: []*eks.NodeGroupEntity{
			{Name: "system", Version: "1.34", Origin: eks.ResourceOrigin{Cluster: "prod-use1", UnitSlug: "ng-system"}},
			{Name: "batch", Version: "1.31", Origin: eks.ResourceOrigin{Cluster: "prod-use1", UnitSlug: "ng-batch"}},
			{Name: "float", Version: "", Origin: eks.ResourceOrigin{Cluster: "prod-use1", UnitSlug: "ng-float"}},
		},
		Addons: []*eks.AddonEntity{
			{Name: "cni", AddonName: "vpc-cni", AddonVersion: "v1.19.5-eksbuild.1"},
			{Name: "dns", AddonName: "coredns", AddonVersion: "v1.11.1-eksbuild.1"},
		},
		Network: []*eks.ResourceEntity{{Kind: "VPC", Name: "vpc"}, {Kind: "Subnet", Name: "sn-a"}},
		IAM:     []*eks.ResourceEntity{{Kind: "Role", Name: "cluster-role"}},
	}
	dev := &eks.ClusterSet{
		Cluster: "dev-use1",
		Control: &eks.ClusterEntity{
			Name: "dev-use1", Version: "1.33", Region: "us-east-1",
			UpgradeSupportType: "STANDARD",
			Origin: eks.ResourceOrigin{
				Cluster: "dev-use1", Space: "eks-dev-use1", UnitSlug: "cluster",
				SpaceLabels: map[string]string{"Environment": "dev", "Cluster": "dev-use1"},
			},
		},
	}
	// A Space with supporting resources but no control plane.
	shared := &eks.ClusterSet{
		Cluster: "shared-net",
		Network: []*eks.ResourceEntity{{Kind: "VPC", Name: "shared-vpc"}},
	}

	return &snapshot.Snapshot{
		Clusters: map[string]*eks.ClusterSet{"prod-use1": prod, "dev-use1": dev, "shared-net": shared},
		Units: map[string]snapshot.UnitMeta{
			"u1": {UnitID: "u1", Slug: "cluster", SpaceSlug: "eks-prod-use1",
				SpaceLabels: map[string]string{"Cluster": "prod-use1"},
				GateCount:   1, HeadRevisionNum: 5, LiveRevisionNum: 4},
			"u2": {UnitID: "u2", Slug: "ng-system", SpaceSlug: "eks-prod-use1",
				SpaceLabels:     map[string]string{"Cluster": "prod-use1"},
				HeadRevisionNum: 2, LiveRevisionNum: 2},
			"u3": {UnitID: "u3", Slug: "cluster", SpaceSlug: "eks-dev-use1",
				SpaceLabels:     map[string]string{"Cluster": "dev-use1"},
				HeadRevisionNum: 1, LiveRevisionNum: 0},
			// Out of scope: its Space is not among snap.Clusters.
			"u9": {UnitID: "u9", Slug: "other", SpaceSlug: "unrelated"},
		},
		Filter: "Space.Labels.Provider = 'aws'",
	}
}

func TestBuildVersionsReport(t *testing.T) {
	r := buildVersionsReport(fixture())

	// shared-net has no control plane, so it is not a versioned cluster.
	if r.Totals.Clusters != 2 {
		t.Fatalf("clusters = %d, want 2 (the control-plane-less Space is excluded)", r.Totals.Clusters)
	}
	if r.Clusters[0].Cluster != "dev-use1" || r.Clusters[1].Cluster != "prod-use1" {
		t.Errorf("clusters not sorted by name: %+v", r.Clusters)
	}

	dev, prod := r.Clusters[0], r.Clusters[1]

	if prod.Environment != "prod" {
		t.Errorf("environment = %q, want it read from the Space labels", prod.Environment)
	}
	if !prod.AutoMode {
		t.Errorf("prod AutoMode = false, want true")
	}
	// batch is 1.31 against a 1.34 control plane.
	if prod.MaxSkew != 3 {
		t.Errorf("prod MaxSkew = %d, want 3", prod.MaxSkew)
	}
	byName := map[string]nodeGroupVersion{}
	for _, n := range prod.NodeGroups {
		byName[n.Name] = n
	}
	if byName["system"].Skew != 0 {
		t.Errorf("system skew = %d, want 0", byName["system"].Skew)
	}
	if byName["batch"].Skew != 3 {
		t.Errorf("batch skew = %d, want 3", byName["batch"].Skew)
	}
	if !byName["float"].Unpinned || byName["float"].Skew != 0 {
		t.Errorf("float = %+v, want unpinned with no skew", byName["float"])
	}
	if len(prod.Addons) != 2 || prod.Addons["vpc-cni"] != "v1.19.5-eksbuild.1" {
		t.Errorf("addons = %v", prod.Addons)
	}
	if prod.SupportTypeRisk {
		t.Errorf("prod flagged as support-type risk, but it is EXTENDED")
	}

	// A pinned version under STANDARD support is the drift trap.
	if !dev.SupportTypeRisk {
		t.Errorf("dev not flagged: pinned 1.33 under STANDARD support")
	}
	if dev.MaxSkew != 0 {
		t.Errorf("dev MaxSkew = %d, want 0 (no node groups)", dev.MaxSkew)
	}

	if r.Totals.DistinctVersions != 2 {
		t.Errorf("distinctVersions = %d, want 2", r.Totals.DistinctVersions)
	}
	if r.VersionCounts["1.34"] != 1 || r.VersionCounts["1.33"] != 1 {
		t.Errorf("versionCounts = %v", r.VersionCounts)
	}
	if r.Totals.Skewed != 1 {
		t.Errorf("skewed = %d, want 1", r.Totals.Skewed)
	}
	if r.Totals.SupportTypeRisk != 1 {
		t.Errorf("supportTypeRisk = %d, want 1", r.Totals.SupportTypeRisk)
	}
	if r.Filter == "" {
		t.Errorf("filter not echoed back")
	}
}

func TestBuildSnapshotReport(t *testing.T) {
	r := buildSnapshotReport(fixture())

	if r.Totals.Clusters != 3 {
		t.Fatalf("clusters = %d, want 3 (including the control-plane-less Space)", r.Totals.Clusters)
	}
	byName := map[string]clusterSummary{}
	for _, c := range r.Clusters {
		byName[c.Cluster] = c
	}

	prod := byName["prod-use1"]
	if prod.Version != "1.34" || prod.Region != "us-east-1" || !prod.AutoMode {
		t.Errorf("prod summary = %+v", prod)
	}
	if prod.NodeGroups != 3 || prod.Addons != 2 || prod.Network != 2 || prod.IAM != 1 {
		t.Errorf("prod counts = %+v", prod)
	}
	// Two Units live in prod's Space; one is gated, one is unapplied (head 5 > live 4).
	if prod.Units != 2 || prod.GatedUnits != 1 || prod.UnappliedUnits != 1 {
		t.Errorf("prod units=%d gated=%d unapplied=%d", prod.Units, prod.GatedUnits, prod.UnappliedUnits)
	}

	// A never-applied Unit (live 0) counts as unapplied.
	dev := byName["dev-use1"]
	if dev.Units != 1 || dev.UnappliedUnits != 1 {
		t.Errorf("dev units=%d unapplied=%d", dev.Units, dev.UnappliedUnits)
	}

	shared := byName["shared-net"]
	if !shared.NoControlPlane {
		t.Errorf("shared-net should be flagged as having no control plane")
	}
	if shared.Version != "" {
		t.Errorf("shared-net version = %q, want empty", shared.Version)
	}

	// The out-of-scope Unit must not be tallied anywhere.
	if r.Totals.Units != 3 {
		t.Errorf("total units = %d, want 3 (the unrelated Space is excluded)", r.Totals.Units)
	}
	if r.Totals.NodeGroups != 3 || r.Totals.Addons != 2 {
		t.Errorf("totals = %+v", r.Totals)
	}
}

func TestBuildResourceRows_Filters(t *testing.T) {
	mk := func(cluster, apiVersion, kind, name string, canonical bool) eks.FleetResource {
		var doc any
		body := `{"apiVersion":"` + apiVersion + `","kind":"` + kind + `","metadata":{"name":"` + name + `"}}`
		_ = json.Unmarshal([]byte(body), &doc)
		return eks.FleetResource{
			Origin: eks.ResourceOrigin{Cluster: cluster, Space: "s", UnitSlug: name, Canonical: canonical},
			Doc:    doc,
		}
	}
	snap := &snapshot.Snapshot{Resources: []eks.FleetResource{
		mk("prod", "eks.aws.upbound.io/v1beta2", "Cluster", "prod", false),
		mk("prod", "eks.aws.upbound.io/v1beta2", "NodeGroup", "ng1", false),
		mk("prod", "ec2.aws.upbound.io/v1beta1", "VPC", "vpc", false),
		mk("prod", "iam.aws.upbound.io/v1beta1", "Role", "role", false),
		mk("dev", "eks.aws.upbound.io/v1beta2", "Cluster", "dev", false),
		mk("base", "eks.aws.upbound.io/v1beta2", "Cluster", "tmpl", true),
		// No group: skipped entirely.
		mk("prod", "v1", "ConfigMap", "cm", false),
	}}

	all := buildResourceRows(snap, "", "", "")
	if len(all) != 6 {
		t.Fatalf("unfiltered rows = %d, want 6 (the group-less resource is dropped)", len(all))
	}
	// Sorted by cluster, then group, then kind, then name.
	if all[0].Cluster != "base" || all[1].Cluster != "dev" {
		t.Errorf("rows not sorted by cluster: %+v", all[:2])
	}
	if !all[0].Canonical {
		t.Errorf("canonical flag lost")
	}

	if got := buildResourceRows(snap, "NodeGroup", "", ""); len(got) != 1 || got[0].Name != "ng1" {
		t.Errorf("--kind NodeGroup = %+v", got)
	}
	// Kind matching is case-insensitive.
	if got := buildResourceRows(snap, "nodegroup", "", ""); len(got) != 1 {
		t.Errorf("--kind is case-sensitive, want insensitive")
	}
	// --group accepts a short prefix.
	if got := buildResourceRows(snap, "", "eks", ""); len(got) != 4 {
		t.Errorf("--group eks = %d rows, want 4", len(got))
	}
	if got := buildResourceRows(snap, "", "ec2.aws.upbound.io", ""); len(got) != 1 {
		t.Errorf("--group full = %d rows, want 1", len(got))
	}
	if got := buildResourceRows(snap, "", "", "prod"); len(got) != 4 {
		t.Errorf("--cluster-name prod = %d rows, want 4", len(got))
	}
	if got := buildResourceRows(snap, "Cluster", "", "dev"); len(got) != 1 || got[0].Name != "dev" {
		t.Errorf("combined filters = %+v", got)
	}
}

func TestFilterFlagsPredicate(t *testing.T) {
	// Empty means "the whole fleet the user can view".
	if got := (filterFlags{}).predicate(); got != "" {
		t.Errorf("empty predicate = %q, want empty", got)
	}
	f := filterFlags{cluster: "prod-use1", environment: "prod"}
	want := "Space.Labels.Cluster = 'prod-use1' AND Space.Labels.Environment = 'prod'"
	if got := f.predicate(); got != want {
		t.Errorf("predicate = %q, want %q", got, want)
	}
	// A raw --where leads, and the shorthands AND onto it.
	f = filterFlags{where: "Slug LIKE 'eks-%'", region: "us-east-1"}
	want = "Slug LIKE 'eks-%' AND Space.Labels.Region = 'us-east-1'"
	if got := f.predicate(); got != want {
		t.Errorf("predicate = %q, want %q", got, want)
	}
	// Single quotes are doubled, so a value cannot break out of the literal.
	f = filterFlags{cluster: "it's"}
	if got := f.predicate(); got != "Space.Labels.Cluster = 'it''s'" {
		t.Errorf("quote escaping = %q", got)
	}
}
