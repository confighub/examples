// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package rbac

import (
	"strconv"
	"sync/atomic"
)

// Test fixtures: build ClusterRbac snapshots from plain Go docs the way the
// snapshot loader does (JSON-decoded resource bodies, i.e. map[string]any /
// []any throughout). Ported from the web app's fixtures.ts.

var fixtureCounter atomic.Int64

func res(cluster string, doc any) FleetResource {
	n := fixtureCounter.Add(1)
	return FleetResource{
		Origin: ResourceOrigin{
			Cluster:      cluster,
			Space:        cluster,
			SpaceID:      "space-" + cluster,
			UnitID:       "unit-" + itoa(n),
			UnitSlug:     "unit-" + itoa(n),
			ResourceName: "r-" + itoa(n),
		},
		Doc: doc,
	}
}

// crOpts are the optional fields of clusterRole (labels, aggregationRule).
type crOpts struct {
	labels          map[string]any
	aggregationRule any
}

func clusterRole(name string, rules []any, opts ...crOpts) map[string]any {
	var o crOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	return map[string]any{
		"apiVersion":      "rbac.authorization.k8s.io/v1",
		"kind":            "ClusterRole",
		"metadata":        map[string]any{"name": name, "labels": o.labels},
		"rules":           rules,
		"aggregationRule": o.aggregationRule,
	}
}

func role(name, namespace string, rules []any) map[string]any {
	return map[string]any{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "Role",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"rules":      rules,
	}
}

func clusterRoleBinding(name, roleName string, subjects []any) map[string]any {
	return map[string]any{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "ClusterRoleBinding",
		"metadata":   map[string]any{"name": name},
		"roleRef":    map[string]any{"apiGroup": rbacGroup, "kind": "ClusterRole", "name": roleName},
		"subjects":   subjects,
	}
}

func roleBinding(name, namespace string, roleRef RoleRef, subjects []any) map[string]any {
	return map[string]any{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "RoleBinding",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"roleRef":    map[string]any{"apiGroup": rbacGroup, "kind": roleRef.Kind, "name": roleRef.Name},
		"subjects":   subjects,
	}
}

func serviceAccount(name, namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}
}

func group(name string) map[string]any {
	return map[string]any{"kind": "Group", "name": name, "apiGroup": rbacGroup}
}

func user(name string) map[string]any {
	return map[string]any{"kind": "User", "name": name, "apiGroup": rbacGroup}
}

func sa(name, namespace string) map[string]any {
	return map[string]any{"kind": "ServiceAccount", "name": name, "namespace": namespace}
}

func build(resources []FleetResource) map[string]*ClusterRbac {
	return BuildClusterRbac(resources)
}

// ruleDoc builds a JSON-shaped policy rule (string slices become []any).
func ruleDoc(apiGroups, resources, verbs []string) map[string]any {
	return map[string]any{
		"apiGroups": toAny(apiGroups),
		"resources": toAny(resources),
		"verbs":     toAny(verbs),
	}
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
