// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

type resourceRow struct {
	Cluster   string `json:"cluster"`
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Space     string `json:"space"`
	UnitSlug  string `json:"unitSlug"`
	Canonical bool   `json:"canonical,omitempty"`
}

func newListCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var kindFilter, groupFilter, clusterFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Crossplane managed resources that make up the fleet (explorer)",
		Long: `list enumerates the Crossplane managed resources ConfigHub holds that the EKS
model reasons about — everything under eks.aws.upbound.io, ec2.aws.upbound.io,
and iam.aws.upbound.io — with the cluster, Space, and Unit each came from.
Canonical base/policy definitions are included and flagged.

Filter with --kind, --group, and --cluster. Note --cluster here is a client-side
display filter over the loaded snapshot; the fleet-scoping flags (--where,
--environment, --region, ...) are applied server-side and are what you want for
a large fleet.`,
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
			rows := buildResourceRows(snap, kindFilter, groupFilter, clusterFilter)
			if output == outputTable {
				printResourceTable(cmd, rows)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&kindFilter, "kind", "", "filter by kind (Cluster, NodeGroup, Addon, VPC, Subnet, Role, ...)")
	cmd.Flags().StringVar(&groupFilter, "group", "", "filter by API group prefix (eks, ec2, iam, or a full group)")
	cmd.Flags().StringVar(&clusterFilter, "cluster-name", "", "restrict output to this cluster (client-side)")
	return cmd
}

func buildResourceRows(snap *snapshot.Snapshot, kindFilter, groupFilter, clusterFilter string) []resourceRow {
	rows := make([]resourceRow, 0, len(snap.Resources))
	for _, r := range snap.Resources {
		apiVersion, kind, name, ok := eks.ResourceMeta(r.Doc)
		if !ok {
			continue
		}
		group, _ := eks.SplitAPIVersion(apiVersion)
		if group == "" {
			continue
		}
		if kindFilter != "" && !strings.EqualFold(kind, kindFilter) {
			continue
		}
		// --group accepts a short prefix ("eks") or a full group.
		if groupFilter != "" && !strings.HasPrefix(group, groupFilter) {
			continue
		}
		if clusterFilter != "" && r.Origin.Cluster != clusterFilter {
			continue
		}
		rows = append(rows, resourceRow{
			Cluster:   r.Origin.Cluster,
			Group:     group,
			Kind:      kind,
			Name:      name,
			Space:     r.Origin.Space,
			UnitSlug:  r.Origin.UnitSlug,
			Canonical: r.Origin.Canonical,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Group != rows[j].Group {
			return rows[i].Group < rows[j].Group
		}
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func printResourceTable(cmd *cobra.Command, rows []resourceRow) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tGROUP\tKIND\tNAME\tUNIT")
	for _, r := range rows {
		name := r.Name
		if r.Canonical {
			name += " (canonical)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Cluster, shortGroup(r.Group), r.Kind, name, r.UnitSlug)
	}
	_ = tw.Flush()
	fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d resources", len(rows)))
}

// shortGroup trims the provider suffix for display: eks.aws.upbound.io -> eks.
func shortGroup(group string) string {
	if i := strings.Index(group, "."); i > 0 {
		return group[:i]
	}
	return group
}
