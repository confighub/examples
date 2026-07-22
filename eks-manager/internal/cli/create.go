// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/base64"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
)

// unitManagedByLabel marks Units this tool authors.
const unitManagedByLabel = "cub-eks"

type createdUnit struct {
	Slug     string `json:"slug"`
	Kind     string `json:"kind"`
	Group    string `json:"group"`
	Revision int64  `json:"revision,omitempty"`
	Error    string `json:"error,omitempty"`
}

type createPlan struct {
	Command      string        `json:"command"`
	Cluster      string        `json:"cluster"`
	Space        string        `json:"space"`
	Region       string        `json:"region"`
	Version      string        `json:"version"`
	AutoMode     bool          `json:"autoMode"`
	Zones        []string      `json:"zones"`
	NAT          string        `json:"nat"`
	DryRun       bool          `json:"dryRun"`
	Units        []createdUnit `json:"units"`
	Placeholders []string      `json:"placeholders,omitempty"`
	Created      int           `json:"created"`
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Generate EKS configuration as ConfigHub Units",
	}
	cmd.AddCommand(newCreateClusterCmd(), newCreateNodeGroupCmd(), newCreateAddonCmd())
	return cmd
}

func newCreateClusterCmd() *cobra.Command {
	var (
		output      string
		commit      cliutil.CommitFlags
		space       string
		region      string
		version     string
		environment string
		autoMode    bool
		nodePools   []string
		autoRoleARN string
		zones       []string
		zoneCount   int
		vpcCIDR     string
		nat         string
		publicAPI   bool
		orphan      bool
		provider    string
		ngName      string
		ngTypes     []string
		ngCapacity  string
		ngMin       int64
		ngMax       int64
		ngDesired   int64
		ngDisk      int64
		addons      []string
	)

	cmd := &cobra.Command{
		Use:   "cluster <name>",
		Short: "Generate the full envelope of Units for a new EKS cluster",
		Long: `create cluster generates every Crossplane managed resource a new EKS cluster
needs — IAM roles and policy attachments, the VPC with public and private subnets
across the chosen availability zones, gateways and route tables, the EKS control
plane, and (in classic mode) node groups and addons — and writes them into a
ConfigHub Space as one Unit per resource.

This is the eksctl 'create cluster' analog, with three differences that matter:

  - The output is the source of record, not a one-shot run. There is no
    CloudFormation stack; the Units are what exists, versioned and diffable.
  - Availability zones are pinned. eksctl randomizes AZ selection per invocation,
    so the same config yields different infrastructure; here the zones are
    written into the Units, and the same input always produces the same output.
  - Nothing is applied. The Units are created; rolling them out is a separate,
    deliberate 'cub unit apply'.

EKS Auto Mode is the default. It removes the NodeGroup resource entirely, and
with it the desiredSize-versus-autoscaler conflict that is currently unfixable
in the provider. Pass --auto-mode=false for classic managed node groups.

Every cross-resource reference is emitted as a Ref or Selector, never a literal
identifier, so the resulting Units can be applied in any order: each managed
resource waits at Synced=False until its dependency exists.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("create EKS cluster %s (%s, k8s %s)", name, region, version))
			if err != nil {
				return err
			}
			if len(zones) == 0 {
				zones = eks.DefaultZones(region, zoneCount)
			}
			if space == "" {
				space = "eks-" + name
			}

			spec := eks.ClusterSpec{
				Name:            name,
				Region:          region,
				Version:         version,
				Environment:     environment,
				AutoMode:        autoMode,
				NodePools:       nodePools,
				AutoNodeRoleARN: autoRoleARN,
				Zones:           zones,
				VPCCIDR:         vpcCIDR,
				NAT:             nat,
				PublicEndpoint:  publicAPI,
				Orphan:          orphan,
				ProviderConfig:  provider,
			}
			if !autoMode {
				spec.NodeGroups = []eks.NodeGroupSpec{{
					Name: ngName, InstanceTypes: ngTypes, CapacityType: ngCapacity,
					MinSize: ngMin, MaxSize: ngMax, DesiredSize: ngDesired, DiskSize: ngDisk,
				}}
				for _, a := range addons {
					n, v, _ := strings.Cut(a, "=")
					spec.Addons = append(spec.Addons, eks.AddonSpec{Name: n, Version: v})
				}
			}

			units, err := eks.Generate(spec)
			if err != nil {
				return err
			}

			plan := createPlan{
				Command: "create cluster", Cluster: name, Space: space,
				Region: region, Version: version, AutoMode: autoMode,
				Zones: zones, NAT: nat, DryRun: dryRun,
			}
			for _, u := range units {
				plan.Units = append(plan.Units, createdUnit{Slug: u.Slug, Kind: u.Kind, Group: u.Group})
				if strings.Contains(u.YAML, eks.Placeholder) {
					plan.Placeholders = append(plan.Placeholders, u.Slug)
				}
			}

			if dryRun {
				if output == outputTable {
					printCreatePlan(cmd, plan, units)
					return nil
				}
				return printJSON(cmd.OutOrStdout(), plan)
			}

			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			sp, err := cubapi.EnsureSpace(cmd.Context(), client, goclientnew.Space{
				Slug: space,
				Labels: map[string]string{
					"Cluster":     name,
					"Region":      region,
					"Provider":    "aws",
					"Environment": environment,
					"app":         unitManagedByLabel,
				},
			})
			if err != nil {
				return fmt.Errorf("create space %s: %w", space, err)
			}

			for i, u := range units {
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
					plan.Units[i].Error = err.Error()
					continue
				}
				plan.Units[i].Revision = created.HeadRevisionNum
				plan.Created++
			}

			if output == outputTable {
				printCreateResult(cmd, plan)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), plan)
		},
	}

	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "Space to create (default \"eks-<name>\")")
	f.StringVar(&region, "region", "", "AWS region (required)")
	f.StringVar(&version, "version", "1.34", "Kubernetes version")
	f.StringVar(&environment, "environment", "", "Environment label for the Space and AWS tags")
	f.BoolVar(&autoMode, "auto-mode", true, "use EKS Auto Mode (recommended); --auto-mode=false for managed node groups")
	f.StringSliceVar(&nodePools, "node-pools", []string{"general-purpose", "system"}, "Auto Mode built-in node pools")
	f.StringVar(&autoRoleARN, "auto-node-role-arn", "", "literal ARN for Auto Mode's computeConfig.nodeRoleArn (no Ref exists); a placeholder is written when omitted")
	f.StringSliceVar(&zones, "zones", nil, "availability zones (default: the first --zone-count zones of the region)")
	f.IntVar(&zoneCount, "zone-count", 3, "how many availability zones to derive when --zones is not given")
	f.StringVar(&vpcCIDR, "vpc-cidr", "10.20.0.0/16", "VPC CIDR block (must be a /16)")
	f.StringVar(&nat, "nat", eks.NATSingle, "NAT gateway strategy: single | per-az | none")
	f.BoolVar(&publicAPI, "public-endpoint", false, "expose the Kubernetes API endpoint publicly")
	f.BoolVar(&orphan, "orphan", false, "set deletionPolicy: Orphan so deleting a Unit leaves the AWS resource")
	f.StringVar(&provider, "provider-config", "default", "Crossplane ProviderConfig name")
	f.StringVar(&ngName, "node-group", "system", "node group name (classic mode)")
	f.StringSliceVar(&ngTypes, "instance-types", []string{"m6i.large"}, "node group instance types (classic mode)")
	f.StringVar(&ngCapacity, "capacity-type", "ON_DEMAND", "ON_DEMAND | SPOT (classic mode)")
	f.Int64Var(&ngMin, "nodes-min", 2, "node group minimum size (classic mode)")
	f.Int64Var(&ngMax, "nodes-max", 6, "node group maximum size (classic mode)")
	f.Int64Var(&ngDesired, "nodes", 2, "node group desired size (classic mode)")
	f.Int64Var(&ngDisk, "node-disk-size", 80, "node group disk size in GiB (classic mode)")
	f.StringSliceVar(&addons, "addons", []string{"vpc-cni", "coredns", "kube-proxy"}, "addons as name or name=version (classic mode)")
	_ = cmd.MarkFlagRequired("region")
	return cmd
}

func unitLabels(u eks.GeneratedUnit) map[string]string {
	labels := map[string]string{"managed-by": unitManagedByLabel}
	for k, v := range u.Labels {
		labels[k] = v
	}
	return labels
}

func printCreatePlan(cmd *cobra.Command, plan createPlan, units []eks.GeneratedUnit) {
	out := cmd.OutOrStdout()
	mode := "auto mode"
	if !plan.AutoMode {
		mode = "classic (managed node groups)"
	}
	fprintln(out, fmt.Sprintf("Dry run — would create Space %s and %d Units for EKS cluster %q",
		plan.Space, len(plan.Units), plan.Cluster))
	fprintln(out, fmt.Sprintf("  region %s | k8s %s | %s | zones %s | nat %s",
		plan.Region, plan.Version, mode, strings.Join(plan.Zones, ","), plan.NAT))
	fprintln(out, "")

	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "  UNIT\tGROUP\tKIND")
	for _, u := range plan.Units {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", u.Slug, u.Group, u.Kind)
	}
	_ = tw.Flush()

	if len(plan.Placeholders) > 0 {
		fprintln(out, "")
		fprintln(out, fmt.Sprintf("  %d Unit(s) contain %s and cannot be applied until filled: %s",
			len(plan.Placeholders), eks.Placeholder, strings.Join(plan.Placeholders, ", ")))
		fprintln(out, "  (Auto Mode's computeConfig.nodeRoleArn has no Ref in the provider, so the")
		fprintln(out, "   node role ARN must be supplied once the Role exists, or via --auto-node-role-arn.)")
	}
	fprintln(out, "")
	fprintln(out, fmt.Sprintf("Re-run with --commit --change-desc \"…\" to write. Nothing is applied to a cluster;"))
	fprintln(out, "rolling out is a separate `cub unit apply`.")
}

func printCreateResult(cmd *cobra.Command, plan createPlan) {
	out := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "UNIT\tGROUP\tKIND\tREVISION\tERROR")
	for _, u := range plan.Units {
		rev := "-"
		if u.Revision > 0 {
			rev = fmt.Sprintf("%d", u.Revision)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", u.Slug, u.Group, u.Kind, rev, dash(u.Error))
	}
	_ = tw.Flush()
	fprintln(out, fmt.Sprintf("\nCreated %d of %d Units in Space %s.", plan.Created, len(plan.Units), plan.Space))
	if len(plan.Placeholders) > 0 {
		fprintln(out, fmt.Sprintf("%d Unit(s) still contain %s and will not apply until filled: %s",
			len(plan.Placeholders), eks.Placeholder, strings.Join(plan.Placeholders, ", ")))
	}
	fprintln(out, fmt.Sprintf("Inspect with: %s get %s", InvocationName(), plan.Cluster))
}
