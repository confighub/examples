// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
	"github.com/confighub/examples/scheduling-manager/internal/scheduling"
	"github.com/confighub/examples/scheduling-manager/internal/snapshot"
)

type findingsReport struct {
	Findings []scheduling.Finding `json:"findings"`
	Totals   struct {
		Findings int `json:"findings"`
		High     int `json:"high"`
		Medium   int `json:"medium"`
		Low      int `json:"low"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newFindingsCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var severityFilter, clusterFilter, namespaceFilter string
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Severity-ranked placement findings across the fleet",
		Long: `findings reports placement anti-patterns across the fleet, most-severe first.

v1 flags workloads that tolerate a taint but don't constrain where they land (no
nodeSelector and no required node affinity), so they may schedule onto general
nodes — usually not the intent of a toleration. Checks that need cluster
node-pool / taint facts are deferred until those Target facts exist.

Filter with --severity (Critical|High|Medium|Low), --cluster, and --namespace.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			severity, err := parseScore(severityFilter)
			if err != nil {
				return err
			}
			report := buildFindingsReport(snap, severity, clusterFilter, namespaceFilter)
			if output == outputTable {
				printFindingsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&severityFilter, "severity", "", "only findings at this severity: Critical | High | Medium | Low")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target slug, or None for Units whose Space has no release Target)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	return cmd
}

func buildFindingsReport(snap *snapshot.Snapshot, severityFilter api.Score, clusterFilter, namespaceFilter string) findingsReport {
	var report findingsReport
	report.Filter = snap.Filter
	for _, f := range scheduling.Findings(snap.Clusters) {
		if severityFilter != api.ScoreNone && f.Severity != severityFilter {
			continue
		}
		if clusterFilter != "" && f.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && f.Namespace != namespaceFilter {
			continue
		}
		report.Findings = append(report.Findings, f)
		report.Totals.Findings++
		switch f.Severity {
		case api.ScoreHigh:
			report.Totals.High++
		case api.ScoreMedium:
			report.Totals.Medium++
		case api.ScoreLow:
			report.Totals.Low++
		}
	}
	return report
}

func printFindingsTable(cmd *cobra.Command, r findingsReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tANALYZER\tCLUSTER\tNAMESPACE\tKIND\tNAME\tMESSAGE")
	for _, f := range r.Findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(string(f.Severity)), f.Analyzer, f.Cluster, dash(f.Namespace), f.Kind, f.Name, f.Message)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d findings (%d high, %d medium, %d low)",
		r.Totals.Findings, r.Totals.High, r.Totals.Medium, r.Totals.Low))
}
