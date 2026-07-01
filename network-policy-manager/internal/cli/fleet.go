// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/network-policy-manager/internal/cub"
	"github.com/confighub/examples/network-policy-manager/internal/netpol"
	"github.com/confighub/examples/network-policy-manager/internal/snapshot"
)

func newFleetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Fleet-wide NetworkPolicy operations",
	}
	cmd.AddCommand(newFleetDefaultDenyCmd())
	return cmd
}

// --- fleet default-deny: remediate every uncovered namespace at once ---

type fleetDenyItem struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Space     string `json:"space"`
	Unit      string `json:"unit"`
	Action    string `json:"action"` // create | created | error | skipped
	Error     string `json:"error,omitempty"`

	dest     createDest
	manifest string
}

func newFleetDefaultDenyCmd() *cobra.Command {
	var output, clusterFilter string
	var egress bool
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "default-deny",
		Short: "Create a default-deny NetworkPolicy for every namespace that lacks one (dry-run unless --commit)",
		Long: `fleet default-deny finds every namespace that has workloads but no default-deny
ingress NetworkPolicy — the coverage gap the analyzers report — and authors a
default-deny Unit for each, in the Space its workloads live in. It is the bulk
form of 'default-deny', driven directly by the coverage analysis.

Namespaces that already have a default-deny ingress are left alone (so re-runs
are idempotent). With --egress the generated policies also deny egress while
allowing DNS. Scope with --where (or a label shorthand) and/or --cluster.

Dry run unless --commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("create default-deny for all uncovered namespaces")
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, filter.predicate())
			if err != nil {
				return err
			}

			items, err := planFleetDefaultDeny(snap, clusterFilter, egress)
			if err != nil {
				return err
			}
			if !dryRun {
				for i := range items {
					if items[i].Action != "create" {
						continue
					}
					if _, err := cub.CreateUnit(cmd.Context(), client, buildUnit(items[i].dest, items[i].Unit, items[i].manifest, changeDesc)); err != nil {
						items[i].Action, items[i].Error = "error", err.Error()
					} else {
						items[i].Action = "created"
					}
				}
			}
			return reportFleetDefaultDeny(cmd, items, dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "limit to this cluster (Target or Space slug)")
	cmd.Flags().BoolVar(&egress, "egress", false, "also deny egress (allowing DNS egress to kube-dns)")
	commit.Bind(cmd)
	return cmd
}

func planFleetDefaultDeny(snap *snapshot.Snapshot, clusterFilter string, egress bool) ([]fleetDenyItem, error) {
	var items []fleetDenyItem
	clusters := make([]string, 0, len(snap.Clusters))
	for name := range snap.Clusters {
		clusters = append(clusters, name)
	}
	sort.Strings(clusters)
	for _, cluster := range clusters {
		if clusterFilter != "" && cluster != clusterFilter {
			continue
		}
		nss, _ := snap.Clusters[cluster].Coverage()
		for _, nc := range nss {
			if nc.Workloads == 0 || nc.DefaultDenyIngress {
				continue // covered, or no workloads to protect
			}
			dest, err := resolveCreateDest(snap, nc.Namespace, cluster, "")
			if err != nil {
				items = append(items, fleetDenyItem{
					Cluster: cluster, Namespace: nc.Namespace, Action: "skipped", Error: err.Error(),
				})
				continue
			}
			slug, manifest := netpol.DefaultDenyYAML(nc.Namespace, egress)
			items = append(items, fleetDenyItem{
				Cluster: cluster, Namespace: nc.Namespace, Space: dest.spaceSlug, Unit: slug,
				Action: "create", dest: dest, manifest: manifest,
			})
		}
	}
	return items, nil
}

func reportFleetDefaultDeny(cmd *cobra.Command, items []fleetDenyItem, dryRun bool, output string) error {
	if output == outputJSON {
		return printJSON(cmd.OutOrStdout(), items)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CLUSTER\tNAMESPACE\tSPACE\tUNIT\tACTION")
	created, planned, skipped := 0, 0, 0
	for _, it := range items {
		action := it.Action
		if it.Error != "" {
			action += " (" + it.Error + ")"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", it.Cluster, it.Namespace, it.Space, it.Unit, action)
		switch it.Action {
		case "created":
			created++
		case "create":
			planned++
		case "skipped", "error":
			skipped++
		}
	}
	_ = tw.Flush()
	if dryRun {
		fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d namespace(s) would get a default-deny (%d skipped). Re-run with --commit --change-desc \"…\".", planned, skipped))
	} else {
		fprintln(cmd.OutOrStdout(), fmt.Sprintf("\nCreated %d default-deny Unit(s) (%d skipped; not applied — apply when ready).", created, skipped))
	}
	return nil
}

// --- promote: pull downstream NetworkPolicy Units up to their upstream ---

func newPromoteCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Upgrade downstream NetworkPolicy Units to their upstream head (dry-run unless --commit)",
		Long: `promote performs an override-preserving upgrade of Kubernetes/YAML Units that are
behind their upstream (UpstreamRevisionNum < the upstream's head) — the variant
propagation path: a baseline authored in a base Space flows to the cluster
Spaces cloned from it, keeping each Space's local customizations.

Scope to the policies you mean with --where (e.g. "Slug LIKE 'default-deny-%'"
or a Space-label predicate). Dry run unless --commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("promote NetworkPolicy Units from upstream")
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			where := "ToolchainType = 'Kubernetes/YAML' AND UpstreamRevisionNum > 0"
			if p := filter.predicate(); p != "" {
				where += " AND " + p
			}
			ch := cubapi.Change{}
			if !dryRun {
				ch = cubapi.Change{Description: changeDesc}
			}
			res, err := cubapi.UpgradeUnits(cmd.Context(), client, where, ch)
			if err != nil {
				return err
			}
			return reportPromote(cmd, res, dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	return cmd
}

func reportPromote(cmd *cobra.Command, res *cubapi.Result, dryRun bool, output string) error {
	var changed []string
	for _, o := range res.Outcomes {
		if !o.Success {
			return fmt.Errorf("promote failed on %s/%s: %s", o.SpaceSlug, o.UnitSlug, o.Error)
		}
		if o.HasMutations {
			changed = append(changed, o.SpaceSlug+"/"+o.UnitSlug)
		}
	}
	sort.Strings(changed)
	if output == outputJSON {
		return printJSON(cmd.OutOrStdout(), map[string]any{"dryRun": dryRun, "changed": changed})
	}
	out := cmd.OutOrStdout()
	if len(changed) == 0 {
		fprintln(out, "No Units are behind their upstream — nothing to promote.")
		return nil
	}
	verb := "Would upgrade"
	if !dryRun {
		verb = "Upgraded"
	}
	fprintln(out, fmt.Sprintf("%s %d Unit(s):", verb, len(changed)))
	for _, u := range changed {
		fprintln(out, "  "+u)
	}
	if dryRun {
		fprintln(out, "\nRe-run with --commit --change-desc \"…\" to promote.")
	}
	return nil
}
