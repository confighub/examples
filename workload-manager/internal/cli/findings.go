// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/managerkit"
	"github.com/confighub/examples/workload-manager/internal/cub"
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
)

type findingsReport struct {
	Findings []workload.Finding `json:"findings"`
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
	var severityFilter, analyzerFilter, clusterFilter, namespaceFilter string
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Severity-ranked workload-readiness findings across the fleet",
		Long: `findings flattens the fleet readiness scorecard into a severity-ranked list —
one finding per issue, most-severe first — spanning every analyzer (security,
resources, probes, hygiene, availability).

Severity: a failing security / resources / availability check is high; a failing
probe is medium; warnings are one step down; hygiene is low.

Filter with --severity (Critical|High|Medium|Low), --analyzer (the dimension), --cluster,
and --namespace.`,
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
			report := buildFindingsReport(snap, severity, analyzerFilter, clusterFilter, namespaceFilter)
			if done, err := managerkit.Render(cmd.OutOrStdout(), output, report); done || err != nil {
				return err
			}
			printFindingsTable(cmd, report)
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&severityFilter, "severity", "", "only findings at this severity: Critical | High | Medium | Low")
	cmd.Flags().StringVar(&analyzerFilter, "analyzer", "", "only findings from this analyzer: security | resources | probes | hygiene | availability")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target slug, or None for Units whose Space has no release Target)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	return cmd
}

func buildFindingsReport(snap *snapshot.Snapshot, severityFilter api.Score, analyzerFilter, clusterFilter, namespaceFilter string) findingsReport {
	var report findingsReport
	report.Filter = snap.Filter
	for _, f := range workload.Findings(snap.Clusters) {
		if severityFilter != api.ScoreNone && f.Severity != severityFilter {
			continue
		}
		if analyzerFilter != "" && f.Analyzer != analyzerFilter {
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
		ns := f.Namespace
		if ns == "" {
			ns = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(string(f.Severity)), f.Analyzer, f.Cluster, ns, f.Kind, f.Name, f.Message)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d findings (%d high, %d medium, %d low)",
		r.Totals.Findings, r.Totals.High, r.Totals.Medium, r.Totals.Low))
}
