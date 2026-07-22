// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"strings"
	"testing"
)

func findingsFor(t *testing.T, cs *ClusterSet) map[string][]Finding {
	t.Helper()
	byAnalyzer := map[string][]Finding{}
	for _, f := range Findings(map[string]*ClusterSet{cs.Cluster: cs}) {
		byAnalyzer[f.Analyzer] = append(byAnalyzer[f.Analyzer], f)
	}
	return byAnalyzer
}

// A clean Auto Mode cluster should produce nothing.
func TestFindings_CleanCluster(t *testing.T) {
	cs := &ClusterSet{
		Cluster: "clean",
		Control: &ClusterEntity{
			Name: "clean", Version: "1.34", UpgradeSupportType: "EXTENDED",
			EncryptionConfigured: true,
			LogTypes:             []string{"api", "audit", "authenticator"},
			DeletionProtection:   ptr(true),
			EndpointPublicAccess: ptr(false),
			AutoMode: AutoMode{
				ComputeEnabled: ptr(true), LoadBalancingEnabled: ptr(true), BlockStorageEnabled: ptr(true),
			},
			BootstrapSelfManagedAddons: ptr(false),
		},
	}
	if got := Findings(map[string]*ClusterSet{"clean": cs}); len(got) != 0 {
		t.Errorf("clean cluster produced %d findings: %+v", len(got), got)
	}
}

func TestFindings_AutoModeInvariant(t *testing.T) {
	// Partially specified: AWS rejects this outright.
	cs := &ClusterSet{Cluster: "c", Control: &ClusterEntity{
		Name: "c", EncryptionConfigured: true, DeletionProtection: ptr(true),
		LogTypes: []string{"api", "audit", "authenticator"},
		AutoMode: AutoMode{ComputeEnabled: ptr(true)},
	}}
	got := findingsFor(t, cs)
	if len(got[AnalyzerAutoModeInvariant]) != 1 {
		t.Fatalf("expected an automode-invariant finding, got %+v", got)
	}
	if got[AnalyzerAutoModeInvariant][0].Severity != SeverityHigh {
		t.Errorf("severity = %q, want high", got[AnalyzerAutoModeInvariant][0].Severity)
	}

	// Fully absent is fine — that is a classic cluster.
	cs.Control.AutoMode = AutoMode{}
	if len(findingsFor(t, cs)[AnalyzerAutoModeInvariant]) != 0 {
		t.Error("a classic cluster was flagged for the Auto Mode invariant")
	}
}

// Auto Mode requires bootstrapSelfManagedAddons false, and the field is
// immutable, so this pairing can never reconcile.
func TestFindings_BootstrapAddons(t *testing.T) {
	cs := &ClusterSet{Cluster: "c", Control: &ClusterEntity{
		Name: "c", EncryptionConfigured: true, DeletionProtection: ptr(true),
		LogTypes: []string{"api", "audit", "authenticator"},
		AutoMode: AutoMode{
			ComputeEnabled: ptr(true), LoadBalancingEnabled: ptr(true), BlockStorageEnabled: ptr(true),
		},
		BootstrapSelfManagedAddons: ptr(true),
	}}
	got := findingsFor(t, cs)
	if len(got[AnalyzerBootstrapAddons]) != 1 {
		t.Fatalf("expected a bootstrap-addons finding, got %+v", got)
	}
	if !strings.Contains(got[AnalyzerBootstrapAddons][0].Fix, "recreated") {
		t.Errorf("fix should say the cluster must be recreated: %q", got[AnalyzerBootstrapAddons][0].Fix)
	}
}

func TestFindings_SupportPolicy(t *testing.T) {
	base := func(support string) *ClusterSet {
		return &ClusterSet{Cluster: "c", Control: &ClusterEntity{
			Name: "c", Version: "1.33", UpgradeSupportType: support,
			EncryptionConfigured: true, DeletionProtection: ptr(true),
			LogTypes: []string{"api", "audit", "authenticator"},
		}}
	}
	if len(findingsFor(t, base("STANDARD"))[AnalyzerSupportPolicy]) != 1 {
		t.Error("pinned version under STANDARD support not flagged")
	}
	if len(findingsFor(t, base("EXTENDED"))[AnalyzerSupportPolicy]) != 0 {
		t.Error("EXTENDED support was flagged")
	}
	// Case-insensitive, since the field is a free string.
	if len(findingsFor(t, base("standard"))[AnalyzerSupportPolicy]) != 1 {
		t.Error("lowercase 'standard' not matched")
	}
}

func TestFindings_AutoscalerConflict(t *testing.T) {
	mk := func(n *NodeGroupEntity) *ClusterSet {
		return &ClusterSet{Cluster: "c",
			Control: &ClusterEntity{Name: "c", Version: "1.34", UpgradeSupportType: "EXTENDED",
				EncryptionConfigured: true, DeletionProtection: ptr(true),
				LogTypes: []string{"api", "audit", "authenticator"}},
			NodeGroups: []*NodeGroupEntity{n},
		}
	}

	// desiredSize under forProvider with Update management: the conflict.
	got := findingsFor(t, mk(&NodeGroupEntity{
		Name: "ng", DesiredSize: ptr(int64(3)), MinSize: ptr(int64(1)), MaxSize: ptr(int64(10)),
		ManagementPolicies: []string{"Observe", "Create", "Update", "Delete"},
	}))
	if len(got[AnalyzerAutoscalerConflict]) != 1 {
		t.Fatalf("desiredSize conflict not flagged: %+v", got)
	}
	// The documented fix is currently broken, so the guidance must say so.
	if !strings.Contains(got[AnalyzerAutoscalerConflict][0].Fix, "Auto Mode") {
		t.Errorf("fix should recommend Auto Mode: %q", got[AnalyzerAutoscalerConflict][0].Fix)
	}

	// Under initProvider it is the intended pattern, so no conflict finding.
	got = findingsFor(t, mk(&NodeGroupEntity{
		Name: "ng", DesiredSize: ptr(int64(3)), MinSize: ptr(int64(1)), MaxSize: ptr(int64(10)),
		DesiredSizeInInitProvider: true,
		ManagementPolicies:        []string{"Observe", "Create", "Update", "Delete"},
	}))
	if len(got[AnalyzerAutoscalerConflict]) != 0 {
		t.Errorf("initProvider desiredSize was flagged: %+v", got[AnalyzerAutoscalerConflict])
	}

	// Without Update management Crossplane will not reconcile it.
	got = findingsFor(t, mk(&NodeGroupEntity{
		Name: "ng", DesiredSize: ptr(int64(3)), MinSize: ptr(int64(1)), MaxSize: ptr(int64(10)),
		ManagementPolicies: []string{"Observe", "Create", "Delete"},
	}))
	if len(got[AnalyzerAutoscalerConflict]) != 0 {
		t.Errorf("no-Update policy set was flagged: %+v", got[AnalyzerAutoscalerConflict])
	}

	// LateInitialize defeats every workaround.
	got = findingsFor(t, mk(&NodeGroupEntity{
		Name: "ng", DesiredSizeInInitProvider: true, DesiredSize: ptr(int64(3)),
		MinSize: ptr(int64(1)), MaxSize: ptr(int64(10)),
		ManagementPolicies: []string{"Observe", "Create", "Update", "Delete", "LateInitialize"},
	}))
	if len(got[AnalyzerAutoscalerConflict]) != 1 {
		t.Errorf("LateInitialize not flagged: %+v", got[AnalyzerAutoscalerConflict])
	}
}

func TestFindings_DesiredOutsideRange(t *testing.T) {
	mk := func(desired, min, max int64) map[string][]Finding {
		return findingsFor(t, &ClusterSet{Cluster: "c",
			Control: &ClusterEntity{Name: "c", Version: "1.34", UpgradeSupportType: "EXTENDED",
				EncryptionConfigured: true, DeletionProtection: ptr(true),
				LogTypes: []string{"api", "audit", "authenticator"}},
			NodeGroups: []*NodeGroupEntity{{
				Name: "ng", DesiredSize: ptr(desired), MinSize: ptr(min), MaxSize: ptr(max),
				DesiredSizeInInitProvider: true,
			}},
		})
	}
	if got := mk(0, 2, 6); len(got[AnalyzerAutoscalerConflict]) != 1 {
		t.Error("desiredSize below minSize not flagged")
	}
	if got := mk(9, 2, 6); len(got[AnalyzerAutoscalerConflict]) != 1 {
		t.Error("desiredSize above maxSize not flagged")
	}
	if got := mk(4, 2, 6); len(got[AnalyzerAutoscalerConflict]) != 0 {
		t.Error("in-range desiredSize was flagged")
	}
}

func TestFindings_VersionSkew(t *testing.T) {
	mk := func(cp, ng string) map[string][]Finding {
		return findingsFor(t, &ClusterSet{Cluster: "c",
			Control: &ClusterEntity{Name: "c", Version: cp, UpgradeSupportType: "EXTENDED",
				EncryptionConfigured: true, DeletionProtection: ptr(true),
				LogTypes: []string{"api", "audit", "authenticator"}},
			NodeGroups: []*NodeGroupEntity{{Name: "ng", Version: ng}},
		})
	}
	tests := []struct {
		cp, ng string
		count  int
		sev    Severity
	}{
		{"1.34", "1.34", 0, ""},
		{"1.34", "1.33", 1, SeverityLow},
		{"1.34", "1.32", 1, SeverityMedium},
		{"1.34", "1.31", 1, SeverityHigh},
		// A node group ahead of its control plane is unsupported by EKS.
		{"1.33", "1.34", 1, SeverityHigh},
	}
	for _, tt := range tests {
		got := mk(tt.cp, tt.ng)[AnalyzerVersionSkew]
		if len(got) != tt.count {
			t.Errorf("cp=%s ng=%s: %d findings, want %d", tt.cp, tt.ng, len(got), tt.count)
			continue
		}
		if tt.count > 0 && got[0].Severity != tt.sev {
			t.Errorf("cp=%s ng=%s: severity %q, want %q", tt.cp, tt.ng, got[0].Severity, tt.sev)
		}
	}
	// An unpinned node group tracks the control plane, so no skew.
	if got := mk("1.34", ""); len(got[AnalyzerVersionSkew]) != 0 {
		t.Error("unpinned node group flagged for skew")
	}
}

func TestFindings_Exposure(t *testing.T) {
	mk := func(mutate func(*ClusterEntity)) map[string][]Finding {
		c := &ClusterEntity{
			Name: "c", Version: "1.34", UpgradeSupportType: "EXTENDED",
			EncryptionConfigured: true, DeletionProtection: ptr(true),
			LogTypes: []string{"api", "audit", "authenticator"},
		}
		mutate(c)
		return findingsFor(t, &ClusterSet{Cluster: "c", Control: c})
	}

	// An unrestricted public endpoint is worse than a restricted one.
	open := mk(func(c *ClusterEntity) { c.EndpointPublicAccess = ptr(true) })
	if len(open[AnalyzerExposure]) != 1 || open[AnalyzerExposure][0].Severity != SeverityHigh {
		t.Errorf("unrestricted public endpoint = %+v, want one high finding", open[AnalyzerExposure])
	}
	restricted := mk(func(c *ClusterEntity) {
		c.EndpointPublicAccess = ptr(true)
		c.PublicAccessCIDRs = []string{"203.0.113.0/24"}
	})
	if len(restricted[AnalyzerExposure]) != 1 || restricted[AnalyzerExposure][0].Severity != SeverityMedium {
		t.Errorf("restricted public endpoint = %+v, want one medium finding", restricted[AnalyzerExposure])
	}
	wideOpen := mk(func(c *ClusterEntity) {
		c.EndpointPublicAccess = ptr(true)
		c.PublicAccessCIDRs = []string{"0.0.0.0/0"}
	})
	if wideOpen[AnalyzerExposure][0].Severity != SeverityHigh {
		t.Error("0.0.0.0/0 not graded high")
	}

	if got := mk(func(c *ClusterEntity) { c.EncryptionConfigured = false }); len(got[AnalyzerExposure]) != 1 {
		t.Error("missing encryptionConfig not flagged")
	}
	if got := mk(func(c *ClusterEntity) { c.LogTypes = []string{"api"} }); len(got[AnalyzerExposure]) != 1 {
		t.Error("missing log types not flagged")
	}
	if got := mk(func(c *ClusterEntity) { c.DeletionProtection = ptr(false) }); len(got[AnalyzerExposure]) != 1 {
		t.Error("deletionProtection false not flagged")
	}
}

// An unresolvable reference blocks reconciliation forever with no other signal.
func TestFindings_DanglingRef(t *testing.T) {
	cs := &ClusterSet{Cluster: "c",
		Control: &ClusterEntity{Name: "real-cluster", Version: "1.34", UpgradeSupportType: "EXTENDED",
			EncryptionConfigured: true, DeletionProtection: ptr(true),
			LogTypes: []string{"api", "audit", "authenticator"}},
		NodeGroups: []*NodeGroupEntity{{Name: "ng", ClusterName: "typo-cluster"}},
		Addons:     []*AddonEntity{{Name: "cni", ClusterName: "typo-cluster"}},
	}
	got := findingsFor(t, cs)
	if len(got[AnalyzerDanglingRef]) != 2 {
		t.Errorf("dangling refs = %d, want 2 (node group + addon)", len(got[AnalyzerDanglingRef]))
	}
	for _, f := range got[AnalyzerDanglingRef] {
		if f.Severity != SeverityHigh {
			t.Errorf("dangling ref severity = %q, want high", f.Severity)
		}
	}

	// A matching reference is fine.
	cs.NodeGroups[0].ClusterName = "real-cluster"
	cs.Addons[0].ClusterName = "real-cluster"
	if len(findingsFor(t, cs)[AnalyzerDanglingRef]) != 0 {
		t.Error("a resolving reference was flagged")
	}
}

// Clusters sharing an Environment should not run different versions.
func TestFindings_Consistency(t *testing.T) {
	mk := func(name, version, env string) *ClusterSet {
		return &ClusterSet{Cluster: name, Control: &ClusterEntity{
			Name: name, Version: version, UpgradeSupportType: "EXTENDED",
			EncryptionConfigured: true, DeletionProtection: ptr(true),
			LogTypes: []string{"api", "audit", "authenticator"},
			Origin:   ResourceOrigin{SpaceLabels: map[string]string{"Environment": env}},
		}}
	}
	fleet := map[string]*ClusterSet{
		"prod-a": mk("prod-a", "1.34", "prod"),
		"prod-b": mk("prod-b", "1.32", "prod"),
		"dev-a":  mk("dev-a", "1.34", "dev"),
	}
	var inconsistent []Finding
	for _, f := range Findings(fleet) {
		if f.Analyzer == AnalyzerInconsistent {
			inconsistent = append(inconsistent, f)
		}
	}
	// Both prod clusters are flagged; dev has only one cluster so is exempt.
	if len(inconsistent) != 2 {
		t.Fatalf("inconsistent findings = %d, want 2: %+v", len(inconsistent), inconsistent)
	}
	for _, f := range inconsistent {
		if !strings.Contains(f.Message, "prod") {
			t.Errorf("message does not name the environment: %q", f.Message)
		}
	}

	// Aligned versions produce nothing.
	fleet["prod-b"] = mk("prod-b", "1.34", "prod")
	for _, f := range Findings(fleet) {
		if f.Analyzer == AnalyzerInconsistent {
			t.Errorf("aligned fleet flagged: %+v", f)
		}
	}
}

func TestFindings_SortedBySeverity(t *testing.T) {
	fleet := map[string]*ClusterSet{
		"c": {Cluster: "c", Control: &ClusterEntity{
			Name: "c", Version: "1.34", UpgradeSupportType: "STANDARD",
			EndpointPublicAccess: ptr(true),  // high
			DeletionProtection:   ptr(false), // low
			LogTypes:             []string{"api"},
		}},
	}
	got := Findings(fleet)
	if len(got) < 3 {
		t.Fatalf("expected several findings, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if severityRank(got[i-1].Severity) < severityRank(got[i].Severity) {
			t.Errorf("findings not sorted worst-first: %q before %q", got[i-1].Severity, got[i].Severity)
		}
	}
	// Every finding must carry an actionable fix.
	for _, f := range got {
		if f.Fix == "" {
			t.Errorf("finding %s/%s has no fix", f.Analyzer, f.Name)
		}
		if f.Message == "" {
			t.Errorf("finding %s has no message", f.Analyzer)
		}
	}
}

func TestValidSeverity(t *testing.T) {
	for _, s := range []string{"critical", "high", "medium", "low"} {
		if !ValidSeverity(s) {
			t.Errorf("%q rejected", s)
		}
	}
	for _, s := range []string{"", "HIGH", "urgent", "info"} {
		if ValidSeverity(s) {
			t.Errorf("%q accepted", s)
		}
	}
}
