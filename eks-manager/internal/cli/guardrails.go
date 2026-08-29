// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The EKS guardrail pack. The mechanism -- policy Space, Triggers, shared
// Filter, deciding which Spaces to wire and which to leave alone, and reporting
// the Units a Trigger marked -- is managerkit/guardrails'. What is EKS's is the
// rules, and which Spaces are candidates at all: only those holding an EKS
// control plane, since the rest have nothing for these rules to say.

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/eks-manager/internal/cub"
	"github.com/confighub/examples/eks-manager/internal/snapshot"
	"github.com/confighub/examples/managerkit/guardrails"
)

// Each expression short-circuits on kind, so a rule is a no-op for resources it
// does not govern rather than a failure.
var pack = guardrails.Pack{
	Label: "eks-guardrails",
	Rules: []guardrails.Rule{
		{
			Slug:        "eks-automode-invariant",
			Description: "EKS Auto Mode requires computeConfig, elasticLoadBalancing and blockStorage to all be set and agree; a partial or disagreeing set is rejected by AWS.",
			Expression: `!(r.kind == 'Cluster' && has(r.spec.forProvider.computeConfig)) || ` +
				`(has(r.spec.forProvider.storageConfig) && has(r.spec.forProvider.kubernetesNetworkConfig))`,
		},
		{
			Slug:        "eks-pinned-version-extended-support",
			Description: "A pinned control-plane version must be paired with upgradePolicy.supportType EXTENDED, or AWS auto-upgrades at end of standard support and fights the pin.",
			Expression: `!(r.kind == 'Cluster' && has(r.spec.forProvider.version)) || ` +
				`(has(r.spec.forProvider.upgradePolicy) && r.spec.forProvider.upgradePolicy.supportType == 'EXTENDED')`,
		},
		{
			Slug:        "eks-private-endpoint",
			Description: "The Kubernetes API endpoint must not be publicly reachable from 0.0.0.0/0.",
			Expression: `!(r.kind == 'Cluster' && has(r.spec.forProvider.vpcConfig) && ` +
				`has(r.spec.forProvider.vpcConfig.endpointPublicAccess) && r.spec.forProvider.vpcConfig.endpointPublicAccess) || ` +
				`(has(r.spec.forProvider.vpcConfig.publicAccessCidrs) && ` +
				`!('0.0.0.0/0' in r.spec.forProvider.vpcConfig.publicAccessCidrs))`,
		},
		{
			Slug:        "eks-control-plane-logging",
			Description: "Control-plane audit logging must be enabled: enabledClusterLogTypes should include api, audit and authenticator.",
			Expression: `!(r.kind == 'Cluster') || (has(r.spec.forProvider.enabledClusterLogTypes) && ` +
				`'audit' in r.spec.forProvider.enabledClusterLogTypes)`,
		},
		{
			Slug:        "eks-secrets-encryption",
			Description: "Kubernetes Secrets must be encrypted with a KMS key (encryptionConfig). Note removing encryptionConfig later replaces the cluster.",
			Expression:  `!(r.kind == 'Cluster') || has(r.spec.forProvider.encryptionConfig)`,
		},
		{
			Slug:        "eks-no-latinit",
			Description: "managementPolicies must not include LateInitialize on a node group: it copies observed values into spec and defeats external autoscaling.",
			Expression: `!(r.kind == 'NodeGroup' && has(r.spec.managementPolicies)) || ` +
				`!('LateInitialize' in r.spec.managementPolicies)`,
		},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

// newGuardrailsInstallCmd is the shared install, over the Spaces that actually
// hold an EKS control plane. It scopes with the fleet filter flags rather than
// --where-space, because which Spaces are EKS Spaces is a property of their
// config, not of their metadata.
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
what the validating function can evaluate. The one check that needs two
revisions — "does this change replace the resource?" — is not expressible here,
because a validator sees only the new data. That is what 'plan' computes.

Spaces that already select Triggers another way are reported, not modified.
Dry-run by default; pass --commit to write.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := preflight(cmd.Context())
			if err != nil {
				return err
			}
			where, err := filter.Predicate()
			if err != nil {
				return err
			}
			snap, err := snapshot.Load(cmd.Context(), client, where)
			if err != nil {
				return err
			}
			plan, err := pack.PlanFor(cmd.Context(), client, policySpace, clusterSpaces(snap), "")
			if err != nil {
				return err
			}
			if commit {
				out := cmd.OutOrStdout()
				if err := pack.Execute(cmd.Context(), client, policySpace, plan,
					func(line string) { fprintln(out, line) }); err != nil {
					return err
				}
				plan.Committed = true
			}
			if output == outputTable {
				guardrails.PrintPlan(cmd, plan)
				return nil
			}
			return printJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&policySpace, "policy-space", guardrails.DefaultPolicySpace,
		"Space holding the guardrail Triggers and the shared Filter; every tool defaults to the same one")
	cmd.Flags().BoolVar(&commit, "commit", false, "apply the plan (default is dry-run)")
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	return cmd
}

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install and inspect the EKS validation pack",
	}
	cmd.AddCommand(newGuardrailsInstallCmd(), pack.StatusCmd(preflight))
	return cmd
}
