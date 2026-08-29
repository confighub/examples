// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"text/tabwriter"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

type findingsReport struct {
	Findings []nsmanager.Finding `json:"findings"`
	Totals   struct {
		Total      int               `json:"total"`
		BySeverity map[api.Score]int `json:"bySeverity"`
		ByAnalyzer map[string]int    `json:"byAnalyzer"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newFindingsCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var severityFilter, analyzerFilter, clusterFilter string
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Namespace-governance findings across the fleet (envelope gaps, duplicates, inconsistency)",
		Long: `findings runs the v1 analyzer set over the fleet:

  missing-pod-security         Namespace object with no pod-security enforce label
  missing-namespace-object     occupied namespace with no v1/Namespace Unit
  duplicate-namespace          two Namespace Units collide on name + Target (one cluster)
  namespace-name-inconsistent  a component's namespace name varies across its variants
  pod-security-inconsistent    a component's pod-security level varies across its variants

These are properties of the whole set of resources across the fleet — the read a
per-resource validator or a runtime tenancy controller cannot do.

Filter with --severity (Critical|High|Medium|Low), --analyzer, --cluster, and --component.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			severity, err := parseScore(severityFilter)
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			where, err := filter.Predicate()
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, where)
			if err != nil {
				return err
			}
			report := buildFindingsReport(snap, severity, analyzerFilter, clusterFilter)
			if output == outputTable {
				printFindingsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&severityFilter, "severity", "", "filter by severity: Critical | High | Medium | Low")
	cmd.Flags().StringVar(&analyzerFilter, "analyzer", "", "filter by analyzer name")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "filter by cluster (Target slug, or None for Units whose Space has no release Target)")
	return cmd
}

func buildFindingsReport(snap *snapshot.Snapshot, severityFilter api.Score, analyzerFilter, clusterFilter string) findingsReport {
	var report findingsReport
	report.Totals.BySeverity = map[api.Score]int{}
	report.Totals.ByAnalyzer = map[string]int{}
	for _, f := range nsmanager.AnalyzeFindings(snap.Clusters) {
		if severityFilter != api.ScoreNone && f.Severity != severityFilter {
			continue
		}
		if analyzerFilter != "" && f.Analyzer != analyzerFilter {
			continue
		}
		if clusterFilter != "" && f.Cluster != clusterFilter {
			continue
		}
		report.Findings = append(report.Findings, f)
		report.Totals.BySeverity[f.Severity]++
		report.Totals.ByAnalyzer[f.Analyzer]++
	}
	report.Totals.Total = len(report.Findings)
	report.Filter = snap.Filter
	return report
}

func printFindingsTable(cmd *cobra.Command, r findingsReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tANALYZER\tCLUSTER\tCOMPONENT\tNAMESPACE\tMESSAGE")
	for _, f := range r.Findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			f.Severity, f.Analyzer, dash(f.Cluster), dash(f.Component), dash(f.Namespace), f.Message)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d findings (%d high, %d medium, %d low)",
		r.Totals.Total, r.Totals.BySeverity[api.ScoreHigh],
		r.Totals.BySeverity[api.ScoreMedium], r.Totals.BySeverity[api.ScoreLow]))
}
