// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/namespace-manager/internal/cub"
)

func newPromoteCmd() *cobra.Command {
	var output string
	var filter filterFlags
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Upgrade downstream envelope Units to their upstream head (dry-run unless --commit)",
		Long: `promote performs an override-preserving upgrade of Kubernetes/YAML Units that are
behind their upstream (UpstreamRevisionNum < the upstream's head) — the variant
propagation path: an envelope authored in a base Space flows to the component and
cluster Spaces cloned from it, keeping each Space's local customizations.

Use it to roll an envelope change (a tightened default-deny, a new required
label) out to every variant Space. Scope to the Units you mean with --where or a
label shorthand (e.g. --component apptique). Dry run unless --commit --change-desc.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			changeDesc, dryRun, err := commit.Validate("promote namespace-envelope Units from upstream")
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
			return reportMutation(cmd, res, "promote", "", dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	addFilterFlags(cmd, &filter)
	commit.Bind(cmd)
	return cmd
}
