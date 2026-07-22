// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in   string
		want Version
		ok   bool
	}{
		{"1.34", Version{1, 34}, true},
		{"v1.34", Version{1, 34}, true},
		{"1.34.0", Version{1, 34}, true},
		{" 1.34 ", Version{1, 34}, true},
		// A node group's releaseVersion carries an AMI build suffix.
		{"1.34.0-20260701", Version{1, 34}, true},
		{"1.9", Version{1, 9}, true},
		{"", Version{}, false},
		{"1", Version{}, false},
		{"latest", Version{}, false},
		{"x.y", Version{}, false},
		{"1.x", Version{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseVersion(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseVersion(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b Version
		want int
	}{
		{Version{1, 34}, Version{1, 34}, 0},
		{Version{1, 33}, Version{1, 34}, -1},
		{Version{1, 34}, Version{1, 33}, 1},
		// Minor must compare numerically, not lexically: 1.9 < 1.10.
		{Version{1, 9}, Version{1, 10}, -1},
		{Version{1, 10}, Version{1, 9}, 1},
		{Version{2, 0}, Version{1, 99}, 1},
	}
	for _, tt := range tests {
		if got := tt.a.Compare(tt.b); got != tt.want {
			t.Errorf("%v.Compare(%v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMinorSkew(t *testing.T) {
	tests := []struct {
		newer, older Version
		want         int
	}{
		{Version{1, 34}, Version{1, 34}, 0},
		{Version{1, 34}, Version{1, 33}, 1},
		{Version{1, 34}, Version{1, 31}, 3},
		// A node group ahead of its control plane is not "behind".
		{Version{1, 33}, Version{1, 34}, 0},
		// Major mismatch is unknown, reported as 0.
		{Version{2, 0}, Version{1, 30}, 0},
	}
	for _, tt := range tests {
		if got := MinorSkew(tt.newer, tt.older); got != tt.want {
			t.Errorf("MinorSkew(%v, %v) = %d, want %d", tt.newer, tt.older, got, tt.want)
		}
	}
}

func TestUpgradeLegal(t *testing.T) {
	tests := []struct {
		name     string
		from, to Version
		wantOK   bool
	}{
		{"no-op", Version{1, 34}, Version{1, 34}, true},
		{"one minor forward", Version{1, 33}, Version{1, 34}, true},
		{"across the 9/10 boundary", Version{1, 9}, Version{1, 10}, true},
		// The two illegal transitions nothing downstream catches.
		{"downgrade", Version{1, 34}, Version{1, 33}, false},
		{"skips a minor", Version{1, 32}, Version{1, 34}, false},
		{"skips several", Version{1, 30}, Version{1, 34}, false},
		{"major bump", Version{1, 34}, Version{2, 0}, false},
	}
	for _, tt := range tests {
		ok, why := UpgradeLegal(tt.from, tt.to)
		if ok != tt.wantOK {
			t.Errorf("%s: UpgradeLegal(%v, %v) = %v (%q), want %v", tt.name, tt.from, tt.to, ok, why, tt.wantOK)
		}
		if !ok && why == "" {
			t.Errorf("%s: illegal transition reported with no explanation", tt.name)
		}
		if ok && why != "" {
			t.Errorf("%s: legal transition carried explanation %q", tt.name, why)
		}
	}
}

func TestVersionString(t *testing.T) {
	if got := (Version{1, 34}).String(); got != "1.34" {
		t.Errorf("String() = %q", got)
	}
}
