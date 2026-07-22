// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// NodeGroupSpecFrom derives a generation spec from a parsed node group, so a
// replacement carries forward everything about the original except what is
// deliberately changed.
//
// Fields the model does not parse (labels, taints, updateConfig) are not carried
// over; the generator re-emits its own defaults for those. That is a known
// limitation of replacement, called out to the operator rather than hidden.
func NodeGroupSpecFrom(n *NodeGroupEntity) NodeGroupSpec {
	s := NodeGroupSpec{
		Name:          n.Name,
		InstanceTypes: append([]string(nil), n.InstanceTypes...),
		CapacityType:  n.CapacityType,
		AMIType:       n.AMIType,
	}
	if n.MinSize != nil {
		s.MinSize = *n.MinSize
	}
	if n.MaxSize != nil {
		s.MaxSize = *n.MaxSize
	}
	if n.DesiredSize != nil {
		s.DesiredSize = *n.DesiredSize
	}
	if n.DiskSize != nil {
		s.DiskSize = *n.DiskSize
	}
	return s
}

// versionSuffix matches a trailing -v<N> generation marker.
var versionSuffix = regexp.MustCompile(`^(.*)-v(\d+)$`)

// NextNodeGroupName derives the replacement's name. A node group name is its
// identity in AWS, and the replacement must coexist with the original during the
// swap, so the two cannot share a name.
//
// "system" becomes "system-v2"; "system-v2" becomes "system-v3". Taken names are
// skipped, so repeated replacements keep climbing rather than colliding.
func NextNodeGroupName(current string, taken map[string]bool) string {
	base, n := current, 1
	if m := versionSuffix.FindStringSubmatch(current); m != nil {
		base = m[1]
		if parsed, err := strconv.Atoi(m[2]); err == nil {
			n = parsed
		}
	}
	for i := n + 1; i < n+100; i++ {
		candidate := fmt.Sprintf("%s-v%d", base, i)
		if !taken[candidate] {
			return candidate
		}
	}
	return fmt.Sprintf("%s-v%d", base, n+1)
}

// ImmutableDiff reports which immutable (replacement-forcing) fields differ
// between an existing node group and a proposed replacement spec, using the same
// disruption table `plan` consults.
func ImmutableDiff(apiVersion string, old *NodeGroupEntity, proposed NodeGroupSpec) []string {
	resourceType := apiVersion + "/NodeGroup"
	var changed []string

	check := func(path, before, after string) {
		if before == after {
			return
		}
		if ClassifyPath(resourceType, path).Disruption.Blocks() {
			changed = append(changed, fmt.Sprintf("%s: %s -> %s",
				strings.TrimPrefix(path, "spec.forProvider."), dashEmpty(before), dashEmpty(after)))
		}
	}

	check("spec.forProvider.instanceTypes",
		strings.Join(old.InstanceTypes, ","), strings.Join(proposed.InstanceTypes, ","))
	check("spec.forProvider.capacityType", old.CapacityType, proposed.CapacityType)
	check("spec.forProvider.amiType", old.AMIType, proposed.AMIType)
	if old.DiskSize != nil {
		check("spec.forProvider.diskSize", strconv.FormatInt(*old.DiskSize, 10),
			strconv.FormatInt(proposed.DiskSize, 10))
	} else if proposed.DiskSize != 0 {
		check("spec.forProvider.diskSize", "", strconv.FormatInt(proposed.DiskSize, 10))
	}
	return changed
}

func dashEmpty(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
