// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Structured RBAC edits compiled to server-side yq (set-yq) expressions. The
// expressions modify the literal YAML in place, preserving comments and
// formatting — never re-serialized client-side. Faithful Go port of the web
// app's edits.ts.

package rbac

import "fmt"

// CompiledEdit is a yq expression plus a human summary used as the default
// change description.
type CompiledEdit struct {
	Expr    string
	Summary string
}

// CompileAddVerb adds a verb to a role's rule, idempotently (unique).
func CompileAddVerb(roleKind, roleName string, ruleIdx int, verb string) CompiledEdit {
	sel := fmt.Sprintf(`select(.kind == "%s" and .metadata.name == "%s").rules[%d].verbs`, roleKind, roleName, ruleIdx)
	return CompiledEdit{
		// `unique` makes the edit idempotent: re-applying never duplicates.
		Expr:    fmt.Sprintf(`(%s) |= ((. + ["%s"]) | unique)`, sel, verb),
		Summary: fmt.Sprintf("Add verb %q to %s %s rule %d", verb, roleKind, roleName, ruleIdx),
	}
}

// CompileRemoveVerb removes a verb from a role's rule.
func CompileRemoveVerb(roleKind, roleName string, ruleIdx int, verb string) CompiledEdit {
	sel := fmt.Sprintf(`select(.kind == "%s" and .metadata.name == "%s").rules[%d].verbs`, roleKind, roleName, ruleIdx)
	return CompiledEdit{
		Expr:    fmt.Sprintf(`(%s) |= (. - ["%s"])`, sel, verb),
		Summary: fmt.Sprintf("Remove verb %q from %s %s rule %d", verb, roleKind, roleName, ruleIdx),
	}
}

// CompileAddSubject adds a subject to a binding.
func CompileAddSubject(bindingKind, bindingName, subjectKind, subjectName, subjectNamespace string) CompiledEdit {
	var subject string
	if subjectKind == "ServiceAccount" {
		subject = fmt.Sprintf(`{"kind": "ServiceAccount", "name": "%s", "namespace": "%s"}`, subjectName, subjectNamespace)
	} else {
		subject = fmt.Sprintf(`{"kind": "%s", "name": "%s", "apiGroup": "rbac.authorization.k8s.io"}`, subjectKind, subjectName)
	}
	return CompiledEdit{
		Expr:    fmt.Sprintf(`select(.kind == "%s" and .metadata.name == "%s").subjects += [%s]`, bindingKind, bindingName, subject),
		Summary: fmt.Sprintf("Add %s %q to %s %s", subjectKind, subjectName, bindingKind, bindingName),
	}
}

// CompileRemoveSubject removes a subject from a binding.
func CompileRemoveSubject(bindingKind, bindingName, subjectKind, subjectName, subjectNamespace string) CompiledEdit {
	var match string
	if subjectKind == "ServiceAccount" {
		match = fmt.Sprintf(`(.kind == "ServiceAccount") and (.name == "%s") and (.namespace == "%s")`, subjectName, subjectNamespace)
	} else {
		match = fmt.Sprintf(`(.kind == "%s") and (.name == "%s")`, subjectKind, subjectName)
	}
	return CompiledEdit{
		Expr:    fmt.Sprintf(`select(.kind == "%s" and .metadata.name == "%s").subjects |= map(select((%s) | not))`, bindingKind, bindingName, match),
		Summary: fmt.Sprintf("Remove %s %q from %s %s", subjectKind, subjectName, bindingKind, bindingName),
	}
}
