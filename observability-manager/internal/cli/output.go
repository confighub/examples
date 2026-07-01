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

const (
	outputJSON  = "json"
	outputTable = "table"
)

func addOutputFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVarP(dest, "output", "o", outputJSON, "output format: json | table")
}

// filterFlags binds the fleet-scoping flags and compiles them into one ConfigHub
// Unit `--where` predicate (Unit / Space.* / Target.* metadata).
type filterFlags struct {
	where       string
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
	cmd.Flags().StringVar(&f.component, "component", "", "select Units whose Space has Labels.Component = <value>")
	cmd.Flags().StringVar(&f.environment, "environment", "", "select Units whose Space has Labels.Environment = <value>")
	cmd.Flags().StringVar(&f.region, "region", "", "select Units whose Space has Labels.Region = <value>")
	cmd.Flags().StringVar(&f.owner, "owner", "", "select Units whose Space has Labels.Owner = <value>")
	cmd.Flags().StringVar(&f.layer, "layer", "", "select Units whose Space has Labels.Layer = <value>")
	cmd.Flags().StringVar(&f.variant, "variant", "", "select Units whose Space has Labels.Variant = <value>")
}

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
	eq("Space.Labels.Component", f.component)
	eq("Space.Labels.Environment", f.environment)
	eq("Space.Labels.Region", f.region)
	eq("Space.Labels.Owner", f.owner)
	eq("Space.Labels.Layer", f.layer)
	eq("Space.Labels.Variant", f.variant)
	return strings.Join(terms, " AND ")
}

func printJSON(w io.Writer, v any) error { return cliutil.PrintJSON(w, v) }

func fprintln(w io.Writer, a ...any) { _, _ = fmt.Fprintln(w, a...) }

func nsOrDash(ns string) string {
	if ns == "" {
		return "-"
	}
	return ns
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
