// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/observability-manager/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the cub-observability version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "cub-observability %s\n", version.Version)
		},
	}
}
