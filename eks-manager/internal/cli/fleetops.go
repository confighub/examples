// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

// eksKindsWhereData scopes a bulk edit to EKS resources, so a broad selector
// cannot reach an unrelated Unit that happens to match the Space labels.
const eksKindsWhereData = "apiVersion LIKE 'eks.aws.upbound.io/%'"

type fleetOutcome struct {
	Space   string `json:"space"`
	Unit    string `json:"unit"`
	Mutated bool   `json:"mutated"`
	Error   string `json:"error,omitempty"`
}

type fleetReport struct {
	Command  string         `json:"command"`
	Filter   string         `json:"filter,omitempty"`
	DryRun   bool           `json:"dryRun"`
	Outcomes []fleetOutcome `json:"outcomes"`
	Mutated  int            `json:"mutated"`
	Total    int            `json:"total"`
	Refused  []string       `json:"refused,omitempty"`
	Refusals int            `json:"refusals,omitempty"`
}

func newFleetEditCmd() *cobra.Command {
	var (
		output string
		commit cliutil.CommitFlags
		filter filterFlags
		path   string
		value  string
		setter string
		kind   string
	)

	cmd := &cobra.Command{
		Use:   "fleet-edit",
		Short: "Apply one field change across every EKS resource matching a selector",
		Long: `fleet-edit sets a single field on every matching EKS resource across the fleet
in one operation — the bulk analog of the single-Unit write commands.

The change is graded before anything is written, using the same classifier
'plan' uses. If the path is one that cannot be reconciled in place, the command
refuses outright rather than committing a fleet-wide set of revisions that would
each fail silently. A bulk immutable change is not a bulk edit; it is a bulk
replacement, and that is 'replace-nodegroup', one node group at a time.

Selection is the standard fleet scoping (--where plus the label shorthands),
narrowed to EKS resources and further to --kind.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" || !cmd.Flags().Changed("value") {
				return fmt.Errorf("--path and --value are both required")
			}
			switch setter {
			case "set-string-path", "set-int-path", "set-bool-path":
			default:
				return fmt.Errorf("--setter %q: want set-string-path | set-int-path | set-bool-path", setter)
			}
			changeDesc, dryRun, err := commit.Validate(
				fmt.Sprintf("fleet-edit %s = %s on %s", path, value, kind))
			if err != nil {
				return err
			}

			// Grade before touching anything. A fleet-wide immutable change is
			// the worst version of the silent-wedge failure.
			resourceType := "eks.aws.upbound.io/v1beta2/" + kind
			c := eks.ClassifyPath(resourceType, path)
			if c.Disruption.Blocks() {
				return fmt.Errorf(
					"refused: %s on %s is %s — %s\n"+
						"This cannot be reconciled in place; Crossplane would refuse every one of these "+
						"updates and retry forever.\nUse `%s replace-nodegroup` per node group instead",
					strings.TrimPrefix(path, "spec.forProvider."), kind, c.Disruption, c.Reason,
					InvocationName())
			}

			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			where := filter.predicate()
			sel := cubapi.Selector{
				Where:         joinWhere("ToolchainType = 'Kubernetes/YAML'", where),
				WhereData:     eksKindsWhereData,
				WhereResource: fmt.Sprintf("ConfigHub.ResourceType LIKE 'eks.aws.upbound.io/%%/%s'", kind),
			}
			ch := cubapi.Change{}
			if !dryRun {
				ch = cubapi.Change{Description: changeDesc}
			}
			res, err := cub.SetPath(cmd.Context(), client, setter, resourceType, path, value, sel, ch)
			if err != nil {
				return err
			}

			r := fleetReport{Command: "fleet-edit", Filter: where, DryRun: dryRun}
			for _, o := range res.Outcomes {
				fo := fleetOutcome{Space: o.SpaceSlug, Unit: o.UnitSlug, Mutated: o.HasMutations}
				if o.Error != "" {
					fo.Error = o.Error
				}
				r.Outcomes = append(r.Outcomes, fo)
				r.Total++
				if o.HasMutations {
					r.Mutated++
				}
			}
			sort.Slice(r.Outcomes, func(i, j int) bool {
				if r.Outcomes[i].Space != r.Outcomes[j].Space {
					return r.Outcomes[i].Space < r.Outcomes[j].Space
				}
				return r.Outcomes[i].Unit < r.Outcomes[j].Unit
			})

			if output == outputTable {
				printFleetReport(cmd, r, fmt.Sprintf("%s = %s", shortPath(path), value))
				return nil
			}
			return printJSON(cmd.OutOrStdout(), r)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	f := cmd.Flags()
	f.StringVar(&path, "path", "", "config path to set, e.g. spec.forProvider.upgradePolicy.supportType")
	f.StringVar(&value, "value", "", "value to set")
	f.StringVar(&setter, "setter", "set-string-path", "set-string-path | set-int-path | set-bool-path")
	f.StringVar(&kind, "kind", "Cluster", "EKS kind to edit: Cluster | NodeGroup | Addon")
	return cmd
}

func newPromoteCmd() *cobra.Command {
	var (
		output string
		commit cliutil.CommitFlags
		filter filterFlags
	)
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Carry upstream cluster config downstream, preserving local overrides",
		Long: `promote upgrades downstream Units from their upstream, preserving whatever each
downstream Space has deliberately customized.

This is what makes a fleet-wide control-plane upgrade a *promotion* rather than
N independent edits: raise the version in the upstream Space, verify it there,
then carry it to staging and every prod region with one command. Region-specific
divergence — different subnet CIDRs, a larger node group, an extra addon —
survives, because the upgrade is a merge rather than an overwrite.

Only Units with an upstream are considered; everything else is skipped.

Dry-run by default; pass --commit --change-desc "..." to write.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("promote EKS config from upstream")
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			where := joinWhere(
				"ToolchainType = 'Kubernetes/YAML' AND UpstreamRevisionNum > 0",
				filter.predicate())
			ch := cubapi.Change{}
			if !dryRun {
				ch = cubapi.Change{Description: changeDesc}
			}
			res, err := cubapi.UpgradeUnits(cmd.Context(), client, where, ch)
			if err != nil {
				return err
			}

			r := fleetReport{Command: "promote", Filter: filter.predicate(), DryRun: dryRun}
			for _, o := range res.Outcomes {
				fo := fleetOutcome{Space: o.SpaceSlug, Unit: o.UnitSlug, Mutated: o.HasMutations}
				if o.Error != "" {
					fo.Error = o.Error
				}
				r.Outcomes = append(r.Outcomes, fo)
				r.Total++
				if o.HasMutations {
					r.Mutated++
				}
			}
			sort.Slice(r.Outcomes, func(i, j int) bool {
				if r.Outcomes[i].Space != r.Outcomes[j].Space {
					return r.Outcomes[i].Space < r.Outcomes[j].Space
				}
				return r.Outcomes[i].Unit < r.Outcomes[j].Unit
			})

			if output == outputTable {
				printFleetReport(cmd, r, "upgrade from upstream")
				return nil
			}
			return printJSON(cmd.OutOrStdout(), r)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	return cmd
}

func printFleetReport(cmd *cobra.Command, r fleetReport, what string) {
	out := cmd.OutOrStdout()
	if r.Total == 0 {
		fprintln(out, "No matching Units.")
		if r.Filter != "" {
			fprintln(out, "  filter: "+r.Filter)
		}
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SPACE\tUNIT\tCHANGED\tERROR")
	for _, o := range r.Outcomes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", o.Space, o.Unit, yesNo(o.Mutated), dash(o.Error))
	}
	_ = tw.Flush()

	suffix := ""
	if r.DryRun {
		suffix = " (dry-run — pass --commit --change-desc to write)"
	}
	fprintln(out, fmt.Sprintf("\n%s: %d of %d Unit(s) would change%s",
		what, r.Mutated, r.Total, suffix))
	if !r.DryRun {
		fprintln(out, "Not applied — rolling out is a separate `cub unit apply`.")
	}
}

// joinWhere ANDs two predicates. ConfigHub `where` is flat AND-only, so this is
// simple concatenation rather than expression building.
func joinWhere(base, extra string) string {
	if extra == "" {
		return base
	}
	return base + " AND " + extra
}

// clusterSpaces returns the Spaces holding EKS clusters in scope, for guardrail
// wiring.
func clusterSpaces(snap *snapshot.Snapshot) []string {
	seen := map[string]bool{}
	var out []string
	for _, cs := range snap.Clusters {
		if cs.Control == nil {
			continue
		}
		if s := cs.Control.Origin.Space; s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
