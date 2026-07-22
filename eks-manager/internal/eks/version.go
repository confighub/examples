// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a Kubernetes major.minor version. EKS pins control planes and node
// groups at minor granularity ("1.34"); patch level is AWS's to choose, and
// surfaces separately as the platform version or a node group's releaseVersion.
type Version struct {
	Major int
	Minor int
}

// ParseVersion parses a Kubernetes version, tolerating the forms that appear
// across these resources: "1.34", "v1.34", "1.34.0", and a node group's
// releaseVersion "1.34.0-20260701". Everything after the minor is ignored.
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}, false
	}
	// Drop any AMI/build suffix ("1.34.0-20260701").
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return Version{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, false
	}
	return Version{Major: major, Minor: minor}, true
}

// String renders the version as "major.minor".
func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

// Compare returns -1 if v < o, 0 if equal, +1 if v > o.
func (v Version) Compare(o Version) int {
	switch {
	case v.Major != o.Major:
		if v.Major < o.Major {
			return -1
		}
		return 1
	case v.Minor != o.Minor:
		if v.Minor < o.Minor {
			return -1
		}
		return 1
	}
	return 0
}

// MinorSkew returns how many minor versions `older` is behind `newer`, within
// the same major. It returns 0 when older is at or ahead of newer, and is only
// meaningful for matching majors (Kubernetes has had one major for its whole
// life; a major mismatch yields 0 and should be treated as unknown).
func MinorSkew(newer, older Version) int {
	if newer.Major != older.Major {
		return 0
	}
	if older.Minor >= newer.Minor {
		return 0
	}
	return newer.Minor - older.Minor
}

// UpgradeLegal reports whether moving a control plane from `from` to `to` is a
// legal EKS transition, and why not when it isn't.
//
// EKS permits exactly one minor version forward at a time and no downgrade at
// all. Nothing downstream enforces this: the CRD marks version as an ordinary
// optional field with no CEL validation, and the provider passes it straight
// through — so an illegal transition becomes an InvalidParameterException from
// AWS and a permanently Synced=False managed resource. Checking it client-side
// is the only thing standing between a typo and a wedged cluster.
func UpgradeLegal(from, to Version) (bool, string) {
	switch {
	case from.Compare(to) == 0:
		return true, ""
	case to.Compare(from) < 0:
		return false, fmt.Sprintf("downgrade %s -> %s: EKS control planes cannot be downgraded", from, to)
	case to.Major != from.Major:
		return false, fmt.Sprintf("%s -> %s crosses a major version", from, to)
	case to.Minor-from.Minor > 1:
		return false, fmt.Sprintf("%s -> %s skips %d minor versions: upgrade one minor at a time",
			from, to, to.Minor-from.Minor-1)
	}
	return true, ""
}
