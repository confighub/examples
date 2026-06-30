// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package netpol

import "strings"

// Matches reports whether the given label set satisfies the selector. A
// not-present selector matches everything (lenient: in NetworkPolicy a
// podSelector is required, but peers may omit it to mean "unconstrained"). An
// empty present selector ({}) also matches everything.
func (s LabelSelector) Matches(labels map[string]string) bool {
	if !s.Present {
		return true
	}
	for k, v := range s.MatchLabels {
		if labels[k] != v {
			return false
		}
	}
	for _, req := range s.MatchExpressions {
		if !matchRequirement(req, labels) {
			return false
		}
	}
	return true
}

// matchRequirement evaluates one matchExpressions clause against a label set,
// per the Kubernetes set-based selector operators.
func matchRequirement(req LabelSelectorRequirement, labels map[string]string) bool {
	val, has := labels[req.Key]
	switch strings.ToLower(req.Operator) {
	case "in":
		return has && contains(req.Values, val)
	case "notin":
		return !has || !contains(req.Values, val)
	case "exists":
		return has
	case "doesnotexist":
		return !has
	}
	return false
}

func contains(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}
