// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

type consistencyReport struct {
	Components []nsmanager.ComponentConsistency `json:"components"`
	Totals     struct {
		Components   int `json:"components"`
		Consistent   int `json:"consistent"`
		Inconsistent int `json:"inconsistent"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newConsistencyCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var componentFilter string
	var inconsistentOnly bool
	cmd := &cobra.Command{
		Use:   "consistency",
		Short: "Cross-variant consistency: is a component's namespace name + pod-security identical across its variant Spaces?",
		Long: `consistency groups the fleet's namespaces by their Space's Component label and
reports, per component, whether the namespace name and pod-security enforce level
are identical across every variant Space (environment / region / cluster).

The invariant is that a component uses the same namespace everywhere; this is the
read side of that invariant (its enforcement is a per-cluster set-namespace
Trigger). Only Spaces carrying a Component label participate; canonical base
Spaces are excluded.

This is a fleet-wide property no per-cluster controller or per-resource validator
can see. Filter with --component and --inconsistent-only.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			report := buildConsistencyReport(snap, componentFilter, inconsistentOnly)
			if output == outputTable {
				printConsistencyTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&componentFilter, "component-name", "", "restrict output to this component (client-side)")
	cmd.Flags().BoolVar(&inconsistentOnly, "inconsistent-only", false, "only components that are inconsistent across variants")
	return cmd
}

func buildConsistencyReport(snap *snapshot.Snapshot, componentFilter string, inconsistentOnly bool) consistencyReport {
	var report consistencyReport
	for _, cc := range nsmanager.AnalyzeConsistency(snap.Clusters) {
		if componentFilter != "" && cc.Component != componentFilter {
			continue
		}
		report.Totals.Components++
		if cc.Consistent {
			report.Totals.Consistent++
		} else {
			report.Totals.Inconsistent++
		}
		if inconsistentOnly && cc.Consistent {
			continue
		}
		report.Components = append(report.Components, cc)
	}
	report.Filter = snap.Filter
	return report
}

func printConsistencyTable(cmd *cobra.Command, r consistencyReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "COMPONENT\tVARIANTS\tNAMESPACES\tPOD-SECURITY\tCONSISTENT\tISSUES")
	for _, c := range r.Components {
		issues := "-"
		if len(c.Issues) > 0 {
			issues = strings.Join(c.Issues, "; ")
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n",
			c.Component, len(c.Variants), dash(strings.Join(c.Namespaces, ",")),
			dash(strings.Join(c.PodSecurityLevels, ",")), yesNo(c.Consistent), issues)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d components (%d consistent, %d inconsistent)",
		r.Totals.Components, r.Totals.Consistent, r.Totals.Inconsistent))
}
