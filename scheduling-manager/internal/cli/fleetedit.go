// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

const workloadKindsWhereData = "kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'CronJob', 'Pod')"

func newFleetEditCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var profileSlug string
	var params []string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "fleet-edit --profile <slug> [--where …]",
		Short: "Apply a placement profile across a selector of workloads (bulk, dry-run unless --commit)",
		Long: `fleet-edit applies a placement profile (a stored Invocation from the
scheduling-profiles Space) to every workload Unit matching a selector, in one
server-side operation. Scoped to pod-bearing workload kinds.

Scope with --where and the label shorthands (e.g. --environment prod). Supply
profile parameters with --param name=value. Dry-run unless --commit --change-desc.

Example: pin every prod ml workload to the gpu pool —
  fleet-edit --profile placement-gpu --environment prod --component ml --commit --change-desc "…"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if profileSlug == "" {
				return fmt.Errorf("--profile is required")
			}
			paramMap, err := parseParams(params)
			if err != nil {
				return err
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("fleet-edit: apply placement profile %s", profileSlug))
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
			sel := cubapi.Selector{Where: where, WhereData: workloadKindsWhereData}
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
	cmd.Flags().StringVar(&profileSlug, "profile", "", "placement profile (stored Invocation) to apply (required)")
	cmd.Flags().StringArrayVar(&params, "param", nil, "profile parameter as name=value (repeatable)")
	return cmd
}
