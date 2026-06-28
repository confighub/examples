// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

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
	var c cliutil.CommitFlags
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
	c.Bind(cmd)
	return cmd
}

func newRemoveVerbCmd() *cobra.Command {
	var f verbEditFlags
	var c cliutil.CommitFlags
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
	c.Bind(cmd)
	return cmd
}

func newAddSubjectCmd() *cobra.Command {
	var f subjectEditFlags
	var c cliutil.CommitFlags
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
	c.Bind(cmd)
	return cmd
}

func newRemoveSubjectCmd() *cobra.Command {
	var f subjectEditFlags
	var c cliutil.CommitFlags
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
	c.Bind(cmd)
	return cmd
}

// runEdit previews (dry-run) or commits an edit against one Unit by invoking the
// shared, parameterized set-yq Invocation (resolved from the edit library Space)
// over that single Unit with the edit's parameter values.
func runEdit(cmd *cobra.Command, unitRef string, edit rbac.EditInvocation, c cliutil.CommitFlags) error {
	space, unit, err := parseUnitRef(unitRef)
	if err != nil {
		return err
	}
	client, err := cub.Preflight(cmd.Context())
	if err != nil {
		return err
	}
	ch, err := commitChange(c, edit.Summary)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	sp, err := cubapi.ResolveSpace(ctx, client, space)
	if err != nil {
		return err
	}
	inv, err := resolveEditInvocation(ctx, client, edit.Slug)
	if err != nil {
		return err
	}

	where := fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", sp.SpaceID.String(), unit)
	res, err := cubapi.InvokeStoredInvocation(ctx, client, inv.InvocationID,
		editParams(edit.Params), cubapi.Selector{Where: where}, ch)
	if err != nil {
		return err
	}
	changed, err := changedUnits(res)
	if err != nil {
		return err
	}
	return reportFleet(cmd, c, edit.Summary, changed)
}

// resolveEditInvocation finds a stored edit Invocation by slug in the edit
// library Space, with a remediation hint when the library is not installed.
func resolveEditInvocation(ctx context.Context, client *cubapi.Client, slug string) (*goclientnew.Invocation, error) {
	lib, err := cubapi.ResolveSpace(ctx, client, rbac.EditLibrarySpace)
	if err != nil {
		return nil, fmt.Errorf("edit library not installed — run `cub-rbac edit install`: %w", err)
	}
	inv, err := cubapi.ResolveInvocation(ctx, client, lib.SpaceID, slug)
	if err != nil {
		return nil, fmt.Errorf("edit library not installed — run `cub-rbac edit install`: %w", err)
	}
	return inv, nil
}
