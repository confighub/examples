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

func newGetCmd() *cobra.Command {
	var output string
	var filter filterFlags
	cmd := &cobra.Command{
		Use:   "get <cluster>",
		Short: "Show the assembled view of one EKS cluster",
		Long: `get assembles every Crossplane managed resource belonging to one EKS cluster
into a single view: the control plane, its node groups and addons, and the
networking and IAM resources supporting it — the analog of 'eksctl get cluster',
read from the ConfigHub source of record rather than from AWS.

The cluster argument is the cluster key reported by 'snapshot' and 'list': the
Space's Cluster label, or the Space slug when that label is absent.

This reports intent — what the Units say. To compare it against what AWS
actually has, use 'status' (a later milestone), which reads the managed
resources' Crossplane conditions and status.atProvider.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}
			cs, ok := snap.Clusters[args[0]]
			if !ok {
				return fmt.Errorf("cluster %q not found; run `%s snapshot` to list the clusters in scope",
					args[0], InvocationName())
			}
			if output == outputTable {
				printClusterDetail(cmd, cs)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), cs)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	return cmd
}

func printClusterDetail(cmd *cobra.Command, cs *eks.ClusterSet) {
	w := cmd.OutOrStdout()
	fprintln(w, "CLUSTER "+cs.Cluster)

	if c := cs.Control; c != nil {
		fprintln(w, "")
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintf(tw, "  control plane\t%s\n", c.Name)
		// Only worth a line when the AWS name differs from the object name.
		if c.ExternalName != "" && c.ExternalName != c.Name {
			fmt.Fprintf(tw, "  external name\t%s\n", c.ExternalName)
		}
		fmt.Fprintf(tw, "  apiVersion\t%s\n", dash(c.APIVersion))
		fmt.Fprintf(tw, "  version\t%s\n", dash(c.Version))
		fmt.Fprintf(tw, "  region\t%s\n", dash(c.Region))
		fmt.Fprintf(tw, "  compute\t%s\n", computeMode(c))
		fmt.Fprintf(tw, "  endpoint\t%s\n", endpointSummary(c))
		fmt.Fprintf(tw, "  logging\t%s\n", dash(strings.Join(c.LogTypes, ",")))
		fmt.Fprintf(tw, "  encryption\t%s\n", yesNo(c.EncryptionConfigured))
		fmt.Fprintf(tw, "  auth mode\t%s\n", dash(c.AuthenticationMode))
		fmt.Fprintf(tw, "  support type\t%s\n", dash(c.UpgradeSupportType))
		fmt.Fprintf(tw, "  deletion policy\t%s\n", dash(c.DeletionPolicy))
		if len(c.ManagementPolicies) > 0 {
			fmt.Fprintf(tw, "  mgmt policies\t%s\n", strings.Join(c.ManagementPolicies, ","))
		}
		fmt.Fprintf(tw, "  unit\t%s/%s\n", c.Origin.Space, c.Origin.UnitSlug)
		_ = tw.Flush()
	} else {
		fprintln(w, "\n  (no Cluster resource in scope — shared infrastructure, or the control plane lives elsewhere)")
	}

	if len(cs.NodeGroups) > 0 {
		fprintln(w, "\nNODE GROUPS")
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "  NAME\tVERSION\tINSTANCE TYPES\tCAPACITY\tMIN\tMAX\tDESIRED\tUNIT")
		ngs := append([]*eks.NodeGroupEntity(nil), cs.NodeGroups...)
		sort.Slice(ngs, func(i, j int) bool { return ngs[i].Name < ngs[j].Name })
		for _, n := range ngs {
			desired := intOrDash(n.DesiredSize)
			if n.DesiredSizeInInitProvider {
				desired += " (init)"
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				n.Name, dash(n.Version), dash(strings.Join(n.InstanceTypes, ",")),
				dash(n.CapacityType), intOrDash(n.MinSize), intOrDash(n.MaxSize), desired,
				n.Origin.UnitSlug)
		}
		_ = tw.Flush()
	}

	if len(cs.Addons) > 0 {
		fprintln(w, "\nADDONS")
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "  NAME\tVERSION\tON UPDATE\tPRESERVE\tUNIT")
		adds := append([]*eks.AddonEntity(nil), cs.Addons...)
		sort.Slice(adds, func(i, j int) bool { return adds[i].AddonName < adds[j].AddonName })
		for _, a := range adds {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n",
				dash(a.AddonName), dash(a.AddonVersion), dash(a.ResolveConflictsOnUpdate),
				yesNo(a.Preserve != nil && *a.Preserve), a.Origin.UnitSlug)
		}
		_ = tw.Flush()
	}

	printSupporting(w, "NETWORKING", cs.Network)
	printSupporting(w, "IAM", cs.IAM)
	printSupporting(w, "OTHER EKS", cs.OtherEKS)
}

func printSupporting(w interface{ Write([]byte) (int, error) }, title string, rs []*eks.ResourceEntity) {
	if len(rs) == 0 {
		return
	}
	fprintln(w, "\n"+title)
	// Summarize by kind rather than listing every subnet and route.
	byKind := map[string][]string{}
	for _, r := range rs {
		byKind[r.Kind] = append(byKind[r.Kind], r.Name)
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, k := range kinds {
		names := byKind[k]
		sort.Strings(names)
		fmt.Fprintf(tw, "  %s\t%d\t%s\n", k, len(names), strings.Join(names, ", "))
	}
	_ = tw.Flush()
}

func computeMode(c *eks.ClusterEntity) string {
	switch {
	case !c.AutoMode.Declared():
		return "classic (managed node groups)"
	case !c.AutoMode.Consistent():
		return "AUTO MODE INCONSISTENT — computeConfig / elasticLoadBalancing / blockStorage must all agree"
	case c.AutoMode.Enabled():
		if len(c.AutoMode.NodePools) > 0 {
			return "auto mode (" + strings.Join(c.AutoMode.NodePools, ",") + ")"
		}
		return "auto mode"
	default:
		return "classic (auto mode explicitly disabled)"
	}
}

func endpointSummary(c *eks.ClusterEntity) string {
	var parts []string
	if c.EndpointPrivateAccess != nil {
		parts = append(parts, "private="+yesNo(*c.EndpointPrivateAccess))
	}
	if c.EndpointPublicAccess != nil {
		parts = append(parts, "public="+yesNo(*c.EndpointPublicAccess))
	}
	if len(c.PublicAccessCIDRs) > 0 {
		parts = append(parts, "cidrs="+strings.Join(c.PublicAccessCIDRs, ","))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func intOrDash(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}
