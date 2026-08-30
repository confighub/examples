// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// The seam between this tool's commands and the shared CLI surface in the SDK's
// cliutil. Keeping the tool-local spellings here means the commands read the
// same whether a helper is shared yet or not.

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/managerkit"
	"github.com/confighub/sdk/cliutil"
	api "github.com/confighub/sdk/core/function/api"
)

type filterFlags = cliutil.QueryFlags

func addOutputFlag(cmd *cobra.Command, dest *string) { managerkit.AddOutputFlag(cmd, dest) }

// addFilterFlags binds --where and the standard Space-label scopes. --select is
// deliberately not among them: the fleet snapshot pins the fields it reads, so a
// caller who narrowed the selection would get them back zeroed.
func addFilterFlags(cmd *cobra.Command, f *filterFlags) {
	f.BindWhere(cmd)
	f.BindSpaceLabels(cmd)
}

func fprintln(w io.Writer, a ...any)         { cliutil.Fprintln(w, a...) }
func parseScore(s string) (api.Score, error) { return api.ParseScore(s) }
func dash(s string) string                   { return cliutil.Dash(s) }
func yesNo(b bool) string                    { return cliutil.YesNo(b) }
