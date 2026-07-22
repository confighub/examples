// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/eks"
)

// disruptionAttributePrefix must match vet-disruption's default, which looks for
// <prefix>-critical / -high / -medium / -low.
const disruptionAttributePrefix = "disruption"

type attributePlan struct {
	Space      string          `json:"space"`
	Prefix     string          `json:"prefix"`
	Attributes []attributeInfo `json:"attributes"`
	Committed  bool            `json:"committed"`
}

type attributeInfo struct {
	Slug       string   `json:"slug"`
	Disruption string   `json:"disruption"`
	Score      string   `json:"score"`
	Types      int      `json:"resourceTypes"`
	Paths      int      `json:"paths"`
	Examples   []string `json:"examples,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func newAttributesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attributes",
		Short: "Register the EKS disruption path tables as ConfigHub Attributes",
	}
	cmd.AddCommand(newAttributesInstallCmd())
	return cmd
}

func newAttributesInstallCmd() *cobra.Command {
	var output, space, prefix string
	var commit bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Register the disruption tables so vet-disruption can enforce them server-side",
		Long: `attributes install registers this tool's disruption table as ConfigHub Attributes —
one per severity tier — so the grading 'plan' computes client-side can also be
enforced server-side by the vet-disruption function.

The tiers map onto vet-disruption's score vocabulary:

  disruption-critical   replace-cluster       Critical
  disruption-high       replace               High
  disruption-medium     rolling               Medium
  disruption-low        in-place-disruptive   Low

These are the same rules 'plan' uses; there is one table, so the client-side
report and the server-side gate cannot drift apart.

Once registered, attach a Trigger:

  cub trigger create eks-disruption --space <space> \
    --function vet-disruption --argument score-threshold=High \
    --other-data-source LastAppliedRevisionNum --event Mutation --warn

The threshold decides which severities fail; the Trigger's Warn field decides
whether a failure warns or blocks. Set --other-data-source explicitly: without a
baseline the function cannot tell a wedging change from a clean slate, and it
passes.

Paths are registered in their canonical object-shaped form
(spec.forProvider.scalingConfig.desiredSize). A fleet still carrying deprecated
list-shaped v1beta1 resources needs the indexed variants registered too — 'plan'
normalizes indices away client-side, but a registered path is a literal lookup.

Dry-run by default; pass --commit to write.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := attributePlan{Space: space, Prefix: prefix}
			for _, tier := range eks.DisruptionTiers() {
				paths := eks.DisruptionPaths(tier)
				info := attributeInfo{
					Slug:       tier.AttributeName(prefix),
					Disruption: string(tier),
					Score:      tier.Score(),
					Types:      len(paths),
				}
				var examples []string
				for rt, ps := range paths {
					info.Paths += len(ps)
					for _, p := range ps {
						examples = append(examples, shortKind(rt)+": "+shortPath(p))
					}
				}
				sort.Strings(examples)
				if len(examples) > 3 {
					examples = examples[:3]
				}
				info.Examples = examples
				plan.Attributes = append(plan.Attributes, info)
			}

			if !commit {
				if output == outputTable {
					printAttributePlan(cmd, plan)
					return nil
				}
				return printJSON(cmd.OutOrStdout(), plan)
			}

			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			sp, err := cubapi.ResolveSpace(cmd.Context(), client, space)
			if err != nil {
				return fmt.Errorf("resolve space %s: %w", space, err)
			}
			for i, tier := range eks.DisruptionTiers() {
				attr := buildDisruptionAttribute(sp.SpaceID, tier, prefix)
				if _, err := cub.CreateAttribute(cmd.Context(), client, attr); err != nil {
					plan.Attributes[i].Error = err.Error()
				}
			}
			plan.Committed = true
			if output == outputTable {
				printAttributePlan(cmd, plan)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), plan)
		},
	}
	addOutputFlag(cmd, &output)
	cmd.Flags().StringVar(&space, "space", "eks-policy", "Space to create the Attributes in")
	cmd.Flags().StringVar(&prefix, "prefix", disruptionAttributePrefix,
		"attribute-name prefix; must match vet-disruption's attribute-prefix")
	cmd.Flags().BoolVar(&commit, "commit", false, "create the Attributes (default is dry-run)")
	return cmd
}

// buildDisruptionAttribute turns one tier of the disruption table into an Attribute. Every path is
// registered as neither needed nor provided: these paths exist to be *compared across revisions*
// by vet-disruption, not to participate in reference resolution.
func buildDisruptionAttribute(spaceID goclientnew.UUID, tier eks.Disruption, prefix string) goclientnew.Attribute {
	slug := tier.AttributeName(prefix)
	paths := eks.DisruptionPaths(tier)

	types := make([]string, 0, len(paths))
	for rt := range paths {
		types = append(types, rt)
	}
	sort.Strings(types)

	entries := make([]goclientnew.ResourceTypePathsEntry, 0, len(types))
	for _, groupKind := range types {
		// The table is keyed by group/Kind so one rule covers every API version. An Attribute
		// registers a concrete ResourceType, so expand to the versions this tool emits.
		for _, resourceType := range resourceTypesFor(groupKind) {
			pathMap := goclientnew.PathToVisitorInfoType{}
			for _, p := range paths[groupKind] {
				pathMap[p] = goclientnew.PathVisitorInfo{
					Path:          p,
					AttributeName: slug,
					// The server requires each path's DataType to match the Attribute's, and an
					// Attribute's is restricted to string/int/bool. These paths hold values of
					// every shape (lists of instance types, nested config blocks), but
					// vet-disruption reads them with DataTypeNone and compares them as opaque
					// values, so the declared type does not limit what it can grade.
					DataType: string(api.DataTypeString),
				}
			}
			entries = append(entries, goclientnew.ResourceTypePathsEntry{
				ResourceType: resourceType,
				Paths:        &pathMap,
			})
		}
	}
	return goclientnew.Attribute{
		SpaceID:       spaceID,
		Slug:          slug,
		DisplayName:   slug,
		ToolchainType: "Kubernetes/YAML",
		// The Attribute entity's own DataType is restricted to string/int/bool, unlike the
		// per-path DataType above — the paths themselves hold values of any shape (lists of
		// instance types, nested config blocks), which is why they register as YAML.
		DataType:          string(api.DataTypeString),
		ResourceTypePaths: entries,
	}
}

// resourceTypesFor expands a group/Kind key into the concrete API versions this tool targets.
// v1beta2 is the current storage version for the kinds that have one; the rest are still v1beta1.
func resourceTypesFor(groupKind string) []string {
	group, kind, ok := strings.Cut(groupKind, "/")
	if !ok {
		return nil
	}
	switch kind {
	case "Cluster", "NodeGroup", "Route", "LaunchTemplate":
		return []string{group + "/v1beta2/" + kind, group + "/v1beta1/" + kind}
	default:
		return []string{group + "/v1beta1/" + kind}
	}
}

func shortKind(groupKind string) string {
	if _, kind, ok := strings.Cut(groupKind, "/"); ok {
		return kind
	}
	return groupKind
}

func printAttributePlan(cmd *cobra.Command, p attributePlan) {
	out := cmd.OutOrStdout()
	verb := "Would register"
	if p.Committed {
		verb = "Registered"
	}
	fprintln(out, fmt.Sprintf("%s %d disruption Attribute(s) in Space %s:", verb, len(p.Attributes), p.Space))
	fprintln(out, "")

	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "  ATTRIBUTE\tDISRUPTION\tSCORE\tTYPES\tPATHS\tERROR")
	for _, a := range p.Attributes {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\t%d\t%s\n",
			a.Slug, a.Disruption, a.Score, a.Types, a.Paths, dash(a.Error))
	}
	_ = tw.Flush()

	fprintln(out, "")
	for _, a := range p.Attributes {
		if len(a.Examples) == 0 {
			continue
		}
		fprintln(out, fmt.Sprintf("  %s e.g. %s", a.Slug, strings.Join(a.Examples, "; ")))
	}

	fprintln(out, "")
	if !p.Committed {
		fprintln(out, "Dry run — nothing written. Re-run with --commit to register.")
		return
	}
	fprintln(out, "Attach a Trigger to enforce them (threshold decides which severities fail;")
	fprintln(out, "the Trigger's Warn field decides whether a failure warns or blocks):")
	fprintln(out, fmt.Sprintf("  cub trigger create eks-disruption --space %s \\", p.Space))
	fprintln(out, "    --function vet-disruption --argument score-threshold=High \\")
	fprintln(out, "    --other-data-source LastAppliedRevisionNum --event Mutation --warn")
}
