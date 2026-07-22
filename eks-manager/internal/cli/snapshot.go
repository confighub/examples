// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

type clusterSummary struct {
	Cluster        string `json:"cluster"`
	Version        string `json:"version,omitempty"`
	Region         string `json:"region,omitempty"`
	AutoMode       bool   `json:"autoMode"`
	NodeGroups     int    `json:"nodeGroups"`
	Addons         int    `json:"addons"`
	Network        int    `json:"network"`
	IAM            int    `json:"iam"`
	Units          int    `json:"units"`
	GatedUnits     int    `json:"gatedUnits"`
	UnappliedUnits int    `json:"unappliedUnits"`
	// NoControlPlane flags a Space holding EKS-adjacent resources with no
	// Cluster resource — usually shared networking, occasionally a mistake.
	NoControlPlane bool `json:"noControlPlane,omitempty"`
}

type snapshotReport struct {
	Clusters []clusterSummary `json:"clusters"`
	Totals   struct {
		Clusters       int `json:"clusters"`
		NodeGroups     int `json:"nodeGroups"`
		Addons         int `json:"addons"`
		Network        int `json:"network"`
		IAM            int `json:"iam"`
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
		Short: "Fleet inventory: per-cluster EKS, networking, and IAM resource counts",
		Long: `snapshot loads the fleet-wide EKS view from ConfigHub and reports a per-cluster
inventory: the control-plane version and region, whether EKS Auto Mode is on,
node group / addon / networking / IAM resource counts, and how many Units are
gated or unapplied.

A cluster here is a Space — its Units describe an EKS cluster rather than deploy
to one — identified by the Space's Cluster label, falling back to the Space slug.
(That is deliberately unlike the sibling managers, where a cluster is a Target.
The Target of one of these Spaces is the Crossplane management cluster the
managed resources get applied to, which is a different cluster entirely.)

Canonical base/policy Spaces are excluded from the inventory.`,
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

	for name, cs := range snap.Clusters {
		s := get(name)
		s.NodeGroups = len(cs.NodeGroups)
		s.Addons = len(cs.Addons)
		s.Network = len(cs.Network)
		s.IAM = len(cs.IAM)
		if cs.Control != nil {
			s.Version = cs.Control.Version
			s.Region = cs.Control.Region
			s.AutoMode = cs.Control.AutoMode.Enabled()
		} else {
			s.NoControlPlane = true
		}
	}

	// Tally Unit-level stats per cluster, restricted to clusters that carry
	// EKS-relevant config (i.e. appear in snap.Clusters); this naturally
	// excludes canonical Spaces.
	for _, u := range snap.Units {
		key := u.SpaceLabels[snapshot.SpaceLabelCluster]
		if key == "" {
			key = u.SpaceSlug
		}
		if _, ok := snap.Clusters[key]; !ok {
			continue
		}
		s := get(key)
		s.Units++
		if u.Gated() {
			s.GatedUnits++
		}
		if u.Unapplied() {
			s.UnappliedUnits++
		}
	}

	var report snapshotReport
	for _, s := range byCluster {
		report.Clusters = append(report.Clusters, *s)
	}
	sort.Slice(report.Clusters, func(i, j int) bool {
		return report.Clusters[i].Cluster < report.Clusters[j].Cluster
	})
	for _, s := range report.Clusters {
		report.Totals.NodeGroups += s.NodeGroups
		report.Totals.Addons += s.Addons
		report.Totals.Network += s.Network
		report.Totals.IAM += s.IAM
		report.Totals.Units += s.Units
		report.Totals.GatedUnits += s.GatedUnits
		report.Totals.UnappliedUnits += s.UnappliedUnits
	}
	report.Totals.Clusters = len(report.Clusters)
	report.Filter = snap.Filter
	return report
}

func printSnapshotTable(cmd *cobra.Command, r snapshotReport) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tVERSION\tREGION\tMODE\tNODEGROUPS\tADDONS\tNETWORK\tIAM\tUNITS\tGATED\tUNAPPLIED")
	for _, c := range r.Clusters {
		mode := "classic"
		if c.AutoMode {
			mode = "auto"
		}
		if c.NoControlPlane {
			mode = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			c.Cluster, dash(c.Version), dash(c.Region), mode,
			c.NodeGroups, c.Addons, c.Network, c.IAM,
			c.Units, c.GatedUnits, c.UnappliedUnits)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d clusters, %d nodegroups, %d addons, %d units (%d gated, %d unapplied)",
		r.Totals.Clusters, r.Totals.NodeGroups, r.Totals.Addons,
		r.Totals.Units, r.Totals.GatedUnits, r.Totals.UnappliedUnits))
}
