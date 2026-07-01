// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/scheduling-manager/internal/cub"
)

func newPromoteCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Upgrade downstream workload Units to their upstream head (dry-run unless --commit)",
		Long: `promote performs an override-preserving upgrade of Kubernetes/YAML Units that are
behind their upstream (UpstreamRevisionNum > 0) — the variant propagation path: a
placement change authored in a base Space flows to the environment/region Spaces
cloned from it, keeping each Space's local customizations.

Scope with --where or a label shorthand. Dry run unless --commit --change-desc;
never bypasses ApplyGates.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("promote workload placement Units from upstream")
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
			return reportMutation(cmd, "promote", "", dryRun, output, res)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	return cmd
}
