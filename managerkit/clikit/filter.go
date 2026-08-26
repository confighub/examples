// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package clikit

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// FilterFlags binds the fleet-scoping flags and compiles them into one ConfigHub
// Unit `--where` predicate.
//
// One Unit-level filter can reference Unit, Space and Target metadata
// (Space.Labels.*, Target.Slug, Target.ProviderType, ...), so there is no need
// for separate Space or Target filters: the server does the scoping, and only
// the matching Units' resources are read. The label flags are convenience
// shorthands over --where, mirroring the standard Space labels the
// `cub variant` commands use.
type FilterFlags struct {
	Where       string
	Component   string
	Environment string
	Region      string
	Owner       string
	Layer       string
	Variant     string

	extra []*labelScope
}

// labelScope is a tool-specific Space-label flag added with Label.
type labelScope struct {
	flag  string
	label string
	usage string
	value string
}

// Label adds a Space-label scope beyond the standard set, for a label a single
// tool cares about. The flag joins the predicate with the others, so a tool gets
// its own scoping without every tool growing the flag.
//
// Call it before Bind. The terms it adds follow the standard labels; a predicate
// is a flat conjunction, so their position carries no meaning.
func (f *FilterFlags) Label(flag, spaceLabel, usage string) {
	f.extra = append(f.extra, &labelScope{flag: flag, label: spaceLabel, usage: usage})
}

// Bind registers the scoping flags on cmd.
func (f *FilterFlags) Bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Where, "where", "",
		"raw ConfigHub Unit filter; may reference Slug, Labels.*, Space.*, Target.* (e.g. \"Target.ProviderType = 'OCI'\")")
	cmd.Flags().StringVar(&f.Component, "component", "", "select Units whose Space has Labels.Component = <value>")
	cmd.Flags().StringVar(&f.Environment, "environment", "", "select Units whose Space has Labels.Environment = <value>")
	cmd.Flags().StringVar(&f.Region, "region", "", "select Units whose Space has Labels.Region = <value>")
	cmd.Flags().StringVar(&f.Owner, "owner", "", "select Units whose Space has Labels.Owner = <value>")
	cmd.Flags().StringVar(&f.Layer, "layer", "", "select Units whose Space has Labels.Layer = <value>")
	cmd.Flags().StringVar(&f.Variant, "variant", "", "select Units whose Space has Labels.Variant = <value>")
	for _, e := range f.extra {
		cmd.Flags().StringVar(&e.value, e.flag, "", e.usage)
	}
}

// Predicate compiles the flags into a single ConfigHub `where` expression, empty
// when nothing is set (the whole fleet the caller can view). ConfigHub `where`
// is flat AND-only -- no parentheses, no OR -- so the label shorthands are
// joined to any raw --where with a bare AND.
func (f FilterFlags) Predicate() string {
	var terms []string
	if f.Where != "" {
		terms = append(terms, f.Where)
	}
	eq := func(field, val string) {
		if val != "" {
			terms = append(terms, fmt.Sprintf("%s = '%s'", field, strings.ReplaceAll(val, "'", "''")))
		}
	}
	eq("Space.Labels.Component", f.Component)
	eq("Space.Labels.Environment", f.Environment)
	eq("Space.Labels.Region", f.Region)
	eq("Space.Labels.Owner", f.Owner)
	eq("Space.Labels.Layer", f.Layer)
	eq("Space.Labels.Variant", f.Variant)
	for _, e := range f.extra {
		eq("Space.Labels."+e.label, e.value)
	}
	return strings.Join(terms, " AND ")
}
