// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestClassifyPath_NodeGroup(t *testing.T) {
	const rt = "eks.aws.upbound.io/v1beta2/NodeGroup"
	tests := []struct {
		path string
		want Disruption
	}{
		// ForceNew: cannot be reconciled in place; Crossplane wedges.
		{"spec.forProvider.instanceTypes", DisruptionReplace},
		{"spec.forProvider.instanceTypes.0", DisruptionReplace},
		{"spec.forProvider.amiType", DisruptionReplace},
		{"spec.forProvider.capacityType", DisruptionReplace},
		{"spec.forProvider.diskSize", DisruptionReplace},
		{"spec.forProvider.remoteAccess.ec2SshKey", DisruptionReplace},
		{"metadata.name", DisruptionReplace},
		// Reference forms are the same logical field.
		{"spec.forProvider.nodeRoleArnRef.name", DisruptionReplace},
		{"spec.forProvider.clusterNameRef.name", DisruptionReplace},
		{"spec.forProvider.subnetIdRefs.0.name", DisruptionReplace},
		{"spec.forProvider.subnetIdSelector.matchLabels.tier", DisruptionReplace},
		// In-place but rolls every node.
		{"spec.forProvider.version", DisruptionRolling},
		{"spec.forProvider.releaseVersion", DisruptionRolling},
		{"spec.forProvider.launchTemplate.version", DisruptionRolling},
		// Genuinely in-place.
		{"spec.forProvider.scalingConfig.desiredSize", DisruptionNone},
		{"spec.forProvider.scalingConfig.minSize", DisruptionNone},
		{"spec.forProvider.labels.role", DisruptionNone},
		{"spec.forProvider.tags.Owner", DisruptionNone},
		{"spec.forProvider.updateConfig.maxUnavailablePercentage", DisruptionNone},
	}
	for _, tt := range tests {
		got := ClassifyPath(rt, tt.path)
		if got.Disruption != tt.want {
			t.Errorf("ClassifyPath(%q) = %q, want %q", tt.path, got.Disruption, tt.want)
		}
		if tt.want != DisruptionNone && got.Reason == "" {
			t.Errorf("ClassifyPath(%q) graded %q with no reason", tt.path, got.Disruption)
		}
	}
}

// launchTemplate.version is a rolling update while launchTemplate.id is a
// replacement, so the more specific rule must win over the shorter prefix.
func TestClassifyPath_LongestPrefixWins(t *testing.T) {
	const rt = "eks.aws.upbound.io/v1beta2/NodeGroup"
	if got := ClassifyPath(rt, "spec.forProvider.launchTemplate.version"); got.Disruption != DisruptionRolling {
		t.Errorf("launchTemplate.version = %q, want rolling", got.Disruption)
	}
	if got := ClassifyPath(rt, "spec.forProvider.launchTemplate.id"); got.Disruption != DisruptionReplace {
		t.Errorf("launchTemplate.id = %q, want replace", got.Disruption)
	}
}

// A rule must not match a sibling field that merely shares a string prefix.
func TestClassifyPath_NoSiblingPrefixBleed(t *testing.T) {
	const rt = "eks.aws.upbound.io/v1beta2/Cluster"
	// "versionRef" IS the version field by reference, so it should match...
	if got := ClassifyPath(rt, "spec.forProvider.versionRef.name"); got.Disruption != DisruptionLow {
		t.Errorf("versionRef = %q, want the version rule to apply", got.Disruption)
	}
	// ...but an unrelated field starting with the same letters must not.
	if got := ClassifyPath(rt, "spec.forProvider.versioningNonsense"); got.Disruption != DisruptionNone {
		t.Errorf("versioningNonsense = %q, want none", got.Disruption)
	}
}

func TestClassifyPath_Cluster(t *testing.T) {
	const rt = "eks.aws.upbound.io/v1beta2/Cluster"
	tests := []struct {
		path string
		want Disruption
	}{
		// Replacing a control plane is a new cluster and everything on it.
		{"spec.forProvider.roleArnRef.name", DisruptionReplaceCluster},
		{"spec.forProvider.bootstrapSelfManagedAddons", DisruptionReplaceCluster},
		{"spec.forProvider.kubernetesNetworkConfig.serviceIpv4Cidr", DisruptionReplaceCluster},
		{"spec.forProvider.kubernetesNetworkConfig.ipFamily", DisruptionReplaceCluster},
		{"metadata.name", DisruptionReplaceCluster},
		// In-place.
		{"spec.forProvider.version", DisruptionLow},
		{"spec.forProvider.vpcConfig.endpointPublicAccess", DisruptionNone},
		{"spec.forProvider.enabledClusterLogTypes.0", DisruptionNone},
		{"spec.forProvider.accessConfig.authenticationMode", DisruptionNone},
		{"spec.forProvider.deletionProtection", DisruptionNone},
		// Nested under kubernetesNetworkConfig, but Auto Mode's ELB toggle is
		// in-place while ipFamily under the same parent replaces the cluster.
		{"spec.forProvider.kubernetesNetworkConfig.elasticLoadBalancing.enabled", DisruptionLow},
	}
	for _, tt := range tests {
		if got := ClassifyPath(rt, tt.path); got.Disruption != tt.want {
			t.Errorf("ClassifyPath(%q) = %q, want %q", tt.path, got.Disruption, tt.want)
		}
	}
}

// The same rules must apply to the deprecated list-shaped v1beta1, where paths
// carry numeric indices.
func TestClassifyPath_VersionAgnosticAndIndexAgnostic(t *testing.T) {
	for _, rt := range []string{
		"eks.aws.upbound.io/v1beta1/NodeGroup",
		"eks.aws.upbound.io/v1beta2/NodeGroup",
		"eks.aws.m.upbound.io/v1beta1/NodeGroup",
	} {
		if got := ClassifyPath(rt, "spec.forProvider.instanceTypes"); got.Disruption != DisruptionReplace {
			t.Errorf("%s: instanceTypes = %q, want replace", rt, got.Disruption)
		}
	}
	// List-shaped v1beta1 path with an index.
	if got := ClassifyPath("eks.aws.upbound.io/v1beta1/NodeGroup",
		"spec.forProvider.launchTemplate.0.version"); got.Disruption != DisruptionRolling {
		t.Errorf("indexed launchTemplate.version = %q, want rolling", got.Disruption)
	}
	if got := ClassifyPath("eks.aws.upbound.io/v1beta1/NodeGroup",
		"spec.forProvider.scalingConfig.0.desiredSize"); got.Disruption != DisruptionNone {
		t.Errorf("indexed scalingConfig.desiredSize = %q, want none", got.Disruption)
	}
}

func TestClassifyPath_UnknownResourceType(t *testing.T) {
	// An unmodelled kind must grade as no disruption rather than guessing.
	if got := ClassifyPath("rds.aws.upbound.io/v1beta1/Instance", "spec.forProvider.engine"); got.Disruption != DisruptionNone {
		t.Errorf("unknown type = %q, want none", got.Disruption)
	}
	if got := ClassifyPath("", "x"); got.Disruption != DisruptionNone {
		t.Errorf("empty type = %q, want none", got.Disruption)
	}
}

func TestClassifyResource_Aggregates(t *testing.T) {
	rc := ClassifyResource("eks.aws.upbound.io/v1beta2/NodeGroup", "batch", []string{
		"spec.forProvider.scalingConfig.desiredSize",
		"spec.forProvider.version",
		"spec.forProvider.instanceTypes.0",
	})
	if rc.MaxDisruption != DisruptionReplace {
		t.Errorf("MaxDisruption = %q, want replace", rc.MaxDisruption)
	}
	if rc.MaxScore != "High" {
		t.Errorf("MaxScore = %q, want High", rc.MaxScore)
	}
	if !rc.Blocks {
		t.Error("Blocks = false; a replacement cannot be applied in place")
	}
	// Worst first.
	if rc.Changes[0].Disruption != DisruptionReplace {
		t.Errorf("changes not sorted worst-first: %+v", rc.Changes)
	}
	if rc.Changes[len(rc.Changes)-1].Disruption != DisruptionNone {
		t.Errorf("benign change not sorted last: %+v", rc.Changes)
	}

	// A purely in-place change set must not block.
	rc = ClassifyResource("eks.aws.upbound.io/v1beta2/NodeGroup", "batch", []string{
		"spec.forProvider.scalingConfig.desiredSize",
	})
	if rc.Blocks || rc.MaxDisruption != DisruptionNone {
		t.Errorf("scaling graded as %q, blocks=%v; want none/false", rc.MaxDisruption, rc.Blocks)
	}
}

func TestDisruptionScoreAndRank(t *testing.T) {
	tests := []struct {
		d     Disruption
		score string
		rank  int
		block bool
	}{
		{DisruptionReplaceCluster, "Critical", 4, true},
		{DisruptionReplace, "High", 3, true},
		{DisruptionRolling, "Medium", 2, false},
		{DisruptionLow, "Low", 1, false},
		{DisruptionNone, "", 0, false},
	}
	for _, tt := range tests {
		if tt.d.Score() != tt.score || tt.d.Rank() != tt.rank || tt.d.Blocks() != tt.block {
			t.Errorf("%q: score=%q rank=%d blocks=%v; want %q/%d/%v",
				tt.d, tt.d.Score(), tt.d.Rank(), tt.d.Blocks(), tt.score, tt.rank, tt.block)
		}
	}
	if MaxDisruption(DisruptionRolling, DisruptionReplaceCluster) != DisruptionReplaceCluster {
		t.Error("MaxDisruption did not pick the worse tier")
	}
	if MaxDisruption(DisruptionReplace, DisruptionNone) != DisruptionReplace {
		t.Error("MaxDisruption regressed to the milder tier")
	}
}

func mustDoc(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return v
}

func TestDiffPaths(t *testing.T) {
	old := mustDoc(t, `{"spec":{"forProvider":{
	  "version":"1.33","instanceTypes":["m6i.large"],
	  "scalingConfig":{"minSize":2,"maxSize":6,"desiredSize":2},
	  "tags":{"a":"1"}}}}`)
	new := mustDoc(t, `{"spec":{"forProvider":{
	  "version":"1.34","instanceTypes":["m6i.xlarge"],
	  "scalingConfig":{"minSize":2,"maxSize":9,"desiredSize":2},
	  "tags":{"a":"1","b":"2"}}}}`)

	got := DiffPaths(old, new)
	want := []string{
		"spec.forProvider.instanceTypes.0",
		"spec.forProvider.scalingConfig.maxSize",
		"spec.forProvider.tags.b",
		"spec.forProvider.version",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiffPaths = %v\nwant %v", got, want)
	}
}

func TestDiffPaths_AddedAndRemoved(t *testing.T) {
	// A field removed from the new document is still a change.
	old := mustDoc(t, `{"spec":{"forProvider":{"diskSize":100,"amiType":"AL2023"}}}`)
	new := mustDoc(t, `{"spec":{"forProvider":{"diskSize":100}}}`)
	got := DiffPaths(old, new)
	if len(got) != 1 || got[0] != "spec.forProvider.amiType" {
		t.Errorf("removal not detected: %v", got)
	}

	// And a field added.
	got = DiffPaths(new, old)
	if len(got) != 1 || got[0] != "spec.forProvider.amiType" {
		t.Errorf("addition not detected: %v", got)
	}
}

func TestDiffPaths_ListLengthChange(t *testing.T) {
	old := mustDoc(t, `{"a":["x"]}`)
	new := mustDoc(t, `{"a":["x","y"]}`)
	got := DiffPaths(old, new)
	if len(got) != 1 || got[0] != "a.1" {
		t.Errorf("list growth = %v, want [a.1]", got)
	}
}

func TestDiffPaths_NoChange(t *testing.T) {
	d := mustDoc(t, `{"spec":{"forProvider":{"version":"1.34","tags":{"a":"1"}}}}`)
	if got := DiffPaths(d, d); len(got) != 0 {
		t.Errorf("identical documents reported changes: %v", got)
	}
}

// The end-to-end shape: diff two revisions of a node group, classify the result.
func TestDiffAndClassify_TheWedgeCase(t *testing.T) {
	lastApplied := mustDoc(t, `{
	  "apiVersion":"eks.aws.upbound.io/v1beta2","kind":"NodeGroup",
	  "metadata":{"name":"batch"},
	  "spec":{"forProvider":{"instanceTypes":["m6i.large"],
	    "scalingConfig":{"minSize":2,"maxSize":6,"desiredSize":2}}}}`)
	head := mustDoc(t, `{
	  "apiVersion":"eks.aws.upbound.io/v1beta2","kind":"NodeGroup",
	  "metadata":{"name":"batch"},
	  "spec":{"forProvider":{"instanceTypes":["m6i.xlarge"],
	    "scalingConfig":{"minSize":2,"maxSize":6,"desiredSize":2}}}}`)

	rc := ClassifyResource("eks.aws.upbound.io/v1beta2/NodeGroup", "batch",
		DiffPaths(lastApplied, head))

	// This is the change that looks completely benign in a diff and then does
	// nothing, forever.
	if !rc.Blocks {
		t.Fatal("an instanceTypes change was not flagged as blocking")
	}
	if rc.MaxDisruption != DisruptionReplace {
		t.Errorf("MaxDisruption = %q, want replace", rc.MaxDisruption)
	}
	if r := Remediation(rc.MaxDisruption, "NodeGroup"); r == "" {
		t.Error("no remediation offered for a blocking change")
	}
}

func TestRemediation(t *testing.T) {
	if Remediation(DisruptionReplaceCluster, "Cluster") == "" {
		t.Error("no remediation for replace-cluster")
	}
	if got := Remediation(DisruptionReplace, "NodeGroup"); got == "" {
		t.Error("no remediation for nodegroup replace")
	}
	if got := Remediation(DisruptionReplace, "Subnet"); got == "" {
		t.Error("no remediation for subnet replace")
	}
	if Remediation(DisruptionNone, "NodeGroup") != "" {
		t.Error("remediation offered for a benign change")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := map[string]string{
		"spec.forProvider.scalingConfig.0.desiredSize": "spec.forProvider.scalingConfig.desiredSize",
		"spec.forProvider.instanceTypes.0":             "spec.forProvider.instanceTypes",
		"a.10.b.2.c":                                   "a.b.c",
		"no.indices.here":                              "no.indices.here",
	}
	for in, want := range tests {
		if got := normalizePath(in); got != want {
			t.Errorf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDisruptionKey(t *testing.T) {
	tests := map[string]string{
		"eks.aws.upbound.io/v1beta2/Cluster":   "eks.aws.upbound.io/Cluster",
		"eks.aws.upbound.io/v1beta1/NodeGroup": "eks.aws.upbound.io/NodeGroup",
		"v1/Pod":                               "v1/Pod",
		"Cluster":                              "Cluster",
	}
	for in, want := range tests {
		if got := disruptionKey(in); got != want {
			t.Errorf("disruptionKey(%q) = %q, want %q", in, got, want)
		}
	}
}
