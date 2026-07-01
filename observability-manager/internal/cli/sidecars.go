// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/observability-manager/internal/cub"
	"github.com/confighub/examples/observability-manager/internal/observability"
	"github.com/confighub/examples/observability-manager/internal/snapshot"
)

type sidecarsReport struct {
	Workloads []observability.SidecarResult `json:"workloads"`
	Totals    struct {
		Workloads int `json:"workloads"`
		WithSidecar int `json:"withSidecar"`
		Without     int `json:"without"`
	} `json:"totals"`
	SidecarNames []string `json:"sidecarNames"`
	Filter       string   `json:"filter,omitempty"`
}

func newSidecarsCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter string
	var sidecarNames []string
	var missingOnly bool
	cmd := &cobra.Command{
		Use:   "sidecars",
		Short: "Report which workloads carry a telemetry (otel) sidecar container",
		Long: `sidecars reports, per workload, whether it has a telemetry-collector sidecar
container. By default it looks for the conventional OpenTelemetry container names
(otel-collector, otc-container, opentelemetry-collector, otel-agent); override
with --sidecar.

Filter with --cluster / --namespace; --missing-only shows workloads without a
sidecar.`,
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
			report := buildSidecarsReport(snap, sidecarNames, clusterFilter, namespaceFilter, missingOnly)
			if output == outputTable {
				printSidecarsTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().StringArrayVar(&sidecarNames, "sidecar", nil, "sidecar container name(s) to look for (default: the otel names)")
	cmd.Flags().BoolVar(&missingOnly, "missing-only", false, "only workloads without the sidecar")
	return cmd
}

func buildSidecarsReport(snap *snapshot.Snapshot, sidecarNames []string, clusterFilter, namespaceFilter string, missingOnly bool) sidecarsReport {
	var report sidecarsReport
	report.Filter = snap.Filter
	report.SidecarNames = sidecarNames
	if len(report.SidecarNames) == 0 {
		report.SidecarNames = observability.DefaultOtelContainerNames
	}
	for _, r := range observability.AnalyzeSidecars(snap.Clusters, sidecarNames) {
		if clusterFilter != "" && r.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && r.Namespace != namespaceFilter {
			continue
		}
		report.Totals.Workloads++
		if r.HasSidecar {
			report.Totals.WithSidecar++
		} else {
			report.Totals.Without++
		}
		if missingOnly && r.HasSidecar {
			continue
		}
		report.Workloads = append(report.Workloads, r)
	}
	return report
}

func printSidecarsTable(cmd *cobra.Command, r sidecarsReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tKIND\tNAME\tSIDECAR")
	for _, w := range r.Workloads {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", w.Cluster, nsOrDash(w.Namespace), w.Kind, w.Name, dash(w.Sidecar))
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d workloads (%d with sidecar, %d without)",
		r.Totals.Workloads, r.Totals.WithSidecar, r.Totals.Without))
}
