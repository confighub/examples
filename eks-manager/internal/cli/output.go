// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
)

// outputFlag is the shared -o/--output value: "json" (default) or "table".
const (
	outputJSON  = "json"
	outputTable = "table"
)

func addOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", outputJSON, "output format: json | table")
}

// filterFlags binds the fleet-scoping flags and compiles them into one ConfigHub
// Unit `--where` predicate. A single Unit-level filter can reference Unit, Space,
// and Target metadata (Space.Labels.*, Target.Slug, Target.ProviderType, ...),
// so there is no need for separate Space/Target filters — the server does the
// scoping and only the matching Units' resources are fetched. The opinionated
// label flags are convenience shorthands over --where, mirroring the standard
// Space labels the `cub variant` commands use.
//
// Note --cluster is server-side here, unlike in the sibling managers where it is
// a client-side display filter over the Target slug. For cub-eks a cluster IS a
// Space (its Units describe that cluster rather than deploy to it), so cluster
// scoping is just another Space label.
type filterFlags struct {
	where       string
	cluster     string
	component   string
	environment string
	region      string
	owner       string
	layer       string
	variant     string
}

func addFilterFlags(cmd *cobra.Command, f *filterFlags) {
	cmd.Flags().StringVar(&f.where, "where", "",
		"raw ConfigHub Unit filter; may reference Slug, Labels.*, Space.*, Target.* (e.g. \"Target.ProviderType = 'OCI'\")")
	cmd.Flags().StringVar(&f.cluster, "cluster", "", "select Units whose Space has Labels.Cluster = <value>")
	cmd.Flags().StringVar(&f.component, "component", "", "select Units whose Space has Labels.Component = <value>")
	cmd.Flags().StringVar(&f.environment, "environment", "", "select Units whose Space has Labels.Environment = <value>")
	cmd.Flags().StringVar(&f.region, "region", "", "select Units whose Space has Labels.Region = <value>")
	cmd.Flags().StringVar(&f.owner, "owner", "", "select Units whose Space has Labels.Owner = <value>")
	cmd.Flags().StringVar(&f.layer, "layer", "", "select Units whose Space has Labels.Layer = <value>")
	cmd.Flags().StringVar(&f.variant, "variant", "", "select Units whose Space has Labels.Variant = <value>")
}

// predicate compiles the flags into a single ConfigHub `where` expression (empty
// when nothing is set, i.e. the whole fleet the user can view). ConfigHub
// `where` is flat AND-only — no parentheses, no OR — so the label shorthands are
// joined to any raw --where with a bare AND.
func (f filterFlags) predicate() string {
	var terms []string
	if f.where != "" {
		terms = append(terms, f.where)
	}
	eq := func(field, val string) {
		if val != "" {
			terms = append(terms, fmt.Sprintf("%s = '%s'", field, strings.ReplaceAll(val, "'", "''")))
		}
	}
	eq("Space.Labels.Cluster", f.cluster)
	eq("Space.Labels.Component", f.component)
	eq("Space.Labels.Environment", f.environment)
	eq("Space.Labels.Region", f.region)
	eq("Space.Labels.Owner", f.owner)
	eq("Space.Labels.Layer", f.layer)
	eq("Space.Labels.Variant", f.variant)
	return strings.Join(terms, " AND ")
}

// printJSON writes v as indented JSON, via cliutil so the example shares the
// SDK's output formatting.
func printJSON(w io.Writer, v any) error {
	return cliutil.PrintJSON(w, v)
}

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

// dash renders an empty string as "-" so table columns stay aligned and an
// absent value is visibly absent rather than blank.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// yesNo renders a bool for table output.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
