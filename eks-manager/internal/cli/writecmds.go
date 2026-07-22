// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

// pathEdit is one field change a write command intends to make.
type pathEdit struct {
	Path       string         `json:"path"`
	Value      string         `json:"value"`
	Setter     string         `json:"-"`
	Disruption eks.Disruption `json:"disruption"`
	Reason     string         `json:"reason,omitempty"`
}

type editReport struct {
	Command       string         `json:"command"`
	Space         string         `json:"space"`
	Unit          string         `json:"unit"`
	ResourceType  string         `json:"resourceType"`
	Edits         []pathEdit     `json:"edits"`
	MaxDisruption eks.Disruption `json:"maxDisruption"`
	MaxScore      string         `json:"maxScore,omitempty"`
	Blocks        bool           `json:"blocks"`
	Remediation   string         `json:"remediation,omitempty"`
	DryRun        bool           `json:"dryRun"`
	Mutated       int            `json:"mutated"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// unitRef identifies a single Unit by "<space>/<unit>".
type unitRef struct {
	spaceID   goclientnew.UUID
	spaceSlug string
	unitSlug  string
}

func (u unitRef) selector() cubapi.Selector {
	return cubapi.Selector{Where: fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", u.spaceID.String(), u.unitSlug)}
}

func parseUnitRef(ctx context.Context, c *cubapi.Client, arg string) (unitRef, error) {
	space, unit, ok := strings.Cut(arg, "/")
	if !ok || space == "" || unit == "" {
		return unitRef{}, fmt.Errorf("target must be <space>/<unit>, got %q", arg)
	}
	sp, err := cubapi.ResolveSpace(ctx, c, space)
	if err != nil {
		return unitRef{}, fmt.Errorf("resolve space %s: %w", space, err)
	}
	return unitRef{spaceID: sp.SpaceID, spaceSlug: sp.Slug, unitSlug: unit}, nil
}

// gradeEdits classifies the intended edits and refuses anything that cannot be
// reconciled in place. This is the payoff of `plan`: a write command grades its
// own change before committing it, so a blocking edit is caught here rather than
// becoming a committed-but-inert revision.
func gradeEdits(resourceType, kind string, edits []pathEdit) editReport {
	r := editReport{ResourceType: resourceType}
	for i := range edits {
		c := eks.ClassifyPath(resourceType, edits[i].Path)
		edits[i].Disruption = c.Disruption
		edits[i].Reason = c.Reason
		r.MaxDisruption = eks.MaxDisruption(r.MaxDisruption, c.Disruption)
	}
	r.Edits = edits
	r.MaxScore = r.MaxDisruption.Score()
	r.Blocks = r.MaxDisruption.Blocks()
	if r.Blocks {
		r.Remediation = eks.Remediation(r.MaxDisruption, kind)
	}
	return r
}

func applyEdits(ctx context.Context, client *cubapi.Client, ref unitRef, r *editReport, changeDesc string, dryRun bool) error {
	ch := cubapi.Change{}
	if !dryRun {
		ch = cubapi.Change{Description: changeDesc}
	}
	for _, e := range r.Edits {
		res, err := cub.SetPath(ctx, client, e.Setter, r.ResourceType, e.Path, e.Value, ref.selector(), ch)
		if err != nil {
			return fmt.Errorf("set %s: %w", e.Path, err)
		}
		for _, o := range res.Outcomes {
			if o.HasMutations {
				r.Mutated++
			}
		}
	}
	return nil
}

func printEditReport(cmd *cobra.Command, r editReport) {
	out := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tVALUE\tDISRUPTION")
	for _, e := range r.Edits {
		level := string(e.Disruption)
		if level == "" {
			level = "in-place"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", shortPath(e.Path), e.Value, level)
	}
	_ = tw.Flush()

	for _, w := range r.Warnings {
		fprintln(out, "\n  warning: "+w)
	}

	if r.Blocks {
		fprintln(out, "")
		fprintln(out, "  REFUSED — "+strings.ToUpper(string(r.MaxDisruption)))
		for _, e := range r.Edits {
			if e.Disruption.Blocks() {
				fprintln(out, fmt.Sprintf("    %s: %s", shortPath(e.Path), e.Reason))
			}
		}
		fprintln(out, "    Crossplane will refuse this update and retry forever; the Unit would read")
		fprintln(out, "    as applied while AWS is never changed.")
		if r.Remediation != "" {
			fprintln(out, "    -> "+r.Remediation)
		}
		return
	}

	if r.DryRun {
		fprintln(out, fmt.Sprintf("\nDry run — %d resource(s) would change in %s/%s.%s",
			r.Mutated, r.Space, r.Unit, " Re-run with --commit --change-desc \"…\" to write."))
		return
	}
	fprintln(out, fmt.Sprintf("\nChanged %d resource(s) in %s/%s. Not applied — rolling out is a separate `cub unit apply`.",
		r.Mutated, r.Space, r.Unit))
}

// runEdit is the shared body of the single-Unit write commands.
func runEdit(cmd *cobra.Command, commit cliutil.CommitFlags, output, target, command, kind string,
	build func(*eks.NodeGroupEntity) ([]pathEdit, []string, string, error)) error {

	summary := fmt.Sprintf("%s %s", command, target)
	changeDesc, dryRun, err := commit.Validate(summary)
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

	// Load just this Unit's resources so the edit can be grounded in the current
	// state (current sizes, current version, the resource's API version).
	snap, err := snapshot.Load(cmd.Context(), client,
		fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", ref.spaceID.String(), ref.unitSlug))
	if err != nil {
		return err
	}
	var ng *eks.NodeGroupEntity
	for _, cs := range snap.Clusters {
		for _, n := range cs.NodeGroups {
			ng = n
		}
	}
	if ng == nil {
		return fmt.Errorf("%s/%s does not contain an EKS NodeGroup", ref.spaceSlug, ref.unitSlug)
	}

	edits, warnings, resourceType, err := build(ng)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		fprintln(cmd.OutOrStdout(), "No change needed.")
		return nil
	}

	r := gradeEdits(resourceType, kind, edits)
	r.Command, r.Space, r.Unit, r.DryRun, r.Warnings = command, ref.spaceSlug, ref.unitSlug, dryRun, warnings

	if r.Blocks {
		if output == outputTable {
			printEditReport(cmd, r)
			return fmt.Errorf("refused: this change cannot be reconciled in place")
		}
		_ = printJSON(cmd.OutOrStdout(), r)
		return fmt.Errorf("refused: this change cannot be reconciled in place")
	}

	if err := applyEdits(cmd.Context(), client, ref, &r, changeDesc, dryRun); err != nil {
		return err
	}
	if output == outputTable {
		printEditReport(cmd, r)
		return nil
	}
	return printJSON(cmd.OutOrStdout(), r)
}

func newScaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale",
		Short: "Change node group capacity",
	}
	cmd.AddCommand(newScaleNodeGroupCmd())
	return cmd
}

func newScaleNodeGroupCmd() *cobra.Command {
	var output string
	var commit cliutil.CommitFlags
	var min, max, desired int64

	cmd := &cobra.Command{
		Use:   "nodegroup <space>/<unit>",
		Short: "Set a node group's min / max / desired size",
		Long: `scale nodegroup edits a node group's scalingConfig. All three fields are
in-place updates — EKS changes the Auto Scaling group without replacing nodes.

Setting --nodes (desiredSize) is reported as a warning when the node group is
under full Crossplane management, because Crossplane will then reconcile that
value on every pass and fight Cluster Autoscaler or Karpenter. Setting only
--nodes-min / --nodes-max leaves the count to the autoscaler, which is usually
what you want.

Dry-run by default; pass --commit --change-desc "..." to write. The Unit is
edited, not applied.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("nodes-min") && !cmd.Flags().Changed("nodes-max") && !cmd.Flags().Changed("nodes") {
				return fmt.Errorf("specify at least one of --nodes-min, --nodes-max, --nodes")
			}
			return runEdit(cmd, commit, output, args[0], "scale nodegroup", "NodeGroup",
				func(ng *eks.NodeGroupEntity) ([]pathEdit, []string, string, error) {
					var edits []pathEdit
					var warnings []string

					newMin, newMax := ng.MinSize, ng.MaxSize
					if cmd.Flags().Changed("nodes-min") {
						newMin = &min
						edits = append(edits, pathEdit{
							Path: "spec.forProvider.scalingConfig.minSize", Value: fmt.Sprint(min), Setter: "set-int-path"})
					}
					if cmd.Flags().Changed("nodes-max") {
						newMax = &max
						edits = append(edits, pathEdit{
							Path: "spec.forProvider.scalingConfig.maxSize", Value: fmt.Sprint(max), Setter: "set-int-path"})
					}
					if cmd.Flags().Changed("nodes") {
						edits = append(edits, pathEdit{
							Path: "spec.forProvider.scalingConfig.desiredSize", Value: fmt.Sprint(desired), Setter: "set-int-path"})
						if !ng.DesiredSizeInInitProvider && managesUpdatesCLI(ng.ManagementPolicies) {
							warnings = append(warnings,
								"desiredSize under forProvider is reconciled on every pass and will fight Cluster Autoscaler "+
									"or Karpenter; prefer EKS Auto Mode, or set only --nodes-min/--nodes-max")
						}
						if newMin != nil && desired < *newMin {
							return nil, nil, "", fmt.Errorf("--nodes %d is below minSize %d", desired, *newMin)
						}
						if newMax != nil && desired > *newMax {
							return nil, nil, "", fmt.Errorf("--nodes %d is above maxSize %d", desired, *newMax)
						}
					}
					if newMin != nil && newMax != nil && *newMin > *newMax {
						return nil, nil, "", fmt.Errorf("minSize %d is above maxSize %d", *newMin, *newMax)
					}
					return edits, warnings, nodeGroupResourceType(ng), nil
				})
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().Int64Var(&min, "nodes-min", 0, "minimum size")
	cmd.Flags().Int64Var(&max, "nodes-max", 0, "maximum size")
	cmd.Flags().Int64Var(&desired, "nodes", 0, "desired size (see the autoscaler warning)")
	return cmd
}

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade a cluster's control plane or a node group",
	}
	cmd.AddCommand(newUpgradeNodeGroupCmd(), newUpgradeClusterCmd())
	return cmd
}

func newUpgradeNodeGroupCmd() *cobra.Command {
	var output string
	var commit cliutil.CommitFlags
	var to string

	cmd := &cobra.Command{
		Use:   "nodegroup <space>/<unit>",
		Short: "Upgrade a node group's Kubernetes version",
		Long: `upgrade nodegroup sets a node group's version. This is an in-place update as
far as Crossplane is concerned, but EKS drains and replaces every node in the
group, honoring updateConfig.maxUnavailable and any PodDisruptionBudgets — so it
is graded 'rolling', not 'in-place'.

The version must not exceed the control plane's, and EKS moves one minor version
at a time. Both are checked before anything is written; nothing downstream
enforces them, and an illegal transition becomes a permanently unsynced managed
resource rather than an error you would see.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(cmd, commit, output, args[0], "upgrade nodegroup", "NodeGroup",
				func(ng *eks.NodeGroupEntity) ([]pathEdit, []string, string, error) {
					target, ok := eks.ParseVersion(to)
					if !ok {
						return nil, nil, "", fmt.Errorf("--to %q is not a Kubernetes major.minor version (e.g. 1.34)", to)
					}
					if ng.Version == to {
						return nil, nil, "", nil
					}
					var warnings []string
					if cur, ok := eks.ParseVersion(ng.Version); ok {
						if legal, why := eks.UpgradeLegal(cur, target); !legal {
							return nil, nil, "", fmt.Errorf("%s", why)
						}
					} else {
						warnings = append(warnings,
							"the node group had no pinned version; it was tracking the control plane")
					}
					warnings = append(warnings,
						"EKS will drain and replace every node in this group; check updateConfig.maxUnavailable "+
							"and PodDisruptionBudgets first")
					return []pathEdit{{
						Path: "spec.forProvider.version", Value: target.String(), Setter: "set-string-path",
					}}, warnings, nodeGroupResourceType(ng), nil
				})
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&to, "to", "", "target Kubernetes version (required)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// nodeGroupResourceType reconstructs the full ResourceType from the parsed
// entity, so the classifier sees the same API version the Unit actually uses.
func nodeGroupResourceType(ng *eks.NodeGroupEntity) string {
	return ng.APIVersion + "/NodeGroup"
}

func managesUpdatesCLI(policies []string) bool {
	if len(policies) == 0 {
		return true
	}
	for _, p := range policies {
		if p == "Update" || p == "*" {
			return true
		}
	}
	return false
}
