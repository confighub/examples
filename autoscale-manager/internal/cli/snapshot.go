// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/autoscale-manager/internal/autoscale"
	"github.com/confighub/examples/autoscale-manager/internal/cub"
	"github.com/confighub/examples/autoscale-manager/internal/snapshot"
)

type clusterSummary struct {
	Cluster         string `json:"cluster"`
	HPAs            int    `json:"hpas"`
	ScaledObjects   int    `json:"scaledObjects"`
	Workloads       int    `json:"workloads"`
	Autoscaled      int    `json:"autoscaledWorkloads"`
	PDBs            int    `json:"pdbs"`
	Units           int    `json:"units"`
	GatedUnits      int    `json:"gatedUnits"`
	UnappliedUnits  int    `json:"unappliedUnits"`
}

type snapshotReport struct {
	Clusters []clusterSummary `json:"clusters"`
	Totals   struct {
		Clusters      int `json:"clusters"`
		HPAs          int `json:"hpas"`
		ScaledObjects int `json:"scaledObjects"`
		Workloads     int `json:"workloads"`
		Autoscaled    int `json:"autoscaledWorkloads"`
		PDBs          int `json:"pdbs"`
		Units         int `json:"units"`
	} `json:"totals"`
	Filter string `json:"filter,omitempty"`
}

func newSnapshotCmd() *cobra.Command {
	var output string
	var filter filterFlags
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Fleet inventory: per-cluster HPA / ScaledObject / workload / PDB counts",
		Long: `snapshot loads the fleet-wide autoscaling view and reports a per-cluster
inventory: HorizontalPodAutoscalers, KEDA ScaledObjects, scalable workloads (and
how many are autoscaled), and PodDisruptionBudgets.

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
		cs.PDBs = len(c.PDBs)
		cs.Workloads = len(c.Workloads)
		for _, a := range c.Autoscalers {
			if a.Kind == autoscale.KindHPA {
				cs.HPAs++
			} else {
				cs.ScaledObjects++
			}
		}
		targeted := map[string]bool{}
		for _, a := range c.Autoscalers {
			targeted[a.Namespace+"/"+a.TargetName] = true
		}
		for _, w := range c.Workloads {
			if targeted[w.Namespace+"/"+w.Name] {
				cs.Autoscaled++
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
		report.Totals.HPAs += cs.HPAs
		report.Totals.ScaledObjects += cs.ScaledObjects
		report.Totals.Workloads += cs.Workloads
		report.Totals.Autoscaled += cs.Autoscaled
		report.Totals.PDBs += cs.PDBs
		report.Totals.Units += cs.Units
	}
	report.Totals.Clusters = len(report.Clusters)
	report.Filter = snap.Filter
	return report
}

func printSnapshotTable(cmd *cobra.Command, r snapshotReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tHPAS\tSCALEDOBJECTS\tWORKLOADS\tAUTOSCALED\tPDBS\tUNITS")
	for _, c := range r.Clusters {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
			c.Cluster, c.HPAs, c.ScaledObjects, c.Workloads, c.Autoscaled, c.PDBs, c.Units)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d clusters, %d HPAs, %d ScaledObjects, %d workloads (%d autoscaled), %d PDBs",
		r.Totals.Clusters, r.Totals.HPAs, r.Totals.ScaledObjects, r.Totals.Workloads, r.Totals.Autoscaled, r.Totals.PDBs))
}
