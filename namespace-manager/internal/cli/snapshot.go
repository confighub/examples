// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/namespace-manager/internal/cub"
	"github.com/confighub/examples/namespace-manager/internal/snapshot"
)

type clusterSummary struct {
	Cluster         string `json:"cluster"`
	Namespaces      int    `json:"namespaces"`
	Workloads       int    `json:"workloads"`
	Units           int    `json:"units"`
	GatedUnits      int    `json:"gatedUnits"`
	UnreleasedUnits int    `json:"unreleasedUnits"`
}

type snapshotReport struct {
	Clusters []clusterSummary `json:"clusters"`
	Totals   struct {
		Clusters        int `json:"clusters"`
		Namespaces      int `json:"namespaces"`
		Workloads       int `json:"workloads"`
		Units           int `json:"units"`
		GatedUnits      int `json:"gatedUnits"`
		UnreleasedUnits int `json:"unreleasedUnits"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newSnapshotCmd() *cobra.Command {
	var output string
	var filter filterFlags
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Fleet inventory: per-cluster namespace and workload counts, and Unit state",
		Long: `snapshot loads the fleet-wide namespace view from ConfigHub and reports a
per-cluster inventory: Namespace and workload counts, plus how many Units are
gated or unreleased.

Clusters are ConfigHub Targets; Units whose Space has no release Target group
under a single "None" cluster. Canonical base/policy Spaces are excluded from
the inventory.

Per-namespace envelope completeness ("which namespaces lack a Namespace object
or pod-security labels?") is reported by the 'envelope' command; this is raw
inventory only.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.Predicate())
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
		cs.Namespaces = len(c.Namespaces)
		cs.Workloads = len(c.Workloads)
	}

	// Tally Unit-level stats per cluster, restricted to clusters that carry
	// envelope-relevant config (i.e. appear in snap.Clusters); this naturally
	// excludes canonical Spaces.
	for _, u := range snap.Units {
		key := u.TargetSlug
		if key == "" {
			key = snapshot.ClusterNone
		}
		if _, ok := snap.Clusters[key]; !ok {
			continue
		}
		cs := get(key)
		cs.Units++
		if u.Gated() {
			cs.GatedUnits++
		}
		if u.Unreleased() {
			cs.UnreleasedUnits++
		}
	}

	var report snapshotReport
	for _, cs := range byCluster {
		report.Clusters = append(report.Clusters, *cs)
	}
	sort.Slice(report.Clusters, func(i, j int) bool {
		return report.Clusters[i].Cluster < report.Clusters[j].Cluster
	})
	for _, cs := range report.Clusters {
		report.Totals.Namespaces += cs.Namespaces
		report.Totals.Workloads += cs.Workloads
		report.Totals.Units += cs.Units
		report.Totals.GatedUnits += cs.GatedUnits
		report.Totals.UnreleasedUnits += cs.UnreleasedUnits
	}
	report.Totals.Clusters = len(report.Clusters)
	report.Filter = snap.Filter
	return report
}

func printSnapshotTable(cmd *cobra.Command, r snapshotReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNS\tWORKLOADS\tUNITS\tGATED\tUNAPPLIED")
	for _, c := range r.Clusters {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\n",
			c.Cluster, c.Namespaces, c.Workloads,
			c.Units, c.GatedUnits, c.UnreleasedUnits)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d clusters, %d namespaces, %d workloads, %d units (%d gated, %d unreleased)",
		r.Totals.Clusters, r.Totals.Namespaces,
		r.Totals.Workloads, r.Totals.Units, r.Totals.GatedUnits, r.Totals.UnreleasedUnits))
}
