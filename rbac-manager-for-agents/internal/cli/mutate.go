// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
)

// commitFlags are shared by every mutating command. Mutations are dry-run by
// default; --commit performs the write and requires --change-desc for
// provenance.
type commitFlags struct {
	commit     bool
	changeDesc string
}

func addCommitFlags(cmd *cobra.Command, c *commitFlags) {
	cmd.Flags().BoolVar(&c.commit, "commit", false, "perform the change (default is dry-run: preview the diff only)")
	cmd.Flags().StringVar(&c.changeDesc, "change-desc", "", "change description recorded with the change (required with --commit)")
}

// runMutation previews (dry-run) or commits a cub mutation. baseArgs is the cub
// argv up to but excluding --dry-run / --change-desc; it must already include an
// output selector (e.g. -o mutations) so the preview shows a diff. summary is a
// human description used in the banner and as the suggested change description.
func runMutation(cmd *cobra.Command, baseArgs []string, c commitFlags, summary string) error {
	if !c.commit {
		fprintln(cmd.OutOrStdout(), "Dry run — "+summary)
		args := append(append([]string{}, baseArgs...), "--dry-run")
		if err := cub.RunStreaming(cmd.Context(), args...); err != nil {
			return err
		}
		fprintln(cmd.OutOrStdout(), fmt.Sprintf(
			"\nNo changes written. Re-run with --commit --change-desc \"...\" to apply (suggested: %q).", summary))
		return nil
	}
	if strings.TrimSpace(c.changeDesc) == "" {
		return fmt.Errorf("--change-desc is required with --commit (describe the change and why; suggested: %q)", summary)
	}
	args := append(append([]string{}, baseArgs...), "--change-desc", c.changeDesc)
	fprintln(cmd.OutOrStdout(), "Committing — "+summary)
	return cub.RunStreaming(cmd.Context(), args...)
}

func parseUnitRef(ref string) (space, unit string, err error) {
	space, unit, ok := strings.Cut(ref, "/")
	if !ok || space == "" || unit == "" {
		return "", "", fmt.Errorf("invalid unit reference %q: expected <space>/<unit>", ref)
	}
	return space, unit, nil
}
