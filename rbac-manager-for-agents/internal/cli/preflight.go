// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/cub"
)

// newPreflightCmd checks that cub-rbac can talk to ConfigHub: the cub CLI is on
// PATH and the session is valid against the server. It is the same gate every
// ConfigHub-touching command runs, exposed as a standalone command so an agent
// can verify readiness before attempting real work.
func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify cub is installed and the ConfigHub session is valid",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			cmd.Println("cub-rbac: ready (cub found, ConfigHub session valid)")
			return nil
		},
	}
}
