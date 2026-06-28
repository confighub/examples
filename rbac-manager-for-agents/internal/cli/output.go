// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/snapshot"
)

// outputFlag is the shared -o/--output value: "json" (default) or "table".
const (
	outputJSON  = "json"
	outputTable = "table"
)

func addOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", outputJSON, "output format: json | table")
}

// scopeFlags binds --target-where / --space-where and yields a snapshot.Scope.
type scopeFlags struct {
	targetWhere string
	spaceWhere  string
}

func addScopeFlags(cmd *cobra.Command, s *scopeFlags) {
	cmd.Flags().StringVar(&s.targetWhere, "target-where", "",
		"ConfigHub filter over Targets to scope deployed Units (e.g. \"Slug LIKE 'prod-%'\")")
	cmd.Flags().StringVar(&s.spaceWhere, "space-where", "",
		"ConfigHub filter over Spaces to scope untargeted base Units")
}

func (s scopeFlags) scope() snapshot.Scope {
	return snapshot.Scope{TargetWhere: s.targetWhere, SpaceWhere: s.spaceWhere}
}

// printJSON writes v as indented JSON, via cliutil so the example shares the
// SDK's output formatting.
func printJSON(w io.Writer, v any) error {
	return cliutil.PrintJSON(w, v)
}

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}
