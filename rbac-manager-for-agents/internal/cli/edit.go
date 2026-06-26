// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Make guardrailed structured edits to a single RBAC Unit",
		Long: `edit applies structured changes to a single RBAC Unit's config — adding or
removing a verb on a role rule, or a subject on a binding — compiled to a
server-side yq edit that modifies the literal YAML in place.

Edits are dry-run by default: the diff is previewed and nothing is written.
Re-run with --commit and a --change-desc to apply. Edits never bypass
ApplyGates; gates and warnings are evaluated server-side as usual, and applying
the resulting revision is a separate step (cub unit apply).`,
	}
	cmd.AddCommand(
		newEditInstallCmd(),
		newAddVerbCmd(),
		newRemoveVerbCmd(),
		newAddSubjectCmd(),
		newRemoveSubjectCmd(),
	)
	return cmd
}

func newAddVerbCmd() *cobra.Command {
	var f verbEditFlags
	var c commitFlags
	cmd := &cobra.Command{
		Use:   "add-verb <space>/<unit>",
		Short: "Add a verb to a role's rule (idempotent)",
		Example: `  cub-rbac edit add-verb prod/rbac --role-kind ClusterRole --role viewer --rule 0 --verb get
  cub-rbac edit add-verb prod/rbac --role-kind ClusterRole --role viewer --rule 0 --verb get --commit --change-desc "grant viewer get on pods (OPS-12)"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(true)
			if err != nil {
				return err
			}
			return runEdit(cmd, args[0], edit, c)
		},
	}
	f.bind(cmd)
	addCommitFlags(cmd, &c)
	return cmd
}

func newRemoveVerbCmd() *cobra.Command {
	var f verbEditFlags
	var c commitFlags
	cmd := &cobra.Command{
		Use:     "remove-verb <space>/<unit>",
		Short:   "Remove a verb from a role's rule",
		Example: `  cub-rbac edit remove-verb prod/rbac --role-kind ClusterRole --role admin --rule 0 --verb '*' --commit --change-desc "drop wildcard verb (OPS-12)"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(false)
			if err != nil {
				return err
			}
			return runEdit(cmd, args[0], edit, c)
		},
	}
	f.bind(cmd)
	addCommitFlags(cmd, &c)
	return cmd
}

func newAddSubjectCmd() *cobra.Command {
	var f subjectEditFlags
	var c commitFlags
	cmd := &cobra.Command{
		Use:     "add-subject <space>/<unit>",
		Short:   "Add a subject to a binding",
		Example: `  cub-rbac edit add-subject prod/rbac --binding-kind ClusterRoleBinding --binding viewers --subject-kind Group --subject-name oncall --commit --change-desc "add oncall to viewers (OPS-12)"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(true)
			if err != nil {
				return err
			}
			return runEdit(cmd, args[0], edit, c)
		},
	}
	f.bind(cmd)
	addCommitFlags(cmd, &c)
	return cmd
}

func newRemoveSubjectCmd() *cobra.Command {
	var f subjectEditFlags
	var c commitFlags
	cmd := &cobra.Command{
		Use:     "remove-subject <space>/<unit>",
		Short:   "Remove a subject from a binding",
		Example: `  cub-rbac edit remove-subject prod/rbac --binding-kind ClusterRoleBinding --binding breakglass --subject-kind Group --subject-name everyone --commit --change-desc "revoke everyone from breakglass (OPS-12)"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(false)
			if err != nil {
				return err
			}
			return runEdit(cmd, args[0], edit, c)
		},
	}
	f.bind(cmd)
	addCommitFlags(cmd, &c)
	return cmd
}

// runEdit previews (dry-run) or commits an edit against one Unit by invoking the
// shared, parameterized set-yq Invocation (resolved cross-space from the edit
// library Space) with the edit's parameter values.
func runEdit(cmd *cobra.Command, unitRef string, edit rbac.EditInvocation, c commitFlags) error {
	space, unit, err := parseUnitRef(unitRef)
	if err != nil {
		return err
	}
	if err := cub.Preflight(cmd.Context()); err != nil {
		return err
	}
	base := []string{"invocation", "invoke", "set", rbac.EditLibrarySpace + "/" + edit.Slug,
		"--space", space, "--unit", unit, "-o", "mutations"}
	for _, p := range edit.Params {
		base = append(base, "--param", p)
	}
	return runMutation(cmd, base, c, edit.Summary)
}
