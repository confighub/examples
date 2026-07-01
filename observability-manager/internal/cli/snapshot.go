// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/observability-manager/internal/cub"
	"github.com/confighub/examples/observability-manager/internal/snapshot"
)

type clusterSummary struct {
	Cluster         string `json:"cluster"`
	ServiceMonitors int    `json:"serviceMonitors"`
	Services        int    `json:"services"`
	MetricsServices int    `json:"metricsServices"`
	Workloads       int    `json:"workloads"`
	Units           int    `json:"units"`
	GatedUnits      int    `json:"gatedUnits"`
	UnappliedUnits  int    `json:"unappliedUnits"`
}

type snapshotReport struct {
	Clusters []clusterSummary `json:"clusters"`
	Totals   struct {
		Clusters        int `json:"clusters"`
		ServiceMonitors int `json:"serviceMonitors"`
		Services        int `json:"services"`
		MetricsServices int `json:"metricsServices"`
		Workloads       int `json:"workloads"`
		Units           int `json:"units"`
		GatedUnits      int `json:"gatedUnits"`
		UnappliedUnits  int `json:"unappliedUnits"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newSnapshotCmd() *cobra.Command {
	var output string
	var filter filterFlags
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Fleet inventory: per-cluster ServiceMonitor / Service / metrics-Service / workload counts",
		Long: `snapshot loads the fleet-wide observability view and reports a per-cluster
inventory: ServiceMonitors, Services (and how many expose metrics), and workloads,
plus how many Units are gated or unapplied.

Clusters are ConfigHub Targets (the Space slug is used for unbound Units).
Canonical base/policy Spaces are excluded.`,
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
			report := buildSnapshotReport(snap)
			if output == outputTable {
				printSnapshotTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	return cmd
}

func buildSnapshotReport(snap *snapshot.Snapshot) snapshotReport {
	byCluster := map[string]*clusterSummary{}
	get := func(name string) *clusterSummary {
		c, ok := byCluster[name]
		if !ok {
			c = &clusterSummary{Cluster: name}
			byCluster[name] = c
		}
		return c
	}
	for name, c := range snap.Clusters {
		cs := get(name)
		cs.ServiceMonitors = len(c.ServiceMonitors)
		cs.Services = len(c.Services)
		cs.Workloads = len(c.Workloads)
		for _, svc := range c.Services {
			if svc.ExposesMetrics() {
				cs.MetricsServices++
			}
		}
	}
	for _, u := range snap.Units {
		key := u.TargetSlug
		if key == "" {
			key = u.SpaceSlug
		}
		if _, ok := snap.Clusters[key]; !ok {
			continue
		}
		cs := get(key)
		cs.Units++
		if u.Gated() {
			cs.GatedUnits++
		}
		if u.Unapplied() {
			cs.UnappliedUnits++
		}
	}
	var report snapshotReport
	for _, cs := range byCluster {
		report.Clusters = append(report.Clusters, *cs)
	}
	sort.Slice(report.Clusters, func(i, j int) bool { return report.Clusters[i].Cluster < report.Clusters[j].Cluster })
	for _, cs := range report.Clusters {
		report.Totals.ServiceMonitors += cs.ServiceMonitors
		report.Totals.Services += cs.Services
		report.Totals.MetricsServices += cs.MetricsServices
		report.Totals.Workloads += cs.Workloads
		report.Totals.Units += cs.Units
		report.Totals.GatedUnits += cs.GatedUnits
		report.Totals.UnappliedUnits += cs.UnappliedUnits
	}
	report.Totals.Clusters = len(report.Clusters)
	report.Filter = snap.Filter
	return report
}

func printSnapshotTable(cmd *cobra.Command, r snapshotReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tSERVICEMONITORS\tSERVICES\tMETRICS-SVC\tWORKLOADS\tUNITS\tGATED\tUNAPPLIED")
	for _, c := range r.Clusters {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			c.Cluster, c.ServiceMonitors, c.Services, c.MetricsServices, c.Workloads, c.Units, c.GatedUnits, c.UnappliedUnits)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d clusters, %d ServiceMonitors, %d Services (%d expose metrics), %d workloads, %d units (%d gated, %d unapplied)",
		r.Totals.Clusters, r.Totals.ServiceMonitors, r.Totals.Services, r.Totals.MetricsServices,
		r.Totals.Workloads, r.Totals.Units, r.Totals.GatedUnits, r.Totals.UnappliedUnits))
}
