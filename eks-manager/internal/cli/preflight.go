// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"

	"github.com/confighub/examples/eks-manager/internal/cub"
)

// newPreflightCmd checks that cub-eks can talk to ConfigHub: the client can be
// built from the ambient session and that session is valid against the server.
// It is the same gate every ConfigHub-touching command runs, exposed as a
// standalone command so an agent can verify readiness before attempting real
// work.
func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify the ConfigHub session is valid",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			cmd.Println("cub-eks: ready (ConfigHub session valid)")
			return nil
		},
	}
}
