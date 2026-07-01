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

type findingsReport struct {
	Findings []nsmanager.Finding `json:"findings"`
	Totals   struct {
		Total      int            `json:"total"`
		BySeverity map[string]int `json:"bySeverity"`
		ByAnalyzer map[string]int `json:"byAnalyzer"`
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

  missing-default-deny         occupied namespace with no default-deny NetworkPolicy
  missing-pod-security         Namespace object with no pod-security enforce label
  missing-baseline-rbac        occupied namespace with no RoleBinding
  missing-namespace-object     occupied namespace with no v1/Namespace Unit
  duplicate-namespace          two Namespace Units collide on name + Target (one cluster)
  namespace-name-inconsistent  a component's namespace name varies across its variants
  pod-security-inconsistent    a component's pod-security level varies across its variants

These are properties of the whole set of resources across the fleet — the read a
per-resource validator or a runtime tenancy controller cannot do.

Filter with --severity (high|medium|low), --analyzer, --cluster, and --component.`,
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
			report := buildFindingsReport(snap, severityFilter, analyzerFilter, clusterFilter)
			if output == outputTable {
				printFindingsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&severityFilter, "severity", "", "filter by severity: high | medium | low")
	cmd.Flags().StringVar(&analyzerFilter, "analyzer", "", "filter by analyzer name")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "filter by cluster (Target or Space slug)")
	return cmd
}

func buildFindingsReport(snap *snapshot.Snapshot, severityFilter, analyzerFilter, clusterFilter string) findingsReport {
	var report findingsReport
	report.Totals.BySeverity = map[string]int{}
	report.Totals.ByAnalyzer = map[string]int{}
	for _, f := range nsmanager.AnalyzeFindings(snap.Clusters) {
		if severityFilter != "" && !strings.EqualFold(f.Severity, severityFilter) {
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
		r.Totals.Total, r.Totals.BySeverity[nsmanager.SeverityHigh],
		r.Totals.BySeverity[nsmanager.SeverityMedium], r.Totals.BySeverity[nsmanager.SeverityLow]))
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
