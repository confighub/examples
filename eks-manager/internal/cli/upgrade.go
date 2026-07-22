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

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

// upgradeStage is one ordered step of a control-plane upgrade.
type upgradeStage struct {
	Order      int            `json:"order"`
	Stage      string         `json:"stage"`
	Space      string         `json:"space"`
	Unit       string         `json:"unit"`
	Resource   string         `json:"resource"`
	From       string         `json:"from,omitempty"`
	To         string         `json:"to"`
	Disruption eks.Disruption `json:"disruption"`
	Note       string         `json:"note,omitempty"`
	// Applied is true when this stage was written in this run. Only stage 1 is
	// ever written; the rest are reported for the operator to run in order.
	Applied bool `json:"applied"`
}

type upgradePlan struct {
	Cluster string         `json:"cluster"`
	From    string         `json:"from"`
	To      string         `json:"to"`
	Stages  []upgradeStage `json:"stages"`
	DryRun  bool           `json:"dryRun"`
	Mutated int            `json:"mutated"`
	// ControlPlaneStage is the order number of the control-plane stage — the
	// only one this command writes. Catch-up stages may precede it.
	ControlPlaneStage int `json:"controlPlaneStage"`
	// Blocked is true when node groups must be caught up before the control
	// plane can move at all, so nothing is written this run.
	Blocked bool `json:"blocked"`
}

func newUpgradeClusterCmd() *cobra.Command {
	var output string
	var commit cliutil.CommitFlags
	var to string

	cmd := &cobra.Command{
		Use:   "cluster <cluster>",
		Short: "Plan and start a staged control-plane upgrade",
		Long: `upgrade cluster raises the EKS control-plane version, and reports the ordered
plan the rest of the upgrade must follow.

EKS constrains the order: control plane first, then addons, then node groups —
and only one minor version at a time, never backwards. Nothing downstream
enforces any of that. The Cluster CRD marks version as an ordinary optional
field with no validation, and the provider passes it straight through, so an
illegal transition surfaces as an InvalidParameterException and a permanently
Synced=False managed resource rather than an error at the point of change.

So this command checks legality first, then writes only the control-plane stage
and prints the remaining stages for you to run once the control plane is
healthy. It deliberately does not write the node-group stages: each of those
drains and replaces every node in its group, and they must not start until the
new control plane is up.

Dry-run by default; pass --commit --change-desc "..." to write stage 1.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cluster := args[0]
			changeDesc, dryRun, err := commit.Validate(
				fmt.Sprintf("upgrade cluster %s control plane to %s", cluster, to))
			if err != nil {
				return err
			}
			target, ok := eks.ParseVersion(to)
			if !ok {
				return fmt.Errorf("--to %q is not a Kubernetes major.minor version (e.g. 1.34)", to)
			}

			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, "")
			if err != nil {
				return err
			}
			cs, ok := snap.Clusters[cluster]
			if !ok || cs.Control == nil {
				return fmt.Errorf("cluster %q not found (or has no Cluster resource); run `%s snapshot` to list clusters",
					cluster, InvocationName())
			}
			c := cs.Control

			current, ok := eks.ParseVersion(c.Version)
			if !ok {
				return fmt.Errorf("cluster %s has no parseable version (%q); set one before upgrading", cluster, c.Version)
			}
			// The check nothing downstream performs.
			if legal, why := eks.UpgradeLegal(current, target); !legal {
				return fmt.Errorf("%s", why)
			}
			if current.Compare(target) == 0 {
				fprintln(cmd.OutOrStdout(), fmt.Sprintf("Cluster %s is already on %s.", cluster, to))
				return nil
			}

			plan := buildUpgradePlan(cs, current, target, dryRun)

			// Refuse to move the control plane while a node group is still
			// behind: doing so strands it further behind than EKS supports, and
			// node groups only move one minor at a time, so it cannot simply
			// catch up afterwards.
			if plan.Blocked && !dryRun {
				if output == outputTable {
					printUpgradePlan(cmd, plan)
				} else {
					_ = printJSON(cmd.OutOrStdout(), plan)
				}
				return fmt.Errorf("refused: %d node group(s) are behind the current control plane and must be caught up first",
					countCatchup(plan))
			}

			if !dryRun {
				ref := unitRef{spaceSlug: c.Origin.Space, unitSlug: c.Origin.UnitSlug}
				sp, err := cubapi.ResolveSpace(cmd.Context(), client, c.Origin.Space)
				if err != nil {
					return fmt.Errorf("resolve space %s: %w", c.Origin.Space, err)
				}
				ref.spaceID = sp.SpaceID
				res, err := cub.SetPath(cmd.Context(), client, "set-string-path",
					c.APIVersion+"/Cluster", "spec.forProvider.version", target.String(),
					ref.selector(), cubapi.Change{Description: changeDesc})
				if err != nil {
					return fmt.Errorf("set control-plane version: %w", err)
				}
				for _, o := range res.Outcomes {
					if o.HasMutations {
						plan.Mutated++
					}
				}
				for i := range plan.Stages {
					if plan.Stages[i].Order == plan.ControlPlaneStage {
						plan.Stages[i].Applied = true
					}
				}
			}

			if output == outputTable {
				printUpgradePlan(cmd, plan)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), plan)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&to, "to", "", "target Kubernetes version, one minor forward (required)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func buildUpgradePlan(cs *eks.ClusterSet, from, to eks.Version, dryRun bool) upgradePlan {
	c := cs.Control
	plan := upgradePlan{Cluster: cs.Cluster, From: from.String(), To: to.String(), DryRun: dryRun}
	order := 1

	ngsSorted := append([]*eks.NodeGroupEntity(nil), cs.NodeGroups...)
	sort.Slice(ngsSorted, func(i, j int) bool { return ngsSorted[i].Name < ngsSorted[j].Name })

	// Catch-up stages come FIRST. A node group already behind the *current*
	// control plane falls further behind the moment the control plane moves, and
	// EKS bounds how far a node group may lag. Upgrading the control plane first
	// would strand it — and because node groups also move one minor at a time,
	// a group two or more behind needs several passes before the control plane
	// can advance at all.
	for _, n := range ngsSorted {
		if n.Version == "" {
			continue // unpinned: tracks the control plane, never behind
		}
		v, ok := eks.ParseVersion(n.Version)
		if !ok || v.Compare(from) >= 0 {
			continue
		}
		next := eks.Version{Major: v.Major, Minor: v.Minor + 1}
		note := "behind the current control plane; catch up before the control plane moves"
		if next.Compare(from) < 0 {
			note = fmt.Sprintf("%d minor versions behind; this is the first of several passes "+
				"(node groups move one minor at a time)", eks.MinorSkew(from, v))
		}
		plan.Stages = append(plan.Stages, upgradeStage{
			Order: order, Stage: "nodegroup-catchup",
			Space: n.Origin.Space, Unit: n.Origin.UnitSlug, Resource: n.Name,
			From: n.Version, To: next.String(),
			Disruption: eks.DisruptionRolling,
			Note:       note,
		})
		order++
	}

	plan.Stages = append(plan.Stages, upgradeStage{
		Order: order, Stage: "control-plane",
		Space: c.Origin.Space, Unit: c.Origin.UnitSlug, Resource: c.Name,
		From: from.String(), To: to.String(),
		Disruption: eks.DisruptionLow,
		Note:       "multi-minute; EKS allows only one cluster update at a time",
	})
	controlPlaneOrder := order
	plan.ControlPlaneStage = controlPlaneOrder
	order++

	// Addons next: their compatibility is tied to the control-plane version.
	addons := append([]*eks.AddonEntity(nil), cs.Addons...)
	sort.Slice(addons, func(i, j int) bool { return addons[i].AddonName < addons[j].AddonName })
	for _, a := range addons {
		plan.Stages = append(plan.Stages, upgradeStage{
			Order: order, Stage: "addon",
			Space: a.Origin.Space, Unit: a.Origin.UnitSlug, Resource: a.AddonName,
			From: a.AddonVersion, To: "(compatible with " + to.String() + ")",
			Disruption: eks.DisruptionLow,
			Note:       "check the addon version compatible with " + to.String() + " before setting it",
		})
		order++
	}

	// Node groups last, and only those actually behind.
	ngs := ngsSorted
	for _, n := range ngs {
		if n.Version == "" {
			plan.Stages = append(plan.Stages, upgradeStage{
				Order: order, Stage: "nodegroup",
				Space: n.Origin.Space, Unit: n.Origin.UnitSlug, Resource: n.Name,
				From: "(unpinned)", To: to.String(),
				Disruption: eks.DisruptionRolling,
				Note:       "unpinned: tracks the control plane, so no edit is needed — but nodes still roll",
			})
			order++
			continue
		}
		if v, ok := eks.ParseVersion(n.Version); ok && v.Compare(to) >= 0 {
			continue // already at or beyond the target
		}
		plan.Stages = append(plan.Stages, upgradeStage{
			Order: order, Stage: "nodegroup",
			Space: n.Origin.Space, Unit: n.Origin.UnitSlug, Resource: n.Name,
			From: n.Version, To: to.String(),
			Disruption: eks.DisruptionRolling,
			Note:       "drains and replaces every node in the group",
		})
		order++
	}
	for _, st := range plan.Stages {
		if st.Stage == "nodegroup-catchup" {
			plan.Blocked = true
			break
		}
	}
	return plan
}

func printUpgradePlan(cmd *cobra.Command, p upgradePlan) {
	out := cmd.OutOrStdout()
	fprintln(out, fmt.Sprintf("Upgrade %s: %s -> %s", p.Cluster, p.From, p.To))
	fprintln(out, "")

	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tSTAGE\tRESOURCE\tFROM\tTO\tDISRUPTION\tSTATUS")
	for _, s := range p.Stages {
		status := "pending"
		if s.Applied {
			status = "written"
		} else if s.Order == p.ControlPlaneStage && p.DryRun && !p.Blocked {
			status = "would write"
		} else if s.Stage == "nodegroup-catchup" {
			status = "REQUIRED FIRST"
		}
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.Order, s.Stage, s.Resource, dash(s.From), s.To, string(s.Disruption), status)
	}
	_ = tw.Flush()

	fprintln(out, "")
	for _, s := range p.Stages {
		if s.Note != "" {
			fprintln(out, fmt.Sprintf("  %d. %s: %s", s.Order, s.Resource, s.Note))
		}
	}

	fprintln(out, "")
	if p.Blocked {
		fprintln(out, "BLOCKED — node groups are behind the current control plane and must be caught up")
		fprintln(out, "first. Moving the control plane now would strand them further behind than EKS")
		fprintln(out, "supports, and node groups only advance one minor version at a time.")
		fprintln(out, "")
		fprintln(out, "Run these, apply each, and wait for the nodes to roll:")
		for _, s := range p.Stages {
			if s.Stage != "nodegroup-catchup" {
				continue
			}
			fprintln(out, fmt.Sprintf("  %s upgrade nodegroup %s/%s --to %s --commit --change-desc \"…\"",
				InvocationName(), s.Space, s.Unit, s.To))
		}
		fprintln(out, "")
		fprintln(out, "Then re-run this command.")
		return
	}

	if p.DryRun {
		fprintln(out, fmt.Sprintf("Dry run — nothing written. Re-run with --commit --change-desc \"…\" to write stage %d",
			p.ControlPlaneStage))
		fprintln(out, "(the control plane). The later stages are yours to run once it is healthy:")
	} else {
		fprintln(out, fmt.Sprintf("Stage %d written (%d resource(s)). It is not applied — roll it out with `cub unit apply`,",
			p.ControlPlaneStage, p.Mutated))
		fprintln(out, "wait for the control plane to report ACTIVE, then continue:")
	}
	for _, s := range p.Stages {
		if s.Order <= p.ControlPlaneStage {
			continue
		}
		switch s.Stage {
		case "nodegroup":
			if s.From == "(unpinned)" {
				continue
			}
			fprintln(out, fmt.Sprintf("  %s upgrade nodegroup %s/%s --to %s --commit --change-desc \"…\"",
				InvocationName(), s.Space, s.Unit, s.To))
		case "addon":
			fprintln(out, fmt.Sprintf("  # set %s/%s addonVersion to the release compatible with %s",
				s.Space, s.Unit, p.To))
		}
	}
}

// countCatchup reports how many node groups must be upgraded before the control
// plane may advance.
func countCatchup(p upgradePlan) int {
	n := 0
	for _, s := range p.Stages {
		if s.Stage == "nodegroup-catchup" {
			n++
		}
	}
	return n
}
