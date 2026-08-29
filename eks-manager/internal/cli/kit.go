// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The seam between this tool's commands and the shared CLI surface in the SDK's
// cliutil. Keeping the tool-local spellings here means the commands read the
// same whether a helper is shared yet or not.

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	api "github.com/confighub/sdk/core/function/api"
)

type filterFlags = cliutil.QueryFlags

// The two formats these commands render. cliutil offers more -- yaml, jq, yq,
// custom columns -- and adopting them is worth doing, but a flag that advertises
// a format the command then cannot render is worse than one that does not offer
// it.
const (
	outputJSON  = "json"
	outputTable = "table"
)

func addOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", outputJSON, "output format: json | table")
}

// addFilterFlags binds --where and the standard Space-label scopes, plus
// Cluster, which is EKS-specific: it names the cluster a Space describes, which
// for this tool is not the Target the config is delivered to.
//
// --select is deliberately not among them: the fleet snapshot pins the fields it
// reads, so a caller who narrowed the selection would get them back zeroed.
func addFilterFlags(cmd *cobra.Command, f *filterFlags) {
	f.Label("cluster", "Cluster", "select Units whose Space has Labels.Cluster = <value>")
	f.BindWhere(cmd)
	f.BindSpaceLabels(cmd)
}

func printJSON(w io.Writer, v any) error     { return cliutil.PrintJSON(w, v) }
func fprintln(w io.Writer, a ...any)         { cliutil.Fprintln(w, a...) }
func parseScore(s string) (api.Score, error) { return api.ParseScore(s) }
func dash(s string) string                   { return cliutil.Dash(s) }
func yesNo(b bool) string                    { return cliutil.YesNo(b) }
