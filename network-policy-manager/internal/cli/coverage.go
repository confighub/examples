// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

type coverageReport struct {
	Namespaces []netpol.NamespaceCoverage `json:"namespaces"`
	Totals     struct {
		Namespaces         int `json:"namespaces"`
		WithoutDefaultDeny int `json:"namespacesWithoutDefaultDenyIngress"`
		Workloads          int `json:"workloads"`
		UncoveredIngress   int `json:"uncoveredIngressWorkloads"`
		UncoveredEgress    int `json:"uncoveredEgressWorkloads"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newCoverageCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter, direction string
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report NetworkPolicy coverage gaps by namespace (default-deny + uncovered workloads)",
		Long: `coverage reports, per namespace, whether a default-deny ingress/egress policy is
present and how many workloads are left uncovered (selected by no policy in that
direction). This is the question per-resource validators can't answer — it's a
property of the whole set of policies and workloads in a namespace.

Filter with --cluster, --namespace, and --direction (ingress|egress). With
--direction, only namespaces with a gap in that direction are shown.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if direction != "" && direction != "ingress" && direction != "egress" {
				return fmt.Errorf("--direction must be 'ingress' or 'egress'")
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			report := buildCoverageReport(snap, clusterFilter, namespaceFilter, direction)
			if output == outputTable {
				printCoverageTable(cmd, report, direction)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "filter by cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	cmd.Flags().StringVar(&direction, "direction", "", "limit to a coverage gap in this direction: ingress | egress")
	return cmd
}

func buildCoverageReport(snap *snapshot.Snapshot, clusterFilter, namespaceFilter, direction string) coverageReport {
	var report coverageReport
	clusters := make([]string, 0, len(snap.Clusters))
	for name := range snap.Clusters {
		clusters = append(clusters, name)
	}
	sort.Strings(clusters)
	for _, name := range clusters {
		if clusterFilter != "" && name != clusterFilter {
			continue
		}
		nss, _ := snap.Clusters[name].Coverage()
		for _, nc := range nss {
			if namespaceFilter != "" && nc.Namespace != namespaceFilter {
				continue
			}
			if direction == "ingress" && nc.DefaultDenyIngress && len(nc.UncoveredIngress) == 0 {
				continue
			}
			if direction == "egress" && nc.DefaultDenyEgress && len(nc.UncoveredEgress) == 0 {
				continue
			}
			report.Namespaces = append(report.Namespaces, nc)
			report.Totals.Workloads += nc.Workloads
			if !nc.DefaultDenyIngress {
				report.Totals.WithoutDefaultDeny++
			}
			report.Totals.UncoveredIngress += len(nc.UncoveredIngress)
			report.Totals.UncoveredEgress += len(nc.UncoveredEgress)
		}
	}
	report.Totals.Namespaces = len(report.Namespaces)
	report.Filter = snap.Filter
	return report
}

func printCoverageTable(cmd *cobra.Command, r coverageReport, direction string) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tPOLICY\tDD-INGRESS\tDD-EGRESS\tWORKLOADS\tUNCOVERED-IN\tUNCOVERED-EG")
	for _, nc := range r.Namespaces {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\n",
			nc.Cluster, nc.Namespace, yesNo(nc.HasPolicy), yesNo(nc.DefaultDenyIngress), yesNo(nc.DefaultDenyEgress),
			nc.Workloads, len(nc.UncoveredIngress), len(nc.UncoveredEgress))
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d namespaces (%d without default-deny ingress), %d workloads, %d uncovered on ingress, %d uncovered on egress",
		r.Totals.Namespaces, r.Totals.WithoutDefaultDeny, r.Totals.Workloads,
		r.Totals.UncoveredIngress, r.Totals.UncoveredEgress))

	// List the uncovered workloads inline so the table is actionable.
	for _, nc := range r.Namespaces {
		if direction != "egress" && len(nc.UncoveredIngress) > 0 {
			fprintln(cmd.OutOrStdout(), fmt.Sprintf("  %s/%s uncovered ingress: %s",
				nc.Cluster, nc.Namespace, strings.Join(nc.UncoveredIngress, ", ")))
		}
		if direction == "egress" && len(nc.UncoveredEgress) > 0 {
			fprintln(cmd.OutOrStdout(), fmt.Sprintf("  %s/%s uncovered egress: %s",
				nc.Cluster, nc.Namespace, strings.Join(nc.UncoveredEgress, ", ")))
		}
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
