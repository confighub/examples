// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/workload-manager/internal/cub"
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
)

type resourceRow struct {
	Cluster   string `json:"cluster"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Space     string `json:"space"`
	UnitSlug  string `json:"unitSlug"`
	Canonical bool   `json:"canonical,omitempty"`
}

func newListCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var kindFilter, clusterFilter, namespaceFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workloads and PodDisruptionBudgets across the fleet (explorer)",
		Long: `list enumerates the resources ConfigHub holds that the workload-posture model
reasons about — pod-bearing workloads (Deployment, StatefulSet, DaemonSet,
ReplicaSet, Job, CronJob, Pod) and PodDisruptionBudgets — with the cluster,
Space, and Unit each came from. Canonical base/policy definitions are included
and flagged.

Filter with --kind, --cluster, and --namespace.`,
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
			rows := buildResourceRows(snap, kindFilter, clusterFilter, namespaceFilter)
			if output == outputTable {
				printResourceTable(cmd, rows)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&kindFilter, "kind", "", "filter by kind (Deployment, StatefulSet, PodDisruptionBudget, ...)")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "restrict output to this cluster (Target or Space slug)")
	cmd.Flags().StringVar(&namespaceFilter, "namespace", "", "filter by namespace")
	return cmd
}

func buildResourceRows(snap *snapshot.Snapshot, kindFilter, clusterFilter, namespaceFilter string) []resourceRow {
	rows := make([]resourceRow, 0, len(snap.Resources))
	for _, r := range snap.Resources {
		kind, name, namespace, ok := workload.ResourceMeta(r.Doc)
		if !ok {
			continue
		}
		// The explorer shows workloads and PDBs (the families the snapshot pulls).
		if !workload.IsWorkloadKind(kind) && kind != "PodDisruptionBudget" {
			continue
		}
		if kindFilter != "" && !strings.EqualFold(kind, kindFilter) {
			continue
		}
		if clusterFilter != "" && r.Origin.Cluster != clusterFilter {
			continue
		}
		if namespaceFilter != "" && namespace != namespaceFilter {
			continue
		}
		rows = append(rows, resourceRow{
			Cluster:   r.Origin.Cluster,
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
			Space:     r.Origin.Space,
			UnitSlug:  r.Origin.UnitSlug,
			Canonical: r.Origin.Canonical,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func printResourceTable(cmd *cobra.Command, rows []resourceRow) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tKIND\tNAMESPACE\tNAME\tUNIT")
	for _, r := range rows {
		ns := r.Namespace
		if ns == "" {
			ns = "-"
		}
		name := r.Name
		if r.Canonical {
			name += " (canonical)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Cluster, r.Kind, ns, name, r.UnitSlug)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d resources", len(rows)))
}
