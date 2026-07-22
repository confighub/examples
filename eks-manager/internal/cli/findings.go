// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

type findingsReport struct {
	Findings []eks.Finding `json:"findings"`
	Totals   struct {
		Findings int `json:"findings"`
		Critical int `json:"critical"`
		High     int `json:"high"`
		Medium   int `json:"medium"`
		Low      int `json:"low"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newFindingsCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var severity, analyzer, clusterFilter string

	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Severity-ranked EKS findings across the fleet",
		Long: `findings reports issues the EKS model can see in the config itself, ranked by
severity. The analyzers:

  autoscaler-conflict  desiredSize managed by Crossplane while an external
                       autoscaler also manages it; LateInitialize in
                       managementPolicies; desiredSize outside min/max
  version-skew         node groups behind (or ahead of) their control plane
  support-policy       a pinned version under STANDARD support, which AWS will
                       auto-upgrade and then fight
  automode-invariant   the three Auto Mode toggles disagree or are incomplete
  bootstrap-addons     Auto Mode enabled with bootstrapSelfManagedAddons true,
                       which is immutable and cannot be reconciled
  exposure             public API endpoint, no secret encryption, missing
                       control-plane logs, deletionProtection off
  dangling-ref         a reference that does not resolve, which blocks
                       reconciliation silently and indefinitely
  inconsistent         clusters in one environment running different versions

These are properties of the stored configuration. Pending *changes* are graded
separately by 'plan', which compares head against last-applied.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate flags before any network call, so bad input fails fast.
			if severity != "" && !eks.ValidSeverity(severity) {
				return fmt.Errorf("unknown --severity %q (want critical | high | medium | low)", severity)
			}
			if analyzer != "" && !validAnalyzer(analyzer) {
				return fmt.Errorf("unknown --analyzer %q (want one of: %s)",
					analyzer, strings.Join(eks.AllAnalyzers, ", "))
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			report := buildFindingsReport(snap, severity, analyzer, clusterFilter)
			if output == outputTable {
				printFindingsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&severity, "severity", "", "filter by severity: critical | high | medium | low")
	cmd.Flags().StringVar(&analyzer, "analyzer", "", "filter by analyzer name")
	cmd.Flags().StringVar(&clusterFilter, "cluster-name", "", "restrict output to this cluster (client-side)")
	return cmd
}

func validAnalyzer(name string) bool {
	for _, a := range eks.AllAnalyzers {
		if a == name {
			return true
		}
	}
	return false
}

func buildFindingsReport(snap *snapshot.Snapshot, severity, analyzer, clusterFilter string) findingsReport {
	var report findingsReport
	for _, f := range eks.Findings(snap.Clusters) {
		if severity != "" && string(f.Severity) != severity {
			continue
		}
		if analyzer != "" && f.Analyzer != analyzer {
			continue
		}
		if clusterFilter != "" && f.Cluster != clusterFilter {
			continue
		}
		report.Findings = append(report.Findings, f)
		switch f.Severity {
		case eks.SeverityCritical:
			report.Totals.Critical++
		case eks.SeverityHigh:
			report.Totals.High++
		case eks.SeverityMedium:
			report.Totals.Medium++
		default:
			report.Totals.Low++
		}
	}
	report.Totals.Findings = len(report.Findings)
	report.Filter = snap.Filter
	return report
}

func printFindingsTable(cmd *cobra.Command, r findingsReport) {
	out := cmd.OutOrStdout()
	if len(r.Findings) == 0 {
		fprintln(out, "No findings.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tANALYZER\tCLUSTER\tKIND\tNAME\tMESSAGE")
	for _, f := range r.Findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(string(f.Severity)), f.Analyzer, f.Cluster, f.Kind, f.Name, f.Message)
	}
	_ = tw.Flush()
	fprintln(out, fmt.Sprintf("\n%d findings (%d critical, %d high, %d medium, %d low)",
		r.Totals.Findings, r.Totals.Critical, r.Totals.High, r.Totals.Medium, r.Totals.Low))
}
