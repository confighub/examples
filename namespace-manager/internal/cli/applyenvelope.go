// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/namespace-manager/internal/cub"
)

func newApplyEnvelopeCmd() *cobra.Command {
	var output, spaceSlug string
	var commit cliutil.CommitFlags
	cmd := &cobra.Command{
		Use:   "apply-envelope --space <space>",
		Short: "Stamp pod-security defaults on the Namespace Unit(s) in a Space (fixes missing-pod-security)",
		Long: `apply-envelope runs the mutating set-pod-security-defaults function over the
v1/Namespace Unit(s) in a Space, adding the Pod Security Admission labels
(enforce=baseline, warn=restricted). This is the in-place fix for the
'missing-pod-security' finding — a hermetic, idempotent, comment-preserving edit
that produces a clean revision (a no-op where the labels are already present).

The Unit is edited, not applied — rolling it out to a cluster is a separate,
deliberate 'cub unit apply'. Dry-run unless --commit --change-desc; never bypasses
ApplyGates.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if spaceSlug == "" {
				return fmt.Errorf("--space is required")
			}
			changeDesc, dryRun, err := commit.Validate(fmt.Sprintf("apply pod-security defaults to Namespace Units in %s", spaceSlug))
			if err != nil {
				return err
			}
			client, err := cub.Preflight(cmd.Context())
			if err != nil {
				return err
			}
			sp, err := cubapi.ResolveSpace(cmd.Context(), client, spaceSlug)
			if err != nil {
				return fmt.Errorf("resolve space %q: %w", spaceSlug, err)
			}
			sel := cubapi.Selector{
				Where:     fmt.Sprintf("SpaceID = '%s'", sp.SpaceID.String()),
				WhereData: "kind = 'Namespace'",
			}
			ch := cubapi.Change{}
			if !dryRun {
				ch = cubapi.Change{Description: changeDesc}
			}
			res, err := cub.InvokeMutation(cmd.Context(), client, "set-pod-security-defaults", nil, sel, ch)
			if err != nil {
				return err
			}
			return reportMutation(cmd, res, "apply-envelope", spaceSlug, dryRun, output)
		},
	}
	addOutputFlag(cmd, &output)
	commit.Bind(cmd)
	cmd.Flags().StringVar(&spaceSlug, "space", "", "Space whose Namespace Units get pod-security defaults (required)")
	return cmd
}
