// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package managerkit

import "github.com/spf13/cobra"

// Output formats. JSON is the default so that the tools compose; a table is for
// a human reading one answer.
//
// These are the two formats the commands actually render. cliutil renders yaml,
// jq, yq and custom columns as well, and adopting them is worth doing, but a
// flag that advertises a format the command then cannot render is worse than one
// that does not offer it.
const (
	OutputJSON  = "json"
	OutputTable = "table"
)

// AddOutputFlag registers the -o/--output flag every command in these tools has.
func AddOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", OutputJSON, "output format: json | table")
}
