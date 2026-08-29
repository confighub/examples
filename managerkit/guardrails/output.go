// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package guardrails

// The output surface these commands share. It is deliberately the two formats
// the commands actually render: JSON, the default so the tools compose, and a
// table for a human. cliutil offers more -- yaml, jq, yq, custom columns -- and
// adopting them is worth doing, but a flag that advertises a format the command
// then cannot render is worse than one that does not offer it.

import (
	"github.com/spf13/cobra"
)

const (
	outputJSON  = "json"
	outputTable = "table"
)

func addOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", outputJSON, "output format: json | table")
}
