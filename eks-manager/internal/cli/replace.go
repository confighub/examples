// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

type replaceReport struct {
	Command    string   `json:"command"`
	Cluster    string   `json:"cluster"`
	Space      string   `json:"space"`
	OldUnit    string   `json:"oldUnit"`
	OldName    string   `json:"oldName"`
	NewUnit    string   `json:"newUnit"`
	NewName    string   `json:"newName"`
	Changes    []string `json:"immutableChanges"`
	NotCarried []string `json:"notCarriedOver,omitempty"`
	DryRun     bool     `json:"dryRun"`
	Revision   int64    `json:"revision,omitempty"`
	YAML       string   `json:"yaml,omitempty"`
	// SourceWedged is true when the original Unit already carries a committed
	// change that cannot reconcile — the usual reason someone reaches for a
	// replacement. Creating the replacement does not clear it.
	SourceWedged    bool   `json:"sourceWedged,omitempty"`
	SourceRevisions string `json:"sourceRevisions,omitempty"`
}

func newReplaceNodeGroupCmd() *cobra.Command {
	var (
		output   string
		commit   cliutil.CommitFlags
		newName  string
		types    []string
		capacity string
		amiType  string
		disk     int64
		min, max int64
		desired  int64
	)

	cmd := &cobra.Command{
		Use:   "replace-nodegroup <space>/<unit>",
		Short: "Blue/green replacement for a node group's immutable fields",
		Long: `replace-nodegroup changes a node group field that cannot be changed in place.

Instance types, AMI type, capacity type, disk size, subnets, and the node role
are all immutable in AWS. Terraform would destroy and recreate the node group;
Crossplane refuses — it returns "refuse to update the external resource because
the following update requires replacing it" and retries forever, so an in-place
edit gives you a committed revision, a clean diff, a successful apply, and
nothing actually happening.

For a Crossplane managed resource, an immutable field is identity, not
configuration. Changing one is not an edit; it is a different resource. So this
command creates a *new* node group Unit carrying the change, alongside the
original, and leaves both in place for a controlled swap.

eksctl has no command for this at all — for self-managed node groups it is a
documented manual procedure.

What this command does:

  1. Reads the existing node group.
  2. Verifies at least one immutable field actually changes (otherwise it tells
     you to use 'scale' or 'upgrade' instead).
  3. Derives a new name — a node group name is its AWS identity, so the two must
     differ to coexist.
  4. Creates the replacement Unit. Nothing is applied.

What it deliberately does NOT do: apply the new node group, drain the old one, or
delete anything. Draining is a converging imperative loop over live pod state and
is out of scope by design; the remaining steps are printed for you to run.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			changeDesc, dryRun, err := commit.Validate(
				fmt.Sprintf("replace node group %s", target))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			ref, err := parseUnitRef(cmd.Context(), client, target)
			if err != nil {
				return err
			}

			// Load the whole fleet: the replacement needs the cluster's context
			// and the set of names already taken in that cluster.
			snap, err := snapshot.Load(cmd.Context(), client, "")
			if err != nil {
				return err
			}
			var (
				cs  *eks.ClusterSet
				old *eks.NodeGroupEntity
			)
			for _, set := range snap.Clusters {
				for _, n := range set.NodeGroups {
					if n.Origin.Space == ref.spaceSlug && n.Origin.UnitSlug == ref.unitSlug {
						cs, old = set, n
					}
				}
			}
			if old == nil {
				return fmt.Errorf("%s/%s does not contain an EKS NodeGroup", ref.spaceSlug, ref.unitSlug)
			}
			if cs.Control == nil {
				return fmt.Errorf("cluster for %s/%s has no Cluster resource in scope", ref.spaceSlug, ref.unitSlug)
			}

			// Start from the existing node group and layer the requested changes.
			proposed := eks.NodeGroupSpecFrom(old)
			if cmd.Flags().Changed("instance-types") {
				proposed.InstanceTypes = types
			}
			if cmd.Flags().Changed("capacity-type") {
				proposed.CapacityType = capacity
			}
			if cmd.Flags().Changed("ami-type") {
				proposed.AMIType = amiType
			}
			if cmd.Flags().Changed("node-disk-size") {
				proposed.DiskSize = disk
			}
			if cmd.Flags().Changed("nodes-min") {
				proposed.MinSize = min
			}
			if cmd.Flags().Changed("nodes-max") {
				proposed.MaxSize = max
			}
			if cmd.Flags().Changed("nodes") {
				proposed.DesiredSize = desired
			}

			// Replacement is destructive and slow; refuse to do it for a change
			// that could simply be applied in place.
			changes := eks.ImmutableDiff(old.APIVersion, old, proposed)
			if len(changes) == 0 {
				return fmt.Errorf(
					"no immutable field changes: nothing here requires a replacement.\n"+
						"Use `%s scale nodegroup %s` for capacity or `%s upgrade nodegroup %s` for a version change",
					InvocationName(), target, InvocationName(), target)
			}

			taken := map[string]bool{}
			for _, n := range cs.NodeGroups {
				taken[n.Name] = true
			}
			proposed.Name = newName
			if proposed.Name == "" {
				proposed.Name = eks.NextNodeGroupName(old.Name, taken)
			}
			if taken[proposed.Name] {
				return fmt.Errorf("node group %q already exists in cluster %s; pick another --name",
					proposed.Name, cs.Cluster)
			}

			cc := eks.ClusterContext{
				Name:           cs.Control.Name,
				Region:         cs.Control.Region,
				Version:        old.Version,
				Environment:    cs.Control.Origin.SpaceLabels["Environment"],
				Orphan:         strings.EqualFold(old.DeletionPolicy, "Orphan"),
				ProviderConfig: "default",
			}
			if cc.Version == "" {
				cc.Version = cs.Control.Version
			}
			u, err := eks.GenerateNodeGroup(cc, proposed)
			if err != nil {
				return err
			}

			r := replaceReport{
				Command: "replace-nodegroup", Cluster: cs.Cluster, Space: ref.spaceSlug,
				OldUnit: ref.unitSlug, OldName: old.Name,
				NewUnit: u.Slug, NewName: proposed.Name,
				Changes: changes, DryRun: dryRun, YAML: u.YAML,
				// Honesty about what the model does not round-trip.
				NotCarried: notCarriedOver(old),
			}

			// The common way to arrive here is having already tried the edit in
			// place: the original Unit then carries a committed change that can
			// never reconcile. The replacement does not fix that — the original
			// stays wedged until its pending change is reverted.
			if meta, ok := snap.Units[old.Origin.UnitID]; ok && meta.LastAppliedRevisionNum > 0 &&
				meta.HeadRevisionNum > meta.LastAppliedRevisionNum {
				oldDocs, err1 := cub.RevisionDocs(cmd.Context(), client, meta.SpaceID, meta.UnitID, meta.LastAppliedRevisionNum)
				newDocs, err2 := cub.RevisionDocs(cmd.Context(), client, meta.SpaceID, meta.UnitID, meta.HeadRevisionNum)
				if err1 == nil && err2 == nil {
					for key, nw := range newDocs {
						o, ok := oldDocs[key]
						if !ok {
							continue
						}
						apiVersion, kind, name, _ := eks.ResourceMeta(nw)
						rc := eks.ClassifyResource(apiVersion+"/"+kind, name, eks.DiffPaths(o, nw))
						if rc.Blocks {
							r.SourceWedged = true
							r.SourceRevisions = fmt.Sprintf("%d->%d",
								meta.LastAppliedRevisionNum, meta.HeadRevisionNum)
						}
					}
				}
			}

			if !dryRun {
				created, err := cub.CreateUnit(cmd.Context(), client, newUnitFor(u, ref.spaceID, changeDesc))
				if err != nil {
					return err
				}
				r.Revision = created.HeadRevisionNum
			}

			if output == outputTable {
				printReplaceReport(cmd, r)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), r)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	f := cmd.Flags()
	f.StringVar(&newName, "name", "", "name for the replacement (default: the current name with an incremented -vN suffix)")
	f.StringSliceVar(&types, "instance-types", nil, "new instance types")
	f.StringVar(&capacity, "capacity-type", "", "new capacity type: ON_DEMAND | SPOT")
	f.StringVar(&amiType, "ami-type", "", "new AMI type")
	f.Int64Var(&disk, "node-disk-size", 0, "new disk size in GiB")
	f.Int64Var(&min, "nodes-min", 0, "minimum size for the replacement")
	f.Int64Var(&max, "nodes-max", 0, "maximum size for the replacement")
	f.Int64Var(&desired, "nodes", 0, "desired size for the replacement")
	return cmd
}

// notCarriedOver lists configuration the model does not parse and therefore
// cannot reproduce on the replacement. Surfacing it is the point: silently
// dropping a taint would strand workloads on a node group that no longer repels
// them.
func notCarriedOver(old *eks.NodeGroupEntity) []string {
	var out []string
	if old.LaunchTemplateID != "" || old.LaunchTemplateName != "" {
		out = append(out, "launchTemplate (the replacement uses the cluster defaults)")
	}
	out = append(out, "labels, taints, and updateConfig are re-emitted from defaults, not copied")
	return out
}

func printReplaceReport(cmd *cobra.Command, r replaceReport) {
	out := cmd.OutOrStdout()
	fprintln(out, fmt.Sprintf("Replace node group %s -> %s (cluster %s)", r.OldName, r.NewName, r.Cluster))
	fprintln(out, "")
	fprintln(out, "  Immutable changes forcing the replacement:")
	for _, c := range r.Changes {
		fprintln(out, "    "+c)
	}
	if len(r.NotCarried) > 0 {
		fprintln(out, "")
		fprintln(out, "  Not carried over from the original:")
		for _, n := range r.NotCarried {
			fprintln(out, "    "+n)
		}
	}

	fprintln(out, "")
	if r.DryRun {
		fprintln(out, fmt.Sprintf("Dry run — would create Unit %s/%s:", r.Space, r.NewUnit))
		fprintln(out, "")
		fprintln(out, r.YAML)
		fprintln(out, "Re-run with --commit --change-desc \"…\" to create it.")
		return
	}

	fprintln(out, fmt.Sprintf("Created Unit %s/%s (revision %d). The original is untouched.",
		r.Space, r.NewUnit, r.Revision))
	fprintln(out, "")
	fprintln(out, "Remaining steps — each is deliberate, and none of them are automated:")
	fprintln(out, "")
	if r.SourceWedged {
		fprintln(out, fmt.Sprintf("  0. The original %s/%s carries a committed change (%s) that can never",
			r.Space, r.OldUnit, r.SourceRevisions))
		fprintln(out, "     reconcile — almost certainly the same edit, attempted in place first. The")
		fprintln(out, "     replacement does not clear it; revert it so the original stops being wedged:")
		fprintln(out, fmt.Sprintf("       cub unit update --space %s %s --restore <last-applied-revision>",
			r.Space, r.OldUnit))
	}
	fprintln(out, fmt.Sprintf("  1. Apply the replacement:   cub unit apply --space %s %s", r.Space, r.NewUnit))
	fprintln(out, "  2. Wait for it to report ACTIVE and its nodes to join the cluster.")
	fprintln(out, fmt.Sprintf("  3. Drain %s and let workloads reschedule onto %s.", r.OldName, r.NewName))
	fprintln(out, "     This is an in-cluster operation over live pod state (Eviction API,")
	fprintln(out, "     PodDisruptionBudgets, DaemonSets) and is out of scope for this tool.")
	fprintln(out, fmt.Sprintf("  4. Retire the original once it is empty:"))
	fprintln(out, fmt.Sprintf("       cub unit destroy --space %s %s   # removes it from AWS", r.Space, r.OldUnit))
	fprintln(out, fmt.Sprintf("       cub unit delete  --space %s %s   # removes the Unit", r.Space, r.OldUnit))
	fprintln(out, "     Destroy removes live compute — check for a Destroy Gate first, and")
	fprintln(out, "     confirm nothing is still scheduled there.")
}
