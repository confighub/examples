// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Fleet-wide effective-access queries: "who can VERB RESOURCE, on which cluster,
// granted by what?" and the inverse subject view. Faithful Go port of the web
// app's whocan.ts.

package rbac

import "sort"

// Grant is one subject's access to the queried resource, with full provenance.
type Grant struct {
	Cluster        string
	Subject        Subject
	SubjectKey     string
	Role           *RoleEntity // resolved role, nil when not in the snapshot
	RoleRefName    string
	ViaBuiltinRole bool // roleRef is a Kubernetes builtin not stored in ConfigHub
	Binding        *BindingEntity
	Scope          string // namespace the grant is effective in; "" = cluster-wide
}

// builtinMatches: cluster-admin grants everything; for other builtins we have no
// manifest, so only cluster-admin is treated as matching arbitrary queries.
// Bindings to admin/edit/view/system:* surface in audits but not who-can.
func builtinMatches(roleRefName string) bool {
	return roleRefName == "cluster-admin"
}

// WhoCanInCluster returns all grants matching the query in one cluster.
func WhoCanInCluster(cluster *ClusterRbac, query AccessQuery) []Grant {
	var grants []Grant
	for _, binding := range cluster.Bindings {
		if !BindingScopeMatches(binding, query.Namespace) {
			continue
		}
		role := ResolveRoleRef(binding, cluster)
		matches := false
		viaBuiltin := false
		if role != nil {
			for _, rule := range EffectiveRules(role, cluster) {
				if RuleMatches(rule, query) {
					matches = true
					break
				}
			}
		} else if IsBuiltinRoleName(binding.RoleRef.Name) {
			matches = builtinMatches(binding.RoleRef.Name)
			viaBuiltin = matches
		}
		if !matches {
			continue
		}
		scope := ""
		if binding.Kind == "RoleBinding" {
			scope = binding.Namespace
		}
		for _, subject := range binding.Subjects {
			grants = append(grants, Grant{
				Cluster:        cluster.Cluster,
				Subject:        subject,
				SubjectKey:     SubjectKey(subject),
				Role:           role,
				RoleRefName:    binding.RoleRef.Name,
				ViaBuiltinRole: viaBuiltin,
				Binding:        binding,
				Scope:          scope,
			})
		}
	}
	return grants
}

// WhoCan returns all grants matching the query across the fleet.
func WhoCan(clusters map[string]*ClusterRbac, query AccessQuery) []Grant {
	var grants []Grant
	for _, c := range clusters {
		grants = append(grants, WhoCanInCluster(c, query)...)
	}
	return grants
}

// SubjectGrant is one role held by a subject in one cluster.
type SubjectGrant struct {
	Cluster     string
	Binding     *BindingEntity
	Role        *RoleEntity
	RoleRefName string
	Scope       string // "" = cluster-wide
}

// SubjectRef identifies a subject for the inverse query.
type SubjectRef struct {
	Kind      string
	Name      string
	Namespace string
}

// SubjectAccess is the inverse view: every role a subject holds, fleet-wide.
func SubjectAccess(clusters map[string]*ClusterRbac, subject SubjectRef) []SubjectGrant {
	var out []SubjectGrant
	for _, cluster := range clusters {
		for _, binding := range cluster.Bindings {
			held := false
			for _, s := range binding.Subjects {
				if s.Kind == subject.Kind && s.Name == subject.Name &&
					(subject.Kind != "ServiceAccount" || s.Namespace == subject.Namespace) {
					held = true
					break
				}
			}
			if !held {
				continue
			}
			scope := ""
			if binding.Kind == "RoleBinding" {
				scope = binding.Namespace
			}
			out = append(out, SubjectGrant{
				Cluster:     cluster.Cluster,
				Binding:     binding,
				Role:        ResolveRoleRef(binding, cluster),
				RoleRefName: binding.RoleRef.Name,
				Scope:       scope,
			})
		}
	}
	return out
}

// AllSubjects returns every distinct subject appearing in any binding, sorted by
// subject key.
func AllSubjects(clusters map[string]*ClusterRbac) []Subject {
	byKey := map[string]Subject{}
	for _, cluster := range clusters {
		for _, binding := range cluster.Bindings {
			for _, s := range binding.Subjects {
				byKey[SubjectKey(s)] = s
			}
		}
	}
	out := make([]Subject, 0, len(byKey))
	for _, s := range byKey {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return SubjectKey(out[i]) < SubjectKey(out[j])
	})
	return out
}
