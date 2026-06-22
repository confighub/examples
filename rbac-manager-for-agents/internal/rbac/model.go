// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package rbac is the Kubernetes RBAC analysis engine: it parses RBAC resources
// drawn from ConfigHub Units into a typed domain model and answers Kubernetes
// authorization questions over them (matching, aggregation, effective access,
// hygiene findings).
//
// Parsing is lenient: malformed documents are skipped, never errored on — a bad
// resource in one Unit must not take down fleet-wide analysis. This file is a
// faithful Go port of the web app's model.ts; behavior is locked by tests.
package rbac

import "strings"

// ResourceOrigin records where a resource came from in ConfigHub. Clusters are
// Targets: a Unit's Target identifies the cluster it deploys to, and Units from
// many Spaces can share one cluster Target. Cluster is the Target slug when the
// Unit is bound, falling back to the Space slug for unbound ("paper cluster")
// Units; Target is set only when actually bound.
type ResourceOrigin struct {
	Cluster      string `json:"cluster"`
	Target       string `json:"target,omitempty"`
	Space        string `json:"space"`
	SpaceID      string `json:"spaceId"`
	UnitID       string `json:"unitId"`
	UnitSlug     string `json:"unitSlug"`
	ResourceName string `json:"resourceName"`
	// Canonical is true for definitions in base/policy Spaces that aren't
	// deployed anywhere — shown in the explorer but excluded from cluster
	// analysis (who-can, findings).
	Canonical bool `json:"canonical,omitempty"`
}

// PolicyRule is a single Kubernetes RBAC rule.
type PolicyRule struct {
	APIGroups       []string `json:"apiGroups"`
	Resources       []string `json:"resources"`
	Verbs           []string `json:"verbs"`
	ResourceNames   []string `json:"resourceNames"`
	NonResourceURLs []string `json:"nonResourceURLs"`
}

// RoleEntity is a parsed Role or ClusterRole.
type RoleEntity struct {
	Kind string `json:"kind"` // Role | ClusterRole
	Name string `json:"name"`
	// Namespace is set for namespaced Roles only ("" otherwise).
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels"`
	Rules     []PolicyRule      `json:"rules"`
	// AggregationSelectors holds a ClusterRole's
	// aggregationRule.clusterRoleSelectors matchLabels, if any.
	AggregationSelectors []map[string]string `json:"aggregationSelectors"`
	Origin               ResourceOrigin      `json:"origin"`
}

// Subject is a binding subject: User, Group, or ServiceAccount.
type Subject struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Namespace is set for ServiceAccount subjects; "" means none (User/Group).
	Namespace string `json:"namespace,omitempty"`
	// hasNamespace records whether the source document carried a namespace key,
	// so subjectKey can distinguish an absent namespace from an empty one.
	hasNamespace bool
}

// RoleRef references the role a binding grants.
type RoleRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// BindingEntity is a parsed RoleBinding or ClusterRoleBinding.
type BindingEntity struct {
	Kind string `json:"kind"` // RoleBinding | ClusterRoleBinding
	Name string `json:"name"`
	// Namespace is set for RoleBindings only ("" otherwise).
	Namespace string         `json:"namespace,omitempty"`
	RoleRef   RoleRef        `json:"roleRef"`
	Subjects  []Subject      `json:"subjects"`
	Origin    ResourceOrigin `json:"origin"`
}

// ServiceAccountEntity is a parsed v1 ServiceAccount.
type ServiceAccountEntity struct {
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Origin    ResourceOrigin `json:"origin"`
}

// ClusterRbac holds all RBAC entities of one cluster (one ConfigHub Space).
type ClusterRbac struct {
	Cluster         string                  `json:"cluster"`
	Roles           []*RoleEntity           `json:"roles"`
	Bindings        []*BindingEntity        `json:"bindings"`
	ServiceAccounts []*ServiceAccountEntity `json:"serviceAccounts"`
}

// FleetResource is a parsed resource document plus its ConfigHub origin. Doc is
// the decoded JSON body (typically a map[string]any).
type FleetResource struct {
	Origin ResourceOrigin
	Doc    any
}

const rbacGroup = "rbac.authorization.k8s.io"

func asRecord(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asStringArray(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func asStringMap(v any) map[string]string {
	rec, ok := asRecord(v)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(rec))
	for k, val := range rec {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func parseRules(v any) []PolicyRule {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	rules := make([]PolicyRule, 0, len(arr))
	for _, item := range arr {
		rec, ok := asRecord(item)
		if !ok {
			continue
		}
		rules = append(rules, PolicyRule{
			APIGroups:       asStringArray(rec["apiGroups"]),
			Resources:       asStringArray(rec["resources"]),
			Verbs:           asStringArray(rec["verbs"]),
			ResourceNames:   asStringArray(rec["resourceNames"]),
			NonResourceURLs: asStringArray(rec["nonResourceURLs"]),
		})
	}
	return rules
}

func parseSubjects(v any) []Subject {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	subjects := make([]Subject, 0, len(arr))
	for _, item := range arr {
		rec, ok := asRecord(item)
		if !ok {
			continue
		}
		kind, hasKind := asString(rec["kind"])
		name, hasName := asString(rec["name"])
		if !hasKind || !hasName {
			continue
		}
		ns, hasNS := asString(rec["namespace"])
		subjects = append(subjects, Subject{Kind: kind, Name: name, Namespace: ns, hasNamespace: hasNS})
	}
	return subjects
}

func parseAggregationSelectors(v any) []map[string]string {
	rec, ok := asRecord(v)
	if !ok {
		return nil
	}
	selectors, ok := rec["clusterRoleSelectors"].([]any)
	if !ok {
		return nil
	}
	var out []map[string]string
	for _, sel := range selectors {
		selRec, _ := asRecord(sel)
		matchLabels := asStringMap(selRec["matchLabels"])
		if len(matchLabels) > 0 {
			out = append(out, matchLabels)
		}
	}
	return out
}

// BuildClusterRbac indexes parsed fleet resources into per-cluster RBAC entity
// sets. Non-RBAC resources (other than ServiceAccounts) and unparseable docs are
// ignored. Entities within a cluster preserve input order.
func BuildClusterRbac(resources []FleetResource) map[string]*ClusterRbac {
	clusters := make(map[string]*ClusterRbac)

	forCluster := func(name string) *ClusterRbac {
		c, ok := clusters[name]
		if !ok {
			c = &ClusterRbac{Cluster: name}
			clusters[name] = c
		}
		return c
	}

	for _, fr := range resources {
		rec, ok := asRecord(fr.Doc)
		if !ok {
			continue
		}
		kind, hasKind := asString(rec["kind"])
		apiVersion, _ := asString(rec["apiVersion"])
		metadata, _ := asRecord(rec["metadata"])
		name, hasName := asString(metadata["name"])
		if !hasKind || !hasName {
			continue
		}
		namespace, _ := asString(metadata["namespace"])
		labels := asStringMap(metadata["labels"])
		cluster := forCluster(fr.Origin.Cluster)

		if kind == "ServiceAccount" && apiVersion == "v1" {
			ns := namespace
			if ns == "" {
				ns = "default"
			}
			cluster.ServiceAccounts = append(cluster.ServiceAccounts, &ServiceAccountEntity{
				Name: name, Namespace: ns, Origin: fr.Origin,
			})
			continue
		}
		if !strings.HasPrefix(apiVersion, rbacGroup+"/") {
			continue
		}

		switch kind {
		case "Role", "ClusterRole":
			roleNS := ""
			if kind == "Role" {
				roleNS = namespace
			}
			var aggSelectors []map[string]string
			if kind == "ClusterRole" {
				aggSelectors = parseAggregationSelectors(rec["aggregationRule"])
			}
			cluster.Roles = append(cluster.Roles, &RoleEntity{
				Kind:                 kind,
				Name:                 name,
				Namespace:            roleNS,
				Labels:               labels,
				Rules:                parseRules(rec["rules"]),
				AggregationSelectors: aggSelectors,
				Origin:               fr.Origin,
			})
		case "RoleBinding", "ClusterRoleBinding":
			roleRef, _ := asRecord(rec["roleRef"])
			refKind, hasRefKind := asString(roleRef["kind"])
			refName, hasRefName := asString(roleRef["name"])
			if !hasRefKind || !hasRefName {
				continue
			}
			bindingNS := ""
			if kind == "RoleBinding" {
				bindingNS = namespace
			}
			cluster.Bindings = append(cluster.Bindings, &BindingEntity{
				Kind:      kind,
				Name:      name,
				Namespace: bindingNS,
				RoleRef:   RoleRef{Kind: refKind, Name: refName},
				Subjects:  parseSubjects(rec["subjects"]),
				Origin:    fr.Origin,
			})
		}
	}

	return clusters
}

// ResourceMeta extracts the kind, name, and namespace from a decoded resource
// document. ok is false when the doc is not an object or lacks a kind/name.
func ResourceMeta(doc any) (kind, name, namespace string, ok bool) {
	rec, isRec := asRecord(doc)
	if !isRec {
		return "", "", "", false
	}
	kind, _ = asString(rec["kind"])
	md, _ := asRecord(rec["metadata"])
	name, _ = asString(md["name"])
	namespace, _ = asString(md["namespace"])
	return kind, name, namespace, kind != "" && name != ""
}

// SubjectKey is the stable display form of a subject: "kind:name" or
// "kind:ns/name" when the subject carries a namespace.
func SubjectKey(s Subject) string {
	if s.hasNamespace {
		return s.Kind + ":" + s.Namespace + "/" + s.Name
	}
	return s.Kind + ":" + s.Name
}
