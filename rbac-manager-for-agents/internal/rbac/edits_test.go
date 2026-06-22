// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package rbac

import (
	"strings"
	"testing"
)

func TestCompileAddVerb(t *testing.T) {
	e := CompileAddVerb("ClusterRole", "viewer", 0, "get")
	want := `(select(.kind == "ClusterRole" and .metadata.name == "viewer").rules[0].verbs) |= ((. + ["get"]) | unique)`
	if e.Expr != want {
		t.Errorf("expr =\n %s\nwant\n %s", e.Expr, want)
	}
	if !strings.Contains(e.Summary, "Add verb") {
		t.Errorf("summary = %q", e.Summary)
	}
}

func TestCompileRemoveVerb(t *testing.T) {
	e := CompileRemoveVerb("Role", "reader", 2, "delete")
	want := `(select(.kind == "Role" and .metadata.name == "reader").rules[2].verbs) |= (. - ["delete"])`
	if e.Expr != want {
		t.Errorf("expr =\n %s\nwant\n %s", e.Expr, want)
	}
}

func TestCompileAddSubject_Group(t *testing.T) {
	e := CompileAddSubject("ClusterRoleBinding", "devs-bind", "Group", "devs", "")
	want := `select(.kind == "ClusterRoleBinding" and .metadata.name == "devs-bind").subjects += [{"kind": "Group", "name": "devs", "apiGroup": "rbac.authorization.k8s.io"}]`
	if e.Expr != want {
		t.Errorf("expr =\n %s\nwant\n %s", e.Expr, want)
	}
}

func TestCompileAddSubject_ServiceAccount(t *testing.T) {
	e := CompileAddSubject("RoleBinding", "rb", "ServiceAccount", "ci", "apps")
	want := `select(.kind == "RoleBinding" and .metadata.name == "rb").subjects += [{"kind": "ServiceAccount", "name": "ci", "namespace": "apps"}]`
	if e.Expr != want {
		t.Errorf("expr =\n %s\nwant\n %s", e.Expr, want)
	}
}

func TestCompileRemoveSubject_ServiceAccount(t *testing.T) {
	e := CompileRemoveSubject("RoleBinding", "rb", "ServiceAccount", "ci", "apps")
	want := `select(.kind == "RoleBinding" and .metadata.name == "rb").subjects |= map(select(((.kind == "ServiceAccount") and (.name == "ci") and (.namespace == "apps")) | not))`
	if e.Expr != want {
		t.Errorf("expr =\n %s\nwant\n %s", e.Expr, want)
	}
}

func TestCompileRemoveSubject_User(t *testing.T) {
	e := CompileRemoveSubject("ClusterRoleBinding", "crb", "User", "alice", "")
	want := `select(.kind == "ClusterRoleBinding" and .metadata.name == "crb").subjects |= map(select(((.kind == "User") and (.name == "alice")) | not))`
	if e.Expr != want {
		t.Errorf("expr =\n %s\nwant\n %s", e.Expr, want)
	}
}
