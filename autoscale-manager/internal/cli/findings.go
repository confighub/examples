// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/autoscale-manager/internal/autoscale"
	"github.com/confighub/examples/autoscale-manager/internal/cub"
	"github.com/confighub/examples/autoscale-manager/internal/snapshot"
)

type findingsReport struct {
	Findings []autoscale.Finding `json:"findings"`
	Summary  map[string]int      `json:"summary"`
	Total    int                 `json:"total"`
	Filter   string              `json:"filter,omitempty"`
}

func newFindingsCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var minSeverity string
	var clusterFilter, namespaceFilter string
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Report autoscaling issues across the fleet, most-severe first",
		Long: `findings analyzes the fleet and reports autoscaling issues, most-severe first:

  - autoscaler-pinned (medium): an HPA/ScaledObject with min == max (can't scale)
  - pdb-blocks-min-scale (medium): a PodDisruptionBudget whose minAvailable blocks
    all voluntary eviction when the workload is at its autoscaler's minReplicas
    (the cross-resource HPA-vs-PDB check)
  - no-autoscaler (low): a Deployment/StatefulSet not targeted by any autoscaler

Canonical base/policy Spaces are excluded from analysis.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			threshold, err := parseSeverity(minSeverity)
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			report := buildFindingsReport(snap, threshold, clusterFilter, namespaceFilter)
			if output == outputTable {
				printFindingsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&minSeverity, "min-severity", "low", "minimum severity to report: low | medium | high")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "restrict output to this namespace")
	return cmd
}

func parseSeverity(s string) (autoscale.Severity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "low":
		return autoscale.SeverityLow, nil
	case "medium":
		return autoscale.SeverityMedium, nil
	case "high":
		return autoscale.SeverityHigh, nil
	}
	return "", fmt.Errorf("invalid --min-severity %q: want low, medium, or high", s)
}

func buildFindingsReport(snap *snapshot.Snapshot, threshold autoscale.Severity, clusterFilter, namespaceFilter string) findingsReport {
	all := autoscale.Findings(snap.Clusters)
	kept := make([]autoscale.Finding, 0, len(all))
	summary := map[string]int{}
	for _, f := range all {
		if !autoscale.AtLeast(f.Severity, threshold) {
			continue
		}
		if clusterFilter != "" && f.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && f.Namespace != namespaceFilter {
			continue
		}
		kept = append(kept, f)
		summary[string(f.Severity)]++
	}
	return findingsReport{Findings: kept, Summary: summary, Total: len(kept), Filter: snap.Filter}
}

func printFindingsTable(cmd *cobra.Command, r findingsReport) {
	if len(r.Findings) == 0 {
		fprintln(cmd.OutOrStdout(), "No autoscaling findings.")
		return
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tANALYZER\tCLUSTER\tNAMESPACE\tKIND\tNAME\tMESSAGE")
	for _, f := range r.Findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			f.Severity, f.Analyzer, f.Cluster, nsOrDash(f.Namespace), f.Kind, f.Name, f.Message)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d findings (high=%d medium=%d low=%d)",
		r.Total, r.Summary["high"], r.Summary["medium"], r.Summary["low"]))
}
