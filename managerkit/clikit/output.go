// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package clikit holds the command-line surface these example tools share: how
// a fleet is scoped from flags, how output is rendered, and how a finding's
// severity is read.
//
// It is deliberately separate from the fleet package. Scoping a query is an API
// concern; spelling that scope as --component and --environment is a CLI one,
// and only the latter belongs here.
package clikit

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/managerkit"
)

// Output formats. JSON is the default so the tools compose; table is for humans.
const (
	OutputJSON  = "json"
	OutputTable = "table"
)

// AddOutputFlag registers the shared -o/--output flag.
func AddOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", OutputJSON, "output format: json | table")
}

// PrintJSON writes v as indented JSON, through the SDK so the tools and cub
// format their output the same way.
func PrintJSON(w io.Writer, v any) error { return cliutil.PrintJSON(w, v) }

// Fprintln writes a line, discarding the error the way a CLI's final output can.
func Fprintln(w io.Writer, a ...any) { _, _ = fmt.Fprintln(w, a...) }

// Dash renders an empty value as "-", so a table column reads as absent rather
// than blank -- a cluster-scoped resource's namespace, an unset owner.
func Dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// YesNo renders a bool for a table column.
func YesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ParseScore resolves a --severity value to ConfigHub's Score vocabulary, which
// findings are ranked in. Capitalization is not significant, so the lowercase
// spellings that predate the shared vocabulary still work. An empty value means
// no severity filter.
func ParseScore(s string) (api.Score, error) {
	if s == "" {
		return api.ScoreNone, nil
	}
	return api.ValidateScore(strings.ToUpper(s[:1]) + strings.ToLower(s[1:]))
}

// AddProfilesSpaceFlag registers --profiles-space, the Space holding a tool's
// stored profile Invocations. It defaults to the shared Space, so an operator
// has one place to look rather than one per tool.
func AddProfilesSpaceFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVar(dest, "profiles-space", managerkit.CommonSpace,
		"Space holding the profile library; every tool defaults to the same one")
}
