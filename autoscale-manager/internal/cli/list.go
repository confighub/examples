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

type autoscalerRow struct {
	Cluster    string `json:"cluster"`
	Namespace  string `json:"namespace,omitempty"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	TargetKind string `json:"targetKind,omitempty"`
	TargetName string `json:"targetName"`
	Min        *int64 `json:"min,omitempty"`
	Max        *int64 `json:"max,omitempty"`
	Pinned     bool   `json:"pinned"`
	Space      string `json:"space"`
	UnitSlug   string `json:"unitSlug"`
}

type listReport struct {
	Autoscalers []autoscalerRow `json:"autoscalers"`
	Total       int             `json:"total"`
	Filter      string          `json:"filter,omitempty"`
}

func newListCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var clusterFilter, namespaceFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every HorizontalPodAutoscaler and KEDA ScaledObject in the fleet",
		Long: `list reports each autoscaler (HorizontalPodAutoscaler or KEDA ScaledObject)
across the fleet: its scale target, min/max replicas, and whether it is pinned
(min == max, so it can't actually scale).`,
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
			report := buildListReport(snap, clusterFilter, namespaceFilter)
			if output == outputTable {
				printListTable(cmd, report)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), report)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "restrict output to this namespace")
	return cmd
}

func buildListReport(snap *snapshot.Snapshot, clusterFilter, namespaceFilter string) listReport {
	var rows []autoscalerRow
	for _, c := range snap.Clusters {
		for _, a := range c.Autoscalers {
			if clusterFilter != "" && a.Origin.Cluster != clusterFilter {
				continue
			}
			if namespaceFilter != "" && a.Namespace != namespaceFilter {
				continue
			}
			rows = append(rows, autoscalerRow{
				Cluster: a.Origin.Cluster, Namespace: a.Namespace, Kind: string(a.Kind), Name: a.Name,
				TargetKind: a.TargetKind, TargetName: a.TargetName,
				Min: a.Min, Max: a.Max, Pinned: a.Pinned(),
				Space: a.Origin.Space, UnitSlug: a.Origin.UnitSlug,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return listReport{Autoscalers: rows, Total: len(rows), Filter: snap.Filter}
}

func printListTable(cmd *cobra.Command, r listReport) {
	if len(r.Autoscalers) == 0 {
		fprintln(cmd.OutOrStdout(), "No autoscalers found.")
		return
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tKIND\tNAME\tTARGET\tMIN\tMAX\tPINNED\tUNIT")
	for _, a := range r.Autoscalers {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			a.Cluster, nsOrDash(a.Namespace), shortKind(a.Kind), a.Name,
			targetLabel(a.TargetKind, a.TargetName), minMax(a.Min), minMax(a.Max), yesNo(a.Pinned), a.UnitSlug)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d autoscalers", r.Total))
}

func shortKind(kind string) string {
	switch kind {
	case string(autoscale.KindHPA):
		return "HPA"
	case string(autoscale.KindScaledObject):
		return "ScaledObject"
	}
	return kind
}

func targetLabel(kind, name string) string {
	if name == "" {
		return "-"
	}
	if kind == "" {
		return name
	}
	return kind + "/" + name
}

func minMax(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}
