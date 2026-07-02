// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"

	"github.com/confighub/examples/autoscale-manager/internal/cub"
)

func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify the ConfigHub session is valid",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := cub.Preflight(cmd.Context()); err != nil {
				return err
			}
			cmd.Println("cub-autoscale: ready (ConfigHub session valid)")
			return nil
		},
	}
}
