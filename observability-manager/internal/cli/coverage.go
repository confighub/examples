// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/observability-manager/internal/cub"
	"github.com/confighub/examples/observability-manager/internal/observability"
	"github.com/confighub/examples/observability-manager/internal/snapshot"
)

type coverageReport struct {
	Services []observability.CoverageResult `json:"services"`
	Totals   struct {
		MetricsServices int `json:"metricsServices"`
		Covered         int `json:"covered"`
		Uncovered       int `json:"uncovered"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newCoverageCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter string
	var uncoveredOnly bool
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "ServiceMonitor coverage of metrics-exposing Services (the cross-Unit join)",
		Long: `coverage reports, for every metrics-exposing Service in the fleet, whether a
ServiceMonitor selects it — the cross-Unit join a per-Unit validator can't do,
since the ServiceMonitor and the Service live in separate Units.

A Service is considered to expose metrics if it has a port named
metrics/http-metrics/monitoring/prometheus/telemetry, or a prometheus.io/scrape=true
annotation. A ServiceMonitor covers a Service if its selector matches the
Service's labels in the same namespace.

Filter with --cluster / --namespace; --uncovered-only shows the gaps.`,
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
			report := buildCoverageReport(snap, clusterFilter, namespaceFilter, uncoveredOnly)
			if output == outputTable {
				printCoverageTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target slug, or None for Units whose Space has no release Target)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().BoolVar(&uncoveredOnly, "uncovered-only", false, "only metrics Services with no ServiceMonitor")
	return cmd
}

func buildCoverageReport(snap *snapshot.Snapshot, clusterFilter, namespaceFilter string, uncoveredOnly bool) coverageReport {
	var report coverageReport
	report.Filter = snap.Filter
	for _, r := range observability.AnalyzeCoverage(snap.Clusters) {
		if clusterFilter != "" && r.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && r.Namespace != namespaceFilter {
			continue
		}
		report.Totals.MetricsServices++
		if r.Covered {
			report.Totals.Covered++
		} else {
			report.Totals.Uncovered++
		}
		if uncoveredOnly && r.Covered {
			continue
		}
		report.Services = append(report.Services, r)
	}
	return report
}

func printCoverageTable(cmd *cobra.Command, r coverageReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tSERVICE\tMETRIC-PORT\tSERVICEMONITOR")
	for _, s := range r.Services {
		sm := "MISSING"
		if s.Covered {
			sm = strings.Join(s.Monitors, ",")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Cluster, dash(s.Namespace), s.Service, dash(s.MetricPort), sm)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d metrics Services (%d covered, %d uncovered)",
		r.Totals.MetricsServices, r.Totals.Covered, r.Totals.Uncovered))
}
