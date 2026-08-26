// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The seam between this tool's commands and the shared CLI surface in
// managerkit/clikit. Keeping the tool-local spellings here means the commands
// read the same whether a helper is shared yet or not.

import (
	"io"

	"github.com/spf13/cobra"

	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/managerkit/clikit"
)

type filterFlags = clikit.FilterFlags

const (
	outputJSON  = clikit.OutputJSON
	outputTable = clikit.OutputTable
)

// addFilterFlags binds the standard fleet scopes plus Cluster, which is
// EKS-specific: it names the cluster a Space describes, which for this tool is
// not the Target the config is delivered to.
func addFilterFlags(cmd *cobra.Command, f *filterFlags) {
	f.Label("cluster", "Cluster", "select Units whose Space has Labels.Cluster = <value>")
	f.Bind(cmd)
}

func addOutputFlag(cmd *cobra.Command, dest *string) { clikit.AddOutputFlag(cmd, dest) }
func printJSON(w io.Writer, v any) error             { return clikit.PrintJSON(w, v) }
func fprintln(w io.Writer, a ...any)                 { clikit.Fprintln(w, a...) }
func parseScore(s string) (api.Score, error)         { return clikit.ParseScore(s) }
func dash(s string) string                           { return clikit.Dash(s) }
func yesNo(b bool) string                            { return clikit.YesNo(b) }
