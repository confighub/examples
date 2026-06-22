// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

// verbEditFlags bind the inputs for add-verb / remove-verb and compile them to a
// CompiledEdit. Shared by the single-Unit `edit` and the fleet-wide `fleet-edit`
// commands.
type verbEditFlags struct {
	roleKind string
	role     string
	verb     string
	rule     int
}

func (f *verbEditFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.roleKind, "role-kind", "", "Role or ClusterRole (required)")
	cmd.Flags().StringVar(&f.role, "role", "", "role name (required)")
	cmd.Flags().IntVar(&f.rule, "rule", 0, "rule index within the role")
	cmd.Flags().StringVar(&f.verb, "verb", "", "verb (required)")
	_ = cmd.MarkFlagRequired("role-kind")
	_ = cmd.MarkFlagRequired("role")
	_ = cmd.MarkFlagRequired("verb")
}

func (f *verbEditFlags) compile(add bool) (rbac.CompiledEdit, error) {
	rk, err := normalizeRoleKind(f.roleKind)
	if err != nil {
		return rbac.CompiledEdit{}, err
	}
	if err := validateName("role", f.role); err != nil {
		return rbac.CompiledEdit{}, err
	}
	if err := validateName("verb", f.verb); err != nil {
		return rbac.CompiledEdit{}, err
	}
	if f.rule < 0 {
		return rbac.CompiledEdit{}, fmt.Errorf("--rule must be >= 0")
	}
	if add {
		return rbac.CompileAddVerb(rk, f.role, f.rule, f.verb), nil
	}
	return rbac.CompileRemoveVerb(rk, f.role, f.rule, f.verb), nil
}

// subjectEditFlags bind the inputs for add-subject / remove-subject.
type subjectEditFlags struct {
	bindingKind      string
	binding          string
	subjectKind      string
	subjectName      string
	subjectNamespace string
}

func (f *subjectEditFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.bindingKind, "binding-kind", "", "RoleBinding or ClusterRoleBinding (required)")
	cmd.Flags().StringVar(&f.binding, "binding", "", "binding name (required)")
	cmd.Flags().StringVar(&f.subjectKind, "subject-kind", "", "User, Group, or ServiceAccount (required)")
	cmd.Flags().StringVar(&f.subjectName, "subject-name", "", "subject name (required)")
	cmd.Flags().StringVar(&f.subjectNamespace, "subject-namespace", "", "subject namespace (required for ServiceAccount)")
	_ = cmd.MarkFlagRequired("binding-kind")
	_ = cmd.MarkFlagRequired("binding")
	_ = cmd.MarkFlagRequired("subject-kind")
	_ = cmd.MarkFlagRequired("subject-name")
}

func (f *subjectEditFlags) compile(add bool) (rbac.CompiledEdit, error) {
	bk, err := normalizeBindingKind(f.bindingKind)
	if err != nil {
		return rbac.CompiledEdit{}, err
	}
	sk, err := normalizeSubjectKind(f.subjectKind)
	if err != nil {
		return rbac.CompiledEdit{}, err
	}
	if err := validateName("binding", f.binding); err != nil {
		return rbac.CompiledEdit{}, err
	}
	if err := validateName("subject-name", f.subjectName); err != nil {
		return rbac.CompiledEdit{}, err
	}
	if f.subjectNamespace != "" {
		if err := validateName("subject-namespace", f.subjectNamespace); err != nil {
			return rbac.CompiledEdit{}, err
		}
	}
	if sk == "ServiceAccount" && f.subjectNamespace == "" {
		return rbac.CompiledEdit{}, fmt.Errorf("--subject-namespace is required for a ServiceAccount subject")
	}
	if add {
		return rbac.CompileAddSubject(bk, f.binding, sk, f.subjectName, f.subjectNamespace), nil
	}
	return rbac.CompileRemoveSubject(bk, f.binding, sk, f.subjectName, f.subjectNamespace), nil
}

func normalizeRoleKind(s string) (string, error) {
	switch strings.ToLower(s) {
	case "role":
		return "Role", nil
	case "clusterrole":
		return "ClusterRole", nil
	default:
		return "", fmt.Errorf("invalid --role-kind %q: use Role or ClusterRole", s)
	}
}

func normalizeBindingKind(s string) (string, error) {
	switch strings.ToLower(s) {
	case "rolebinding":
		return "RoleBinding", nil
	case "clusterrolebinding":
		return "ClusterRoleBinding", nil
	default:
		return "", fmt.Errorf("invalid --binding-kind %q: use RoleBinding or ClusterRoleBinding", s)
	}
}

// validateName rejects empty values and double quotes, which would break the
// generated yq expression (and could be an injection vector).
func validateName(field, v string) error {
	if v == "" {
		return fmt.Errorf("--%s is required", field)
	}
	if strings.Contains(v, `"`) {
		return fmt.Errorf("--%s %q must not contain a double quote", field, v)
	}
	return nil
}
