// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
	"github.com/confighub/examples/scheduling-manager/internal/snapshot"
)

type clusterSummary struct {
	Cluster        string `json:"cluster"`
	Workloads      int    `json:"workloads"`
	WithSelector   int    `json:"withNodeSelector"`
	WithToleration int    `json:"withTolerations"`
	WithAffinity   int    `json:"withNodeAffinity"`
	Units          int    `json:"units"`
	GatedUnits     int    `json:"gatedUnits"`
	UnappliedUnits int    `json:"unappliedUnits"`
}

type snapshotReport struct {
	Clusters []clusterSummary `json:"clusters"`
	Totals   struct {
		Clusters       int `json:"clusters"`
		Workloads      int `json:"workloads"`
		WithSelector   int `json:"withNodeSelector"`
		WithToleration int `json:"withTolerations"`
		WithAffinity   int `json:"withNodeAffinity"`
		Units          int `json:"units"`
		GatedUnits     int `json:"gatedUnits"`
		UnappliedUnits int `json:"unappliedUnits"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newSnapshotCmd() *cobra.Command {
	var output string
	var filter filterFlags
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Fleet inventory: per-cluster workload counts and how many pin placement",
		Long: `snapshot loads the fleet-wide placement view and reports a per-cluster inventory:
workload counts, and how many set a nodeSelector, tolerations, or node affinity,
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
		cs.Workloads = len(c.Workloads)
		for _, w := range c.Workloads {
			if w.HasNodeSelector() {
				cs.WithSelector++
			}
			if w.HasTolerations() {
				cs.WithToleration++
			}
			if w.HasNodeAffinity {
				cs.WithAffinity++
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
		report.Totals.Workloads += cs.Workloads
		report.Totals.WithSelector += cs.WithSelector
		report.Totals.WithToleration += cs.WithToleration
		report.Totals.WithAffinity += cs.WithAffinity
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
	fmt.Fprintln(tw, "CLUSTER\tWORKLOADS\tNODESELECTOR\tTOLERATIONS\tNODEAFFINITY\tUNITS\tGATED\tUNAPPLIED")
	for _, c := range r.Clusters {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			c.Cluster, c.Workloads, c.WithSelector, c.WithToleration, c.WithAffinity, c.Units, c.GatedUnits, c.UnappliedUnits)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d clusters, %d workloads (%d nodeSelector, %d tolerations, %d nodeAffinity), %d units (%d gated, %d unapplied)",
		r.Totals.Clusters, r.Totals.Workloads, r.Totals.WithSelector, r.Totals.WithToleration, r.Totals.WithAffinity,
		r.Totals.Units, r.Totals.GatedUnits, r.Totals.UnappliedUnits))
}
