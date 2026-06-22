// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Kubernetes RBAC matching semantics, implemented over the model in model.go.
// Mirrors the API server's authorization rules: wildcard verbs/resources/
// apiGroups, resource/subresource forms, resourceNames, nonResourceURLs,
// ClusterRole aggregation, and binding scope resolution. Faithful Go port of
// the web app's semantics.ts; behavior is locked by tests.

package rbac

import "strings"

// AccessQuery is a resource-access question: "who can VERB RESOURCE [in
// NAMESPACE] [named NAME]?".
type AccessQuery struct {
	Verb string
	// Resource is the plural resource, optionally with subresource:
	// "pods", "pods/log".
	Resource string
	// APIGroup is the API group; "" (core) when omitted.
	APIGroup string
	// Namespace restricts to grants effective in this namespace. "" means
	// "anywhere".
	Namespace string
	// Name is a specific object name, to honor resourceNames restrictions.
	// "" means no specific object is queried.
	Name string
}

func verbMatches(ruleVerbs []string, verb string) bool {
	for _, v := range ruleVerbs {
		if v == "*" || v == verb {
			return true
		}
	}
	return false
}

func groupMatches(ruleGroups []string, group string) bool {
	for _, g := range ruleGroups {
		if g == "*" || g == group {
			return true
		}
	}
	return false
}

// resourceEntryMatches matches a rule's resource entry against a queried
// resource. Entries and queries may carry a subresource ("pods/log"); each
// slash-separated segment matches exactly or via "*" ("*/scale" matches
// "deployments/scale"). A bare entry never matches a subresource query and vice
// versa.
func resourceEntryMatches(entry, resource string) bool {
	if entry == "*" {
		return true
	}
	entrySegs := strings.Split(entry, "/")
	querySegs := strings.Split(resource, "/")
	if len(entrySegs) != len(querySegs) {
		return false
	}
	for i, seg := range entrySegs {
		if seg != "*" && seg != querySegs[i] {
			return false
		}
	}
	return true
}

func resourceMatches(ruleResources []string, resource string) bool {
	for _, entry := range ruleResources {
		if resourceEntryMatches(entry, resource) {
			return true
		}
	}
	return false
}

// nameMatches honors resourceNames, which restrict a rule to specific objects.
// An empty list means all names. When the query names no object (""), restricted
// rules still "can" reach some object, so they match — callers that need strict
// per-object answers must pass a non-empty query name.
func nameMatches(ruleNames []string, name string) bool {
	if len(ruleNames) == 0 {
		return true
	}
	if name == "" {
		return true
	}
	for _, n := range ruleNames {
		if n == name {
			return true
		}
	}
	return false
}

// RuleMatches reports whether a single policy rule grants the queried access.
func RuleMatches(rule PolicyRule, query AccessQuery) bool {
	if len(rule.Resources) == 0 {
		return false // nonResourceURL-only rule
	}
	return verbMatches(rule.Verbs, query.Verb) &&
		groupMatches(rule.APIGroups, query.APIGroup) &&
		resourceMatches(rule.Resources, query.Resource) &&
		nameMatches(rule.ResourceNames, query.Name)
}

// NonResourceURLMatches reports non-resource URL access: exact, or prefix via a
// trailing "*".
func NonResourceURLMatches(rule PolicyRule, verb, url string) bool {
	if !verbMatches(rule.Verbs, verb) {
		return false
	}
	for _, entry := range rule.NonResourceURLs {
		if entry == "*" {
			return true
		}
		if strings.HasSuffix(entry, "*") {
			if strings.HasPrefix(url, entry[:len(entry)-1]) {
				return true
			}
			continue
		}
		if entry == url {
			return true
		}
	}
	return false
}

func labelsMatch(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// EffectiveRules returns the effective rules of a role. For aggregated
// ClusterRoles, the API server's controller unions the rules of every
// ClusterRole matching any selector; aggregated roles can themselves aggregate,
// so iterate to a fixed point.
func EffectiveRules(role *RoleEntity, cluster *ClusterRbac) []PolicyRule {
	if len(role.AggregationSelectors) == 0 {
		return role.Rules
	}

	collected := map[*RoleEntity]bool{}
	pending := []*RoleEntity{role}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if collected[current] {
			continue
		}
		collected[current] = true
		if len(current.AggregationSelectors) == 0 {
			continue
		}
		for _, candidate := range cluster.Roles {
			if candidate.Kind != "ClusterRole" || collected[candidate] {
				continue
			}
			for _, sel := range current.AggregationSelectors {
				if labelsMatch(candidate.Labels, sel) {
					pending = append(pending, candidate)
					break
				}
			}
		}
	}

	var rules []PolicyRule
	for r := range collected {
		rules = append(rules, r.Rules...)
	}
	return rules
}

// builtinClusterRoles are ClusterRoles Kubernetes ships in every cluster; they
// exist even when not stored in ConfigHub, so bindings referencing them are not
// orphans, and cluster-admin's rules are known without a manifest.
var builtinClusterRoles = map[string]bool{
	"cluster-admin": true,
	"admin":         true,
	"edit":          true,
	"view":          true,
}

// IsBuiltinRoleName reports whether name is a built-in ClusterRole or a
// system: role that Kubernetes provides by default.
func IsBuiltinRoleName(name string) bool {
	return builtinClusterRoles[name] || strings.HasPrefix(name, "system:")
}

// ResolveRoleRef resolves a binding's roleRef within its cluster. RoleBindings
// may reference a Role in their own namespace or any ClusterRole;
// ClusterRoleBindings reference ClusterRoles only. Returns nil when the role is
// not in the snapshot (possibly a builtin — see IsBuiltinRoleName).
func ResolveRoleRef(binding *BindingEntity, cluster *ClusterRbac) *RoleEntity {
	kind, name := binding.RoleRef.Kind, binding.RoleRef.Name
	if kind == "ClusterRole" {
		for _, r := range cluster.Roles {
			if r.Kind == "ClusterRole" && r.Name == name {
				return r
			}
		}
		return nil
	}
	if kind == "Role" && binding.Kind == "RoleBinding" {
		for _, r := range cluster.Roles {
			if r.Kind == "Role" && r.Name == name && r.Namespace == binding.Namespace {
				return r
			}
		}
	}
	return nil
}

// BindingScopeMatches reports whether this binding's grant applies in the
// queried namespace. A ClusterRoleBinding applies everywhere; a RoleBinding only
// within its own namespace. An empty query namespace means "anywhere".
func BindingScopeMatches(binding *BindingEntity, namespace string) bool {
	if binding.Kind == "ClusterRoleBinding" {
		return true
	}
	if namespace == "" {
		return true
	}
	return binding.Namespace == namespace
}
