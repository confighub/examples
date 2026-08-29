// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

type findingsReport struct {
	Findings []netpol.Finding `json:"findings"`
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
		Short: "NetworkPolicy hygiene and anti-pattern findings across the fleet",
		Long: `findings runs the v1 analyzer set over the fleet:

  missing-default-deny-ingress  namespace has workloads but no default-deny ingress
  uncovered-ingress             workload selected by no ingress NetworkPolicy
  allow-all                     a policy rule with an empty from/to (admits all peers)
  metadata-egress               egress ipBlock exposes the cloud metadata IP 169.254.169.254
  ingress-egress-asymmetry      one side allows a flow the other side silently drops

Filter with --severity (Critical|High|Medium|Low), --analyzer, and --cluster.`,
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

	names := make([]string, 0, len(snap.Clusters))
	for n := range snap.Clusters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, cn := range names {
		if clusterFilter != "" && cn != clusterFilter {
			continue
		}
		for _, f := range netpol.AnalyzeFindings(snap.Clusters[cn]) {
			if severityFilter != api.ScoreNone && f.Severity != severityFilter {
				continue
			}
			if analyzerFilter != "" && f.Analyzer != analyzerFilter {
				continue
			}
			report.Findings = append(report.Findings, f)
			report.Totals.BySeverity[f.Severity]++
			report.Totals.ByAnalyzer[f.Analyzer]++
		}
	}
	report.Totals.Total = len(report.Findings)
	report.Filter = snap.Filter
	return report
}

func printFindingsTable(cmd *cobra.Command, r findingsReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tANALYZER\tCLUSTER\tNAMESPACE\tRESOURCE\tMESSAGE")
	for _, f := range r.Findings {
		res := f.Resource
		if res == "" {
			res = "-"
		}
		ns := f.Namespace
		if ns == "" {
			ns = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", f.Severity, f.Analyzer, f.Cluster, ns, res, f.Message)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d findings (%d high, %d medium, %d low)",
		r.Totals.Total, r.Totals.BySeverity[api.ScoreHigh],
		r.Totals.BySeverity[api.ScoreMedium], r.Totals.BySeverity[api.ScoreLow]))
}
