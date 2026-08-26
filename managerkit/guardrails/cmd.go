// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package guardrails

// The command surface over a Pack. pack.go and query.go are API-shaped and have
// no CLI in them; this file is the CLI half, and is what would move to the SDK's
// command-line layer rather than its API layer.

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/managerkit/clikit"
)

// Preflight verifies a tool's ConfigHub session and returns the client. Each
// tool has one; the commands take it rather than reaching for a global.
type Preflight func(context.Context) (*cubapi.Client, error)

// InstallCmd is `guardrails install` for this pack.
func (p Pack) InstallCmd(preflight Preflight) *cobra.Command {
	var policySpace, whereSpace, output string
	var commit bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the guardrail pack and wire in-scope Spaces (dry-run by default)",
		Long: `install creates the policy Space, the Warn=true guardrail Triggers, and the
shared Trigger Filter, then points each in-scope Space's TriggerFilterID at it.

The Space and the Filter are shared with the sibling tools. A Space has exactly
one TriggerFilterID, so a Filter per tool could not compose: the first pack
installed would claim a Space and the rest would find it taken. Instead every
pack's Triggers carry a common label, one Filter selects them, and installing
another pack adds its rules to every Space already wired.

Dry-run by default: it prints the plan and changes nothing. Re-run with --commit
to apply. Spaces that already select Triggers another way are reported, not
modified. Narrow which Spaces get wired with --where-space.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := preflight(cmd.Context())
			if err != nil {
				return err
			}
			plan, err := p.BuildPlan(cmd.Context(), client, policySpace, whereSpace)
			if err != nil {
				return err
			}
			if commit {
				out := cmd.OutOrStdout()
				if err := p.Execute(cmd.Context(), client, policySpace, plan,
					func(line string) { clikit.Fprintln(out, line) }); err != nil {
					return err
				}
				plan.Committed = true
			}
			if output == clikit.OutputTable {
				PrintPlan(cmd, plan)
				return nil
			}
			return clikit.PrintJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&policySpace, "policy-space", DefaultPolicySpace,
		"Space holding the guardrail Triggers and the shared Filter; every tool defaults to the same one")
	cmd.Flags().StringVar(&whereSpace, "where-space", "", "ConfigHub filter over Spaces to narrow which Spaces get wired")
	cmd.Flags().BoolVar(&commit, "commit", false, "apply the plan (default is dry-run)")
	clikit.AddOutputFlag(cmd, &output)
	return cmd
}

// StatusCmd is `guardrails status`: which Units the Triggers have marked.
func (p Pack) StatusCmd(preflight Preflight) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Units carrying ApplyWarnings or ApplyGates",
		Long: `status lists the Units a Trigger has marked — ApplyWarnings from an advisory
rule, ApplyGates from one promoted to blocking.

It reports what any Trigger attached, not only this pack's: a Unit does not
record which rule marked it, and reporting only this pack's rules would hide a
Unit that is blocked for some other reason.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := preflight(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := Status(cmd.Context(), client)
			if err != nil {
				return err
			}
			if output == clikit.OutputTable {
				PrintStatus(cmd, rows)
				return nil
			}
			return clikit.PrintJSON(cmd.OutOrStdout(), rows)
		},
	}
	clikit.AddOutputFlag(cmd, &output)
	return cmd
}

// PrintPlan renders an install plan as a table.
func PrintPlan(cmd *cobra.Command, plan Plan) {
	out := cmd.OutOrStdout()
	verb := "Plan (dry-run)"
	if plan.Committed {
		verb = "Applied"
	}
	clikit.Fprintln(out, fmt.Sprintf("%s — policy pack %q, filter %q", verb, plan.PolicySpace, plan.Filter))
	clikit.Fprintln(out, "  triggers: "+strings.Join(plan.Triggers, ", "))
	clikit.Fprintln(out, fmt.Sprintf("  spaces to wire (%d): %s", len(plan.Wire), strings.Join(plan.Wire, ", ")))
	if len(plan.AlreadyWired) > 0 {
		clikit.Fprintln(out, fmt.Sprintf("  already wired (%d): %s", len(plan.AlreadyWired), strings.Join(plan.AlreadyWired, ", ")))
	}
	if len(plan.Skipped) > 0 {
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "  SKIPPED\tREASON")
		for _, s := range plan.Skipped {
			fmt.Fprintf(tw, "  %s\t%s\n", s.Space, s.Reason)
		}
		_ = tw.Flush()
	}
	if !plan.Committed {
		clikit.Fprintln(out, "\nNothing changed. Re-run with --commit to apply.")
	}
}

// PrintStatus renders the marked Units as a table.
func PrintStatus(cmd *cobra.Command, rows []StatusRow) {
	out := cmd.OutOrStdout()
	if len(rows) == 0 {
		clikit.Fprintln(out, "No Units carry ApplyWarnings or ApplyGates.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SPACE\tUNIT\tWARNINGS\tGATES")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", r.Space, r.Unit, r.Warnings, r.Gates)
	}
	_ = tw.Flush()
	clikit.Fprintln(out, fmt.Sprintf("\n%d Unit(s) marked.", len(rows)))
}
