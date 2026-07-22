// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
)

const (
	guardrailPack       = "eks-guardrails"
	defaultPolicySpace  = "eks-policy"
	guardrailFilterSlug = "eks-guardrails"
)

// guardrailRule is one validating Trigger in the pack.
//
// All of these are per-resource assertions a single Unit can answer, so they are
// plain vet-cel. The one rule that genuinely needs two revisions — "does this
// change replace the resource?" — is not expressible as vet-cel, because a
// validator sees only the new data. That is what `plan` computes client-side and
// what the motivated vet-disruption function would enforce; until it exists, the
// closest server-side approximation is a vet-immutable Trigger over registered
// immutable paths.
type guardrailRule struct {
	slug string
	desc string
	expr string
}

// Each expression short-circuits on kind, so a rule is a no-op for resources it
// does not govern rather than a failure.
var guardrailRules = []guardrailRule{
	{
		slug: "eks-automode-invariant",
		desc: "EKS Auto Mode requires computeConfig, elasticLoadBalancing and blockStorage to all be set and agree; a partial or disagreeing set is rejected by AWS.",
		expr: `!(r.kind == 'Cluster' && has(r.spec.forProvider.computeConfig)) || ` +
			`(has(r.spec.forProvider.storageConfig) && has(r.spec.forProvider.kubernetesNetworkConfig))`,
	},
	{
		slug: "eks-pinned-version-extended-support",
		desc: "A pinned control-plane version must be paired with upgradePolicy.supportType EXTENDED, or AWS auto-upgrades at end of standard support and fights the pin.",
		expr: `!(r.kind == 'Cluster' && has(r.spec.forProvider.version)) || ` +
			`(has(r.spec.forProvider.upgradePolicy) && r.spec.forProvider.upgradePolicy.supportType == 'EXTENDED')`,
	},
	{
		slug: "eks-private-endpoint",
		desc: "The Kubernetes API endpoint must not be publicly reachable from 0.0.0.0/0.",
		expr: `!(r.kind == 'Cluster' && has(r.spec.forProvider.vpcConfig) && ` +
			`has(r.spec.forProvider.vpcConfig.endpointPublicAccess) && r.spec.forProvider.vpcConfig.endpointPublicAccess) || ` +
			`(has(r.spec.forProvider.vpcConfig.publicAccessCidrs) && ` +
			`!('0.0.0.0/0' in r.spec.forProvider.vpcConfig.publicAccessCidrs))`,
	},
	{
		slug: "eks-control-plane-logging",
		desc: "Control-plane audit logging must be enabled: enabledClusterLogTypes should include api, audit and authenticator.",
		expr: `!(r.kind == 'Cluster') || (has(r.spec.forProvider.enabledClusterLogTypes) && ` +
			`'audit' in r.spec.forProvider.enabledClusterLogTypes)`,
	},
	{
		slug: "eks-secrets-encryption",
		desc: "Kubernetes Secrets must be encrypted with a KMS key (encryptionConfig). Note removing encryptionConfig later replaces the cluster.",
		expr: `!(r.kind == 'Cluster') || has(r.spec.forProvider.encryptionConfig)`,
	},
	{
		slug: "eks-no-latinit",
		desc: "managementPolicies must not include LateInitialize on a node group: it copies observed values into spec and defeats external autoscaling.",
		expr: `!(r.kind == 'NodeGroup' && has(r.spec.managementPolicies)) || ` +
			`!('LateInitialize' in r.spec.managementPolicies)`,
	},
}

type guardrailsPlan struct {
	PolicySpace  string   `json:"policySpace"`
	Filter       string   `json:"filter"`
	Triggers     []string `json:"triggers"`
	Wire         []string `json:"wire,omitempty"`
	AlreadyWired []string `json:"alreadyWired,omitempty"`
	Skipped      []skip   `json:"skipped,omitempty"`
	Committed    bool     `json:"committed"`
}

type skip struct {
	Space  string `json:"space"`
	Reason string `json:"reason"`
}

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install and inspect the EKS validation pack",
	}
	cmd.AddCommand(newGuardrailsInstallCmd(), newGuardrailsStatusCmd())
	return cmd
}

func newGuardrailsInstallCmd() *cobra.Command {
	var output, policySpace string
	var commit bool
	var filter filterFlags

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the EKS validating Triggers and wire them to cluster Spaces",
		Long: `guardrails install creates a policy Space holding validating Triggers, a Filter
selecting them, and wires that Filter to every Space holding an EKS cluster in
scope.

The Triggers are created with Warn=true, so a failing rule attaches a
non-blocking ApplyWarning rather than an ApplyGate. Promote a rule to blocking
with:

  cub trigger update <slug> --space <policy-space> --unwarn

That gate-versus-warning choice lives on the Trigger, not in the rule, so the
same pack can advise in dev and block in prod.

Every rule is a per-resource assertion that a single Unit can answer, which is
what vet-cel can evaluate. The one check that needs two revisions — "does this
change replace the resource?" — is not expressible here, because a validator
sees only the new data. That is what 'plan' computes, and what a vet-disruption
function would enforce server-side.

Spaces that already have their own Trigger configuration are skipped rather than
clobbered. Dry-run by default; pass --commit to write.`,
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

			plan := guardrailsPlan{PolicySpace: policySpace, Filter: guardrailFilterSlug}
			for _, r := range guardrailRules {
				plan.Triggers = append(plan.Triggers, r.slug)
			}
			for _, space := range clusterSpaces(snap) {
				if space == policySpace {
					continue
				}
				plan.Wire = append(plan.Wire, space)
			}
			sort.Strings(plan.Wire)

			if !commit {
				if output == outputTable {
					printGuardrailsPlan(cmd, plan)
					return nil
				}
				return printJSON(cmd.OutOrStdout(), plan)
			}

			ps, err := cubapi.EnsureSpace(cmd.Context(), client, goclientnew.Space{
				Slug:   policySpace,
				Labels: map[string]string{"app": unitManagedByLabel, "role": "policy"},
			})
			if err != nil {
				return fmt.Errorf("create policy space %s: %w", policySpace, err)
			}
			for _, r := range guardrailRules {
				if _, err := cubapi.EnsureTrigger(cmd.Context(), client, goclientnew.Trigger{
					SpaceID:       ps.SpaceID,
					Slug:          r.slug,
					Description:   r.desc,
					Event:         "Mutation",
					ToolchainType: "Kubernetes/YAML",
					FunctionName:  "vet-cel",
					Arguments: cubapi.Arguments([]api.FunctionArgument{
						{ParameterName: "expression", Value: r.expr},
					}),
					// Advisory by default; --unwarn promotes a rule to blocking.
					Warn:   true,
					Labels: map[string]string{"Pack": guardrailPack},
				}); err != nil {
					return fmt.Errorf("create trigger %s: %w", r.slug, err)
				}
			}
			fl, err := cubapi.EnsureFilter(cmd.Context(), client, goclientnew.Filter{
				SpaceID: ps.SpaceID,
				Slug:    guardrailFilterSlug,
				From:    "Trigger",
				Where:   fmt.Sprintf("Labels.Pack = '%s'", guardrailPack),
			})
			if err != nil {
				return fmt.Errorf("create filter: %w", err)
			}

			for _, space := range plan.Wire {
				sp, err := cubapi.ResolveSpace(cmd.Context(), client, space)
				if err != nil {
					plan.Skipped = append(plan.Skipped, skip{Space: space, Reason: err.Error()})
					continue
				}
				// Never clobber a Space that already has its own policy wiring.
				if sp.TriggerFilterID != nil && *sp.TriggerFilterID != fl.FilterID {
					plan.Skipped = append(plan.Skipped, skip{
						Space:  space,
						Reason: "already has a different Trigger Filter; left untouched",
					})
					continue
				}
				if sp.TriggerFilterID != nil && *sp.TriggerFilterID == fl.FilterID {
					plan.AlreadyWired = append(plan.AlreadyWired, space)
					continue
				}
				if err := cubapi.SetSpaceTriggerFilter(cmd.Context(), client, sp, fl.FilterID); err != nil {
					plan.Skipped = append(plan.Skipped, skip{Space: space, Reason: err.Error()})
				}
			}
			plan.Committed = true

			if output == outputTable {
				printGuardrailsPlan(cmd, plan)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), plan)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	cmd.Flags().StringVar(&policySpace, "policy-space", defaultPolicySpace, "Space to hold the Triggers and Filter")
	cmd.Flags().BoolVar(&commit, "commit", false, "apply the plan (default is dry-run)")
	return cmd
}

func printGuardrailsPlan(cmd *cobra.Command, p guardrailsPlan) {
	out := cmd.OutOrStdout()
	verb := "Would create"
	if p.Committed {
		verb = "Created"
	}
	fprintln(out, fmt.Sprintf("%s policy Space %s with %d validating Trigger(s) (Warn=true) and Filter %s:",
		verb, p.PolicySpace, len(p.Triggers), p.Filter))
	for _, t := range p.Triggers {
		fprintln(out, "  "+t)
	}
	if len(p.Wire) > 0 {
		fprintln(out, "")
		fprintln(out, fmt.Sprintf("%s wire to %d cluster Space(s):", verb, len(p.Wire)))
		for _, s := range p.Wire {
			fprintln(out, "  "+s)
		}
	}
	if len(p.AlreadyWired) > 0 {
		fprintln(out, "")
		fprintln(out, "Already wired: "+fmt.Sprint(p.AlreadyWired))
	}
	if len(p.Skipped) > 0 {
		fprintln(out, "")
		fprintln(out, "Skipped:")
		for _, s := range p.Skipped {
			fprintln(out, fmt.Sprintf("  %s — %s", s.Space, s.Reason))
		}
	}
	fprintln(out, "")
	if !p.Committed {
		fprintln(out, "Dry run — nothing written. Re-run with --commit to install.")
		return
	}
	fprintln(out, "Rules are advisory (ApplyWarning). Promote one to blocking with:")
	fprintln(out, fmt.Sprintf("  cub trigger update <slug> --space %s --unwarn", p.PolicySpace))
}

type gateRow struct {
	Space    string `json:"space"`
	Unit     string `json:"unit"`
	Gates    int    `json:"gates"`
	Warnings int    `json:"warnings"`
}

func newGuardrailsStatusCmd() *cobra.Command {
	var output string
	var filter filterFlags

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Units carrying ApplyGates or ApplyWarnings",
		Long: `guardrails status lists the Units in scope that currently carry an ApplyGate
(blocking) or an ApplyWarning (advisory), so you can see what the validation
pack is actually catching.

A gate is not something to bypass. Fix the data, or change the rule.`,
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
			var rows []gateRow
			for _, u := range snap.Units {
				if u.GateCount == 0 && u.WarningCount == 0 {
					continue
				}
				rows = append(rows, gateRow{
					Space: u.SpaceSlug, Unit: u.Slug,
					Gates: u.GateCount, Warnings: u.WarningCount,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Space != rows[j].Space {
					return rows[i].Space < rows[j].Space
				}
				return rows[i].Unit < rows[j].Unit
			})
			if output == outputTable {
				if len(rows) == 0 {
					fprintln(cmd.OutOrStdout(), "No Units carry ApplyGates or ApplyWarnings.")
					return nil
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
				fmt.Fprintln(tw, "SPACE\tUNIT\tGATES\tWARNINGS")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", r.Space, r.Unit, r.Gates, r.Warnings)
				}
				_ = tw.Flush()
				fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%d Unit(s)", len(rows)))
				return nil
			}
			return printJSON(cmd.OutOrStdout(), rows)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	return cmd
}
