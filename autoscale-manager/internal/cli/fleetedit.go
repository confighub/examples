// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/autoscale-manager/internal/cub"
)

// autoscalerKindsWhereData scopes fleet-edit to HPA / ScaledObject resources.
const autoscalerKindsWhereData = "kind IN ('HorizontalPodAutoscaler', 'ScaledObject')"

func newFleetEditCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var profileSlug string
	var params []string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "fleet-edit --profile <slug> [--where …]",
		Short: "Apply an autoscaling profile across a selector of HPA Units (bulk, dry-run unless --commit)",
		Long: `fleet-edit applies an autoscaling profile (a stored Invocation from the
autoscale-profiles Space) to every autoscaler Unit matching a selector, in one
server-side operation. Scoped to HorizontalPodAutoscaler / ScaledObject Units.

Scope with --where and the label shorthands (e.g. --environment prod). Supply
profile parameters with --param name=value. Dry-run unless --commit --change-desc.

Example: set every prod HPA to scale out early —
  fleet-edit --profile hpa-conservative --environment prod --commit --change-desc "…"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if profileSlug == "" {
				return fmt.Errorf("--profile is required")
			}
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("fleet-edit: apply autoscaling profile %s", profileSlug))
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cub.Preflight(ctx)
			if err != nil {
				return err
			}
			lib, err := cubapi.ResolveSpace(ctx, client, profilesSpace)
			if err != nil {
				return fmt.Errorf("resolve %s (run `%s profile install`): %w", profilesSpace, InvocationName(), err)
			}
			inv, err := cubapi.ResolveInvocation(ctx, client, lib.SpaceID, profileSlug)
			if err != nil {
				return fmt.Errorf("resolve profile %q: %w", profileSlug, err)
			}
			where := "ToolchainType = 'Kubernetes/YAML'"
			if p := filter.predicate(); p != "" {
				where += " AND " + p
			}
			sel := cubapi.Selector{Where: where, WhereData: autoscalerKindsWhereData}
			res, err := cubapi.InvokeStoredInvocation(ctx, client, inv.InvocationID, paramMap, sel, changeOf(changeDesc, dryRun))
			if err != nil {
				return err
			}
			return reportMutation(cmd, "fleet-edit "+profileSlug, "", dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&profileSlug, "profile", "", "autoscaling profile (stored Invocation) to apply (required)")
	cmd.Flags().StringArrayVar(&params, "param", nil, "profile parameter as name=value (repeatable)")
	return cmd
}
