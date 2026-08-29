// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

// fleetFlags scope a fleet-wide mutation to a set of Units and carry the
// dry-run/commit controls. A selector (raw --where and/or a label shorthand) is
// required so a fleet mutation is always a deliberate, scoped action. The
// selector is ANDed with ToolchainType='Kubernetes/YAML'.
type fleetFlags struct {
	filterFlags
	cliutil.CommitFlags
}

func (f *fleetFlags) bind(cmd *cobra.Command) {
	addFilterFlags(cmd, &f.filterFlags)
	f.CommitFlags.Bind(cmd)
}

const k8sWhere = "ToolchainType = 'Kubernetes/YAML'"

// --- fleet-edit ---

func newFleetEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet-edit",
		Short: "Apply a structured RBAC edit across many Units in one server request",
		Long: `fleet-edit applies the same structured change (add/remove a verb or subject) to
every Unit matching a --where selector, in a single server-side invocation —
useful for persona roles replicated across clusters.

Dry-run by default: it reports the per-Unit changes and writes nothing. Re-run
with --commit and a --change-desc to apply. Like edit, it never bypasses
ApplyGates and does not apply to clusters.`,
	}
	cmd.AddCommand(
		newFleetVerbCmd("add-verb", true),
		newFleetVerbCmd("remove-verb", false),
		newFleetSubjectCmd("add-subject", true),
		newFleetSubjectCmd("remove-subject", false),
	)
	return cmd
}

func newFleetVerbCmd(use string, add bool) *cobra.Command {
	var f verbEditFlags
	var fl fleetFlags
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("%s on a role across every matching Unit", use),
		Example: `  cub-rbac fleet-edit ` + use + ` --where "Space.Labels.Environment = 'dev'" \
    --role-kind ClusterRole --role developer --rule 0 --verb deletecollection`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(add)
			if err != nil {
				return err
			}
			return runFleetEdit(cmd, fl, edit)
		},
	}
	f.bind(cmd)
	fl.bind(cmd)
	return cmd
}

func newFleetSubjectCmd(use string, add bool) *cobra.Command {
	var f subjectEditFlags
	var fl fleetFlags
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("%s on a binding across every matching Unit", use),
		Example: `  cub-rbac fleet-edit ` + use + ` --where "Space.Labels.Environment = 'prod'" \
    --binding-kind ClusterRoleBinding --binding viewers --subject-kind Group --subject-name oncall`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, err := f.compile(add)
			if err != nil {
				return err
			}
			return runFleetEdit(cmd, fl, edit)
		},
	}
	f.bind(cmd)
	fl.bind(cmd)
	return cmd
}

func runFleetEdit(cmd *cobra.Command, fl fleetFlags, edit rbac.EditInvocation) error {
	sel, err := fl.Predicate()
	if err != nil {
		return err
	}
	if strings.TrimSpace(sel) == "" {
		return fmt.Errorf("--where (or a label selector like --environment) is required to scope a fleet edit (use a deliberate selector, e.g. \"Space.Labels.Environment = 'prod'\")")
	}
	client, err := cub.Preflight(cmd.Context())
	if err != nil {
		return err
	}
	ch, err := commitChange(fl.CommitFlags, edit.Summary)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	inv, err := resolveEditInvocation(ctx, client, edit.Slug)
	if err != nil {
		return err
	}
	// One org-wide invocation over every matching Unit. The structured response
	// reports per-Unit results directly — no CLI output parsing.
	where := k8sWhere + " AND " + sel
	res, err := cubapi.InvokeStoredInvocation(ctx, client, inv.InvocationID,
		editParams(edit.Params), cubapi.Selector{Where: where}, ch)
	if err != nil {
		return err
	}
	changed, err := changedUnits(res)
	if err != nil {
		return err
	}
	return reportFleet(cmd, fl.CommitFlags, edit.Summary+" — fleet: "+sel, changed)
}

// --- promote (variant propagation) ---

func reportFleet(cmd *cobra.Command, c cliutil.CommitFlags, summary string, changed []string) error {
	out := cmd.OutOrStdout()
	if !c.Commit {
		fprintln(out, "Dry run — "+summary)
		if len(changed) == 0 {
			fprintln(out, "No changes: already in effect, or no matching Units.")
			return nil
		}
		fprintln(out, fmt.Sprintf("Would change %d Unit(s):", len(changed)))
		for _, u := range changed {
			fprintln(out, "  "+u)
		}
		fprintln(out, "\nRe-run with --commit --change-desc \"...\" to apply.")
		return nil
	}
	fprintln(out, "Committed — "+summary)
	if len(changed) == 0 {
		fprintln(out, "No Units changed (already in effect, or no matching Units).")
		return nil
	}
	fprintln(out, fmt.Sprintf("Changed %d Unit(s):", len(changed)))
	for _, u := range changed {
		fprintln(out, "  "+u)
	}
	return nil
}
