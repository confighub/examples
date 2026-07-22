// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

// addonsManagedByAutoMode are the addons EKS Auto Mode provides itself. Adding
// them as separate Addon resources on an Auto Mode cluster duplicates
// functionality the control plane already manages.
var addonsManagedByAutoMode = map[string]bool{
	"vpc-cni": true, "coredns": true, "kube-proxy": true,
	"aws-ebs-csi-driver": true, "aws-load-balancer-controller": true,
}

type addResourceReport struct {
	Command  string   `json:"command"`
	Cluster  string   `json:"cluster"`
	Space    string   `json:"space"`
	Unit     string   `json:"unit"`
	Kind     string   `json:"kind"`
	DryRun   bool     `json:"dryRun"`
	Revision int64    `json:"revision,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	YAML     string   `json:"yaml,omitempty"`
}

// resolveCluster loads the fleet and returns the requested cluster along with
// the context a single generated resource needs. The context is read from the
// existing control plane, so a resource added later matches the one the cluster
// was created with.
func resolveCluster(ctx context.Context, client *cubapi.Client, name string) (*eks.ClusterSet, eks.ClusterContext, error) {
	snap, err := snapshot.Load(ctx, client, "")
	if err != nil {
		return nil, eks.ClusterContext{}, err
	}
	cs, ok := snap.Clusters[name]
	if !ok || cs.Control == nil {
		return nil, eks.ClusterContext{}, fmt.Errorf(
			"cluster %q not found (or has no Cluster resource); run `%s snapshot` to list clusters",
			name, InvocationName())
	}
	c := cs.Control
	cc := eks.ClusterContext{
		Name:        c.Name,
		Region:      c.Region,
		Version:     c.Version,
		Environment: c.Origin.SpaceLabels["Environment"],
		Orphan:      strings.EqualFold(c.DeletionPolicy, "Orphan"),
		// The control plane's own ProviderConfig is not parsed into the model;
		// "default" matches what `create cluster` emits and what the provider
		// falls back to when the field is absent.
		ProviderConfig: "default",
	}
	return cs, cc, nil
}

func createResourceUnit(cmd *cobra.Command, client *cubapi.Client, cs *eks.ClusterSet,
	u eks.GeneratedUnit, r addResourceReport, changeDesc string, dryRun bool, output string) error {

	space := cs.Control.Origin.Space
	r.Space, r.Unit, r.Kind, r.DryRun, r.YAML = space, u.Slug, u.Kind, dryRun, u.YAML

	// A slug collision would otherwise surface as an opaque API error.
	for _, existing := range unitSlugs(cs) {
		if existing == u.Slug {
			return fmt.Errorf("Unit %s/%s already exists; pick another name or edit the existing one",
				space, u.Slug)
		}
	}

	if dryRun {
		if output == outputTable {
			printAddResource(cmd, r)
			return nil
		}
		return printJSON(cmd.OutOrStdout(), r)
	}

	sp, err := cubapi.ResolveSpace(cmd.Context(), client, space)
	if err != nil {
		return fmt.Errorf("resolve space %s: %w", space, err)
	}
	created, err := cub.CreateUnit(cmd.Context(), client, goclientnew.Unit{
		Slug:                  u.Slug,
		DisplayName:           u.Slug,
		Data:                  base64.StdEncoding.EncodeToString([]byte(u.YAML)),
		ToolchainType:         "Kubernetes/YAML",
		SpaceID:               sp.SpaceID,
		Labels:                unitLabels(u),
		LastChangeDescription: changeDesc,
	})
	if err != nil {
		return err
	}
	r.Revision = created.HeadRevisionNum
	if output == outputTable {
		printAddResource(cmd, r)
		return nil
	}
	return printJSON(cmd.OutOrStdout(), r)
}

func unitSlugs(cs *eks.ClusterSet) []string {
	var out []string
	for _, n := range cs.NodeGroups {
		out = append(out, n.Origin.UnitSlug)
	}
	for _, a := range cs.Addons {
		out = append(out, a.Origin.UnitSlug)
	}
	for _, r := range cs.OtherEKS {
		out = append(out, r.Origin.UnitSlug)
	}
	return out
}

func printAddResource(cmd *cobra.Command, r addResourceReport) {
	out := cmd.OutOrStdout()
	for _, w := range r.Warnings {
		fprintln(out, "warning: "+w)
		fprintln(out, "")
	}
	if r.DryRun {
		fprintln(out, fmt.Sprintf("Dry run — would create Unit %s/%s (%s) for cluster %s:",
			r.Space, r.Unit, r.Kind, r.Cluster))
		fprintln(out, "")
		fprintln(out, r.YAML)
		fprintln(out, "Re-run with --commit --change-desc \"…\" to create it. Nothing is applied.")
		return
	}
	fprintln(out, fmt.Sprintf("Created Unit %s/%s (%s, revision %d) for cluster %s.",
		r.Space, r.Unit, r.Kind, r.Revision, r.Cluster))
	fprintln(out, fmt.Sprintf("Not applied — roll it out with: cub unit apply --space %s %s", r.Space, r.Unit))
}

func newCreateNodeGroupCmd() *cobra.Command {
	var (
		output   string
		commit   cliutil.CommitFlags
		types    []string
		capacity string
		min, max int64
		desired  int64
		disk     int64
		version  string
	)
	cmd := &cobra.Command{
		Use:   "nodegroup <cluster> <name>",
		Short: "Add a managed node group to an existing cluster",
		Long: `create nodegroup generates a NodeGroup managed resource for an existing cluster
and stores it as a new Unit in that cluster's Space.

The cluster, node role, and subnets are referenced rather than named literally —
subnets by the tier=private label selector — so the Unit is valid regardless of
the order things are applied.

The node group inherits the cluster's Kubernetes version unless --version is
given. A node group may not run ahead of its control plane, so that is checked
before anything is written.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, ngName := args[0], args[1]
			changeDesc, dryRun, err := commit.Validate(
				fmt.Sprintf("add node group %s to cluster %s", ngName, clusterName))
			if err != nil {
				return err
			}
			if min > max {
				return fmt.Errorf("--nodes-min %d is above --nodes-max %d", min, max)
			}
			if desired < min || desired > max {
				return fmt.Errorf("--nodes %d is outside the range %d..%d", desired, min, max)
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			cs, cc, err := resolveCluster(cmd.Context(), client, clusterName)
			if err != nil {
				return err
			}

			var warnings []string
			if version != "" {
				v, ok := eks.ParseVersion(version)
				if !ok {
					return fmt.Errorf("--version %q is not a Kubernetes major.minor version", version)
				}
				if cp, ok := eks.ParseVersion(cc.Version); ok {
					if v.Compare(cp) > 0 {
						return fmt.Errorf("--version %s is ahead of the control plane (%s); EKS does not support nodes newer than the control plane",
							version, cc.Version)
					}
					if skew := eks.MinorSkew(cp, v); skew > 0 {
						warnings = append(warnings, fmt.Sprintf(
							"the node group will start %d minor version(s) behind the control plane (%s)", skew, cc.Version))
					}
				}
				cc.Version = version
			}
			// Legal on AWS, but usually a misunderstanding of what Auto Mode does.
			if cs.Control.AutoMode.Enabled() {
				warnings = append(warnings,
					"this cluster runs EKS Auto Mode, which manages capacity itself; a managed node group is "+
						"supported alongside it but is rarely what you want")
			}

			u, err := eks.GenerateNodeGroup(cc, eks.NodeGroupSpec{
				Name: ngName, InstanceTypes: types, CapacityType: capacity,
				MinSize: min, MaxSize: max, DesiredSize: desired, DiskSize: disk,
			})
			if err != nil {
				return err
			}
			return createResourceUnit(cmd, client, cs, u,
				addResourceReport{Command: "create nodegroup", Cluster: clusterName, Warnings: warnings},
				changeDesc, dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	f := cmd.Flags()
	f.StringSliceVar(&types, "instance-types", []string{"m6i.large"}, "instance types")
	f.StringVar(&capacity, "capacity-type", "ON_DEMAND", "ON_DEMAND | SPOT")
	f.Int64Var(&min, "nodes-min", 2, "minimum size")
	f.Int64Var(&max, "nodes-max", 6, "maximum size")
	f.Int64Var(&desired, "nodes", 2, "desired size")
	f.Int64Var(&disk, "node-disk-size", 80, "disk size in GiB")
	f.StringVar(&version, "version", "", "Kubernetes version (default: the cluster's)")
	return cmd
}

func newCreateAddonCmd() *cobra.Command {
	var output string
	var commit cliutil.CommitFlags
	var version string

	cmd := &cobra.Command{
		Use:   "addon <cluster> <name>",
		Short: "Add an EKS addon to an existing cluster",
		Long: `create addon generates an Addon managed resource for an existing cluster and
stores it as a new Unit in that cluster's Space.

The addon is written with resolveConflictsOnCreate/OnUpdate: OVERWRITE, because
ConfigHub is the source of record — out-of-band edits to addon configuration are
drift to be overwritten, not state to preserve. It also sets preserve: true, so
deleting the addon record leaves its Kubernetes objects in place; removing
CoreDNS or the CNI outright is an outage.

Omitting --version lets EKS pick the default compatible with the cluster's
Kubernetes version, which is usually what you want until you need to pin.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, addonName := args[0], args[1]
			changeDesc, dryRun, err := commit.Validate(
				fmt.Sprintf("add addon %s to cluster %s", addonName, clusterName))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			cs, cc, err := resolveCluster(cmd.Context(), client, clusterName)
			if err != nil {
				return err
			}

			var warnings []string
			if cs.Control.AutoMode.Enabled() && addonsManagedByAutoMode[addonName] {
				warnings = append(warnings, fmt.Sprintf(
					"EKS Auto Mode already provides %s; adding it as a separate Addon duplicates "+
						"functionality the control plane manages", addonName))
			}

			u, err := eks.GenerateAddon(cc, eks.AddonSpec{Name: addonName, Version: version})
			if err != nil {
				return err
			}
			return createResourceUnit(cmd, client, cs, u,
				addResourceReport{Command: "create addon", Cluster: clusterName, Warnings: warnings},
				changeDesc, dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&version, "version", "", "addon version (default: EKS picks the compatible default)")
	return cmd
}

// newUnitFor builds the Unit payload for a generated resource.
func newUnitFor(u eks.GeneratedUnit, spaceID goclientnew.UUID, changeDesc string) goclientnew.Unit {
	return goclientnew.Unit{
		Slug:                  u.Slug,
		DisplayName:           u.Slug,
		Data:                  base64.StdEncoding.EncodeToString([]byte(u.YAML)),
		ToolchainType:         "Kubernetes/YAML",
		SpaceID:               spaceID,
		Labels:                unitLabels(u),
		LastChangeDescription: changeDesc,
	}
}
