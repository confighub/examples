// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Structured RBAC edits expressed as stored, parameterized set-yq Invocations.
// The fixed yq templates live in ConfigHub (created at setup, see
// `cub-rbac edit install`); each edit supplies only the variable values as
// parameters. The server expands the templated arguments and applies the change
// to the literal YAML in place — preserving comments and formatting, never
// re-serialized client-side. The web app (../rbac-manager) references the same
// Invocations by the same slugs.

package rbac

import (
	"fmt"
	"strconv"
)

// EditLibrarySpace is the Space that holds the shared edit Invocations. Created
// (idempotently) by `cub-rbac edit install` and referenced cross-space as
// "<EditLibrarySpace>/<slug>" when invoking.
const EditLibrarySpace = "rbac-edits"

// Invocation slugs (shared with the web app).
const (
	InvAddVerb       = "rbac-add-verb"
	InvRemoveVerb    = "rbac-remove-verb"
	InvAddSubject    = "rbac-add-subject"
	InvRemoveSubject = "rbac-remove-subject"
)

// EditInvocation is a reference to a stored edit Invocation plus the parameter
// values to supply (as "name=value" entries for `--param`) and a human summary
// used as the default change description.
type EditInvocation struct {
	Slug    string
	Params  []string
	Summary string
}

// AddVerb adds a verb to a role's rule, idempotently (the template uses `unique`).
func AddVerb(roleKind, roleName string, ruleIdx int, verb string) EditInvocation {
	return EditInvocation{
		Slug: InvAddVerb,
		Params: []string{
			"roleKind=" + roleKind,
			"roleName=" + roleName,
			"ruleIdx=" + strconv.Itoa(ruleIdx),
			"verb=" + verb,
		},
		Summary: fmt.Sprintf("Add verb %q to %s %s rule %d", verb, roleKind, roleName, ruleIdx),
	}
}

// RemoveVerb removes a verb from a role's rule.
func RemoveVerb(roleKind, roleName string, ruleIdx int, verb string) EditInvocation {
	return EditInvocation{
		Slug: InvRemoveVerb,
		Params: []string{
			"roleKind=" + roleKind,
			"roleName=" + roleName,
			"ruleIdx=" + strconv.Itoa(ruleIdx),
			"verb=" + verb,
		},
		Summary: fmt.Sprintf("Remove verb %q from %s %s rule %d", verb, roleKind, roleName, ruleIdx),
	}
}

// AddSubject adds a subject to a binding. The subject's structural difference —
// ServiceAccount carries a namespace, User/Group carries an apiGroup — is
// expressed by which of the two fields is non-empty; the template drops the
// empty one.
func AddSubject(bindingKind, bindingName, subjectKind, subjectName, subjectNamespace string) EditInvocation {
	apiGroup := "rbac.authorization.k8s.io"
	if subjectKind == "ServiceAccount" {
		apiGroup = ""
	}
	return EditInvocation{
		Slug: InvAddSubject,
		Params: []string{
			"bindingKind=" + bindingKind,
			"bindingName=" + bindingName,
			"subjectKind=" + subjectKind,
			"subjectName=" + subjectName,
			"subjectNamespace=" + subjectNamespace,
			"subjectApiGroup=" + apiGroup,
		},
		Summary: fmt.Sprintf("Add %s %q to %s %s", subjectKind, subjectName, bindingKind, bindingName),
	}
}

// RemoveSubject removes a subject from a binding. The match is uniform across
// subject kinds: kind + name + namespace (defaulting a missing namespace to "").
func RemoveSubject(bindingKind, bindingName, subjectKind, subjectName, subjectNamespace string) EditInvocation {
	return EditInvocation{
		Slug: InvRemoveSubject,
		Params: []string{
			"bindingKind=" + bindingKind,
			"bindingName=" + bindingName,
			"subjectKind=" + subjectKind,
			"subjectName=" + subjectName,
			"subjectNamespace=" + subjectNamespace,
		},
		Summary: fmt.Sprintf("Remove %s %q from %s %s", subjectKind, subjectName, bindingKind, bindingName),
	}
}

// EditParam declares one parameter of an edit Invocation.
type EditParam struct {
	Name     string
	DataType string // "string" or "int"
}

// EditInvocationSpec is the full definition of a stored edit Invocation, used by
// `cub-rbac edit install` to create it. The YQExpression references the declared
// parameters as $params.<name> (set-yq's param binding); ints are coerced in the
// expression with `| tonumber`.
type EditInvocationSpec struct {
	Slug         string
	YQExpression string
	Parameters   []EditParam
}

// EditInvocationSpecs is the canonical set of edit Invocations. The yq templates
// are verified to preserve comments and to handle the ServiceAccount vs
// User/Group subject shape without branching (see edit install).
var EditInvocationSpecs = []EditInvocationSpec{
	{
		Slug:         InvAddVerb,
		YQExpression: `(select(.kind == $params.roleKind and .metadata.name == $params.roleName).rules[$params.ruleIdx | tonumber].verbs) |= ((. + [$params.verb]) | unique)`,
		Parameters:   []EditParam{{"roleKind", "string"}, {"roleName", "string"}, {"ruleIdx", "int"}, {"verb", "string"}},
	},
	{
		Slug:         InvRemoveVerb,
		YQExpression: `(select(.kind == $params.roleKind and .metadata.name == $params.roleName).rules[$params.ruleIdx | tonumber].verbs) |= (. - [$params.verb])`,
		Parameters:   []EditParam{{"roleKind", "string"}, {"roleName", "string"}, {"ruleIdx", "int"}, {"verb", "string"}},
	},
	{
		Slug:         InvAddSubject,
		YQExpression: `select(.kind == $params.bindingKind and .metadata.name == $params.bindingName).subjects += [ {"kind": $params.subjectKind, "name": $params.subjectName, "namespace": $params.subjectNamespace, "apiGroup": $params.subjectApiGroup} | with_entries(select(.value != "")) ]`,
		Parameters:   []EditParam{{"bindingKind", "string"}, {"bindingName", "string"}, {"subjectKind", "string"}, {"subjectName", "string"}, {"subjectNamespace", "string"}, {"subjectApiGroup", "string"}},
	},
	{
		Slug:         InvRemoveSubject,
		YQExpression: `select(.kind == $params.bindingKind and .metadata.name == $params.bindingName).subjects |= map(select((.kind == $params.subjectKind and .name == $params.subjectName and (.namespace // "") == $params.subjectNamespace) | not))`,
		Parameters:   []EditParam{{"bindingKind", "string"}, {"bindingName", "string"}, {"subjectKind", "string"}, {"subjectName", "string"}, {"subjectNamespace", "string"}},
	},
}
