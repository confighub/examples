// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
	"github.com/confighub/examples/rbac-manager-for-agents/internal/snapshot"
)

type clusterSummary struct {
	Cluster             string `json:"cluster"`
	Roles               int    `json:"roles"`
	ClusterRoles        int    `json:"clusterRoles"`
	RoleBindings        int    `json:"roleBindings"`
	ClusterRoleBindings int    `json:"clusterRoleBindings"`
	ServiceAccounts     int    `json:"serviceAccounts"`
	Units               int    `json:"units"`
	GatedUnits          int    `json:"gatedUnits"`
	UnappliedUnits      int    `json:"unappliedUnits"`
}

type snapshotReport struct {
	Clusters []clusterSummary `json:"clusters"`
	Totals   struct {
		Clusters       int `json:"clusters"`
		Roles          int `json:"roles"`
		Bindings       int `json:"bindings"`
		ServiceAccount int `json:"serviceAccounts"`
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
		Short: "Fleet RBAC inventory: per-cluster role/binding/SA and Unit counts",
		Long: `snapshot loads the fleet-wide Kubernetes RBAC view from ConfigHub and reports a
per-cluster inventory: Role/ClusterRole/RoleBinding/ClusterRoleBinding and
ServiceAccount counts, plus how many Units are gated or unapplied.

Clusters are ConfigHub Targets (the Space slug is used for unbound "paper
cluster" Units). Canonical base/policy Spaces are excluded from the inventory.`,
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
		for _, r := range c.Roles {
			if r.Kind == "ClusterRole" {
				cs.ClusterRoles++
			} else {
				cs.Roles++
			}
		}
		for _, b := range c.Bindings {
			if b.Kind == "ClusterRoleBinding" {
				cs.ClusterRoleBindings++
			} else {
				cs.RoleBindings++
			}
		}
		cs.ServiceAccounts = len(c.ServiceAccounts)
	}

	// Tally Unit-level stats per cluster, restricted to clusters that carry RBAC
	// (i.e. appear in snap.Clusters); this naturally excludes canonical Spaces.
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
	sort.Slice(report.Clusters, func(i, j int) bool {
		return report.Clusters[i].Cluster < report.Clusters[j].Cluster
	})
	for _, cs := range report.Clusters {
		report.Totals.Roles += cs.Roles + cs.ClusterRoles
		report.Totals.Bindings += cs.RoleBindings + cs.ClusterRoleBindings
		report.Totals.ServiceAccount += cs.ServiceAccounts
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
	fmt.Fprintln(tw, "CLUSTER\tROLES\tCROLES\tRB\tCRB\tSA\tUNITS\tGATED\tUNAPPLIED")
	for _, c := range r.Clusters {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			c.Cluster, c.Roles, c.ClusterRoles, c.RoleBindings, c.ClusterRoleBindings,
			c.ServiceAccounts, c.Units, c.GatedUnits, c.UnappliedUnits)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf(
		"\n%d clusters, %d roles, %d bindings, %d service accounts, %d units (%d gated, %d unapplied)",
		r.Totals.Clusters, r.Totals.Roles, r.Totals.Bindings, r.Totals.ServiceAccount,
		r.Totals.Units, r.Totals.GatedUnits, r.Totals.UnappliedUnits))
}
