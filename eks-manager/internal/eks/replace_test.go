// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"strings"
	"testing"
)

func TestNextNodeGroupName(t *testing.T) {
	tests := []struct {
		current string
		taken   []string
		want    string
	}{
		{"system", nil, "system-v2"},
		{"system-v2", nil, "system-v3"},
		{"system-v9", nil, "system-v10"},
		// Skip names already in use, so repeated replacements keep climbing.
		{"system", []string{"system-v2"}, "system-v3"},
		{"system", []string{"system-v2", "system-v3"}, "system-v4"},
		{"system-v2", []string{"system-v3"}, "system-v4"},
		// A name that merely contains a digit is not a generation marker.
		{"gpu-a100", nil, "gpu-a100-v2"},
		{"batch2", nil, "batch2-v2"},
	}
	for _, tt := range tests {
		taken := map[string]bool{}
		for _, n := range tt.taken {
			taken[n] = true
		}
		if got := NextNodeGroupName(tt.current, taken); got != tt.want {
			t.Errorf("NextNodeGroupName(%q, %v) = %q, want %q", tt.current, tt.taken, got, tt.want)
		}
	}
}

// The replacement must carry forward everything it can from the original.
func TestNodeGroupSpecFrom(t *testing.T) {
	old := &NodeGroupEntity{
		Name:          "system",
		InstanceTypes: []string{"m6i.large", "m6a.large"},
		CapacityType:  "SPOT",
		AMIType:       "AL2023_x86_64_STANDARD",
		MinSize:       ptr(int64(2)),
		MaxSize:       ptr(int64(9)),
		DesiredSize:   ptr(int64(4)),
		DiskSize:      ptr(int64(120)),
	}
	s := NodeGroupSpecFrom(old)
	if s.Name != "system" || s.CapacityType != "SPOT" || s.AMIType != "AL2023_x86_64_STANDARD" {
		t.Errorf("scalars not carried: %+v", s)
	}
	if len(s.InstanceTypes) != 2 || s.InstanceTypes[0] != "m6i.large" {
		t.Errorf("instanceTypes = %v", s.InstanceTypes)
	}
	if s.MinSize != 2 || s.MaxSize != 9 || s.DesiredSize != 4 || s.DiskSize != 120 {
		t.Errorf("sizes not carried: %+v", s)
	}

	// The copy must be independent: mutating it must not corrupt the original.
	s.InstanceTypes[0] = "changed"
	if old.InstanceTypes[0] != "m6i.large" {
		t.Error("NodeGroupSpecFrom aliased the original's slice")
	}

	// Absent optional sizes become zero rather than panicking.
	empty := NodeGroupSpecFrom(&NodeGroupEntity{Name: "x"})
	if empty.MinSize != 0 || empty.DiskSize != 0 {
		t.Errorf("nil sizes = %+v", empty)
	}
}

func TestImmutableDiff(t *testing.T) {
	const av = "eks.aws.upbound.io/v1beta2"
	old := &NodeGroupEntity{
		Name: "system", InstanceTypes: []string{"m6i.large"}, CapacityType: "ON_DEMAND",
		AMIType: "AL2023_x86_64_STANDARD", DiskSize: ptr(int64(80)),
		MinSize: ptr(int64(2)), MaxSize: ptr(int64(6)),
	}

	// Identical spec: nothing forces a replacement.
	if got := ImmutableDiff(av, old, NodeGroupSpecFrom(old)); len(got) != 0 {
		t.Errorf("identical spec reported changes: %v", got)
	}

	// Scaling alone must not force one either — that is an in-place edit.
	scaled := NodeGroupSpecFrom(old)
	scaled.MinSize, scaled.MaxSize, scaled.DesiredSize = 5, 20, 5
	if got := ImmutableDiff(av, old, scaled); len(got) != 0 {
		t.Errorf("scaling reported as immutable: %v", got)
	}

	// Each immutable field on its own.
	for _, tc := range []struct {
		name   string
		mutate func(*NodeGroupSpec)
		expect string
	}{
		{"instanceTypes", func(s *NodeGroupSpec) { s.InstanceTypes = []string{"m6i.2xlarge"} }, "instanceTypes"},
		{"capacityType", func(s *NodeGroupSpec) { s.CapacityType = "SPOT" }, "capacityType"},
		{"amiType", func(s *NodeGroupSpec) { s.AMIType = "AL2023_ARM_64_STANDARD" }, "amiType"},
		{"diskSize", func(s *NodeGroupSpec) { s.DiskSize = 200 }, "diskSize"},
	} {
		s := NodeGroupSpecFrom(old)
		tc.mutate(&s)
		got := ImmutableDiff(av, old, s)
		if len(got) != 1 {
			t.Errorf("%s: %d changes, want 1: %v", tc.name, len(got), got)
			continue
		}
		if !strings.HasPrefix(got[0], tc.expect) {
			t.Errorf("%s: reported %q", tc.name, got[0])
		}
		// The message must show both sides so the operator can sanity-check it.
		if !strings.Contains(got[0], "->") {
			t.Errorf("%s: no before/after in %q", tc.name, got[0])
		}
	}

	// Several at once are all reported, not just the first.
	s := NodeGroupSpecFrom(old)
	s.InstanceTypes = []string{"m6i.2xlarge"}
	s.CapacityType = "SPOT"
	if got := ImmutableDiff(av, old, s); len(got) != 2 {
		t.Errorf("combined changes = %v, want 2", got)
	}
}

// The deprecated list-shaped API version must classify identically.
func TestImmutableDiff_VersionAgnostic(t *testing.T) {
	old := &NodeGroupEntity{Name: "x", InstanceTypes: []string{"m6i.large"}}
	s := NodeGroupSpecFrom(old)
	s.InstanceTypes = []string{"m6i.xlarge"}
	for _, av := range []string{
		"eks.aws.upbound.io/v1beta1",
		"eks.aws.upbound.io/v1beta2",
		"eks.aws.m.upbound.io/v1beta1",
	} {
		if got := ImmutableDiff(av, old, s); len(got) != 1 {
			t.Errorf("%s: %d changes, want 1", av, len(got))
		}
	}
}

// A replacement generated from an existing node group must be a valid, complete
// resource — not a partial patch.
func TestReplacementGeneratesCompleteResource(t *testing.T) {
	old := &NodeGroupEntity{
		Name: "system", InstanceTypes: []string{"m6i.large"}, CapacityType: "ON_DEMAND",
		MinSize: ptr(int64(2)), MaxSize: ptr(int64(6)), DesiredSize: ptr(int64(2)),
		DiskSize: ptr(int64(80)), Version: "1.34",
	}
	s := NodeGroupSpecFrom(old)
	s.InstanceTypes = []string{"m6i.2xlarge"}
	s.Name = NextNodeGroupName(old.Name, map[string]bool{"system": true})

	u, err := GenerateNodeGroup(ClusterContext{
		Name: "demo", Region: "us-east-1", Version: "1.34", ProviderConfig: "default",
	}, s)
	if err != nil {
		t.Fatal(err)
	}
	if u.Slug != "nodegroup-system-v2" {
		t.Errorf("slug = %q", u.Slug)
	}
	for _, want := range []string{
		"name: system-v2", "m6i.2xlarge", "clusterNameRef", "nodeRoleArnRef", "subnetIdSelector",
	} {
		if !strings.Contains(u.YAML, want) {
			t.Errorf("replacement missing %q:\n%s", want, u.YAML)
		}
	}
	// It must not carry the original's name anywhere.
	if strings.Contains(u.YAML, "name: system\n") {
		t.Errorf("replacement reuses the original name:\n%s", u.YAML)
	}
}
