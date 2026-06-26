// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package rbac

import (
	"reflect"
	"strings"
	"testing"
)

func TestAddVerb(t *testing.T) {
	e := AddVerb("ClusterRole", "viewer", 0, "get")
	if e.Slug != InvAddVerb {
		t.Errorf("slug = %q, want %q", e.Slug, InvAddVerb)
	}
	want := []string{"roleKind=ClusterRole", "roleName=viewer", "ruleIdx=0", "verb=get"}
	if !reflect.DeepEqual(e.Params, want) {
		t.Errorf("params = %v, want %v", e.Params, want)
	}
	if !strings.Contains(e.Summary, "Add verb") {
		t.Errorf("summary = %q", e.Summary)
	}
}

func TestRemoveVerb(t *testing.T) {
	e := RemoveVerb("Role", "reader", 2, "delete")
	if e.Slug != InvRemoveVerb {
		t.Errorf("slug = %q", e.Slug)
	}
	want := []string{"roleKind=Role", "roleName=reader", "ruleIdx=2", "verb=delete"}
	if !reflect.DeepEqual(e.Params, want) {
		t.Errorf("params = %v, want %v", e.Params, want)
	}
}

// add-subject: a Group/User carries an apiGroup and an empty namespace; a
// ServiceAccount carries a namespace and an empty apiGroup. The template drops
// the empty field server-side.
func TestAddSubject_Group(t *testing.T) {
	e := AddSubject("ClusterRoleBinding", "devs-bind", "Group", "devs", "")
	want := []string{
		"bindingKind=ClusterRoleBinding", "bindingName=devs-bind",
		"subjectKind=Group", "subjectName=devs",
		"subjectNamespace=", "subjectApiGroup=rbac.authorization.k8s.io",
	}
	if !reflect.DeepEqual(e.Params, want) {
		t.Errorf("params = %v, want %v", e.Params, want)
	}
}

func TestAddSubject_ServiceAccount(t *testing.T) {
	e := AddSubject("RoleBinding", "rb", "ServiceAccount", "ci", "apps")
	want := []string{
		"bindingKind=RoleBinding", "bindingName=rb",
		"subjectKind=ServiceAccount", "subjectName=ci",
		"subjectNamespace=apps", "subjectApiGroup=",
	}
	if !reflect.DeepEqual(e.Params, want) {
		t.Errorf("params = %v, want %v", e.Params, want)
	}
}

func TestRemoveSubject(t *testing.T) {
	e := RemoveSubject("RoleBinding", "rb", "ServiceAccount", "ci", "apps")
	if e.Slug != InvRemoveSubject {
		t.Errorf("slug = %q", e.Slug)
	}
	want := []string{
		"bindingKind=RoleBinding", "bindingName=rb",
		"subjectKind=ServiceAccount", "subjectName=ci", "subjectNamespace=apps",
	}
	if !reflect.DeepEqual(e.Params, want) {
		t.Errorf("params = %v, want %v", e.Params, want)
	}
}

// TestEditInvocationSpecs_Cover asserts every builder slug has a matching stored
// spec, and that each spec declares a parameter for every value the builder
// supplies (so an invoke never sends an undeclared parameter).
func TestEditInvocationSpecs_Cover(t *testing.T) {
	specBySlug := map[string]EditInvocationSpec{}
	for _, s := range EditInvocationSpecs {
		specBySlug[s.Slug] = s
	}
	cases := []EditInvocation{
		AddVerb("Role", "r", 0, "get"),
		RemoveVerb("Role", "r", 0, "get"),
		AddSubject("RoleBinding", "b", "ServiceAccount", "ci", "apps"),
		RemoveSubject("RoleBinding", "b", "User", "alice", ""),
	}
	for _, c := range cases {
		spec, ok := specBySlug[c.Slug]
		if !ok {
			t.Errorf("no EditInvocationSpec for slug %q", c.Slug)
			continue
		}
		declared := map[string]bool{}
		for _, p := range spec.Parameters {
			declared[p.Name] = true
		}
		for _, kv := range c.Params {
			name := strings.SplitN(kv, "=", 2)[0]
			if !declared[name] {
				t.Errorf("%s supplies undeclared parameter %q", c.Slug, name)
			}
		}
	}
}
