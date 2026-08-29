// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package write is the surface a mutating command in these tools shares: naming
// the Unit to write to, saying whether this is a dry run, and reporting what a
// function invocation did.
//
// It is deliberately the reporting half only. Which function to run, and with
// what arguments, is what each tool is about; how the outcome is rendered is
// not, and four tools rendering it four ways would be four things to keep in
// step for no gain.
package write

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"

	"github.com/confighub/examples/managerkit"
)

// UnitRef is a resolved <space>/<unit> target for a single-Unit write command.
type UnitRef struct {
	SpaceID   goclientnew.UUID
	SpaceSlug string
	UnitSlug  string
}

// Selector scopes a mutation to exactly this Unit.
func (u UnitRef) Selector() cubapi.Selector {
	return cubapi.Selector{Where: (cubapi.Where{}).SpaceID(u.SpaceID).Slug(u.UnitSlug).String()}
}

// ParseUnitRef parses a "<space>/<unit>" argument and resolves the Space to its
// ID against the server.
//
// A Unit slug carrying a quote or a backslash is rejected here rather than at
// the selector, where the error would name a filter expression the operator
// never wrote.
func ParseUnitRef(ctx context.Context, c *cubapi.Client, arg string) (UnitRef, error) {
	space, unit, ok := strings.Cut(arg, "/")
	if !ok || space == "" || unit == "" {
		return UnitRef{}, fmt.Errorf("target must be <space>/<unit>, got %q", arg)
	}
	sp, err := cubapi.ResolveSpace(ctx, c, space)
	if err != nil {
		return UnitRef{}, fmt.Errorf("resolve space %q: %w", space, err)
	}
	if err := (cubapi.Where{}).Slug(unit).Err(); err != nil {
		return UnitRef{}, err
	}
	return UnitRef{SpaceID: sp.SpaceID, SpaceSlug: space, UnitSlug: unit}, nil
}

// Change turns dry-run/description into a cubapi.Change. An empty description is
// what makes a call a dry run, so a dry run is spelled by not describing one.
func Change(changeDesc string, dryRun bool) cubapi.Change {
	if dryRun {
		return cubapi.Change{}
	}
	return cubapi.Change{Description: changeDesc}
}

// Outcome is one Unit's aggregated result from one or more mutating function
// invocations.
type Outcome struct {
	Unit    string `json:"unit"`
	Mutated bool   `json:"mutated"`
	Error   string `json:"error,omitempty"`

	// succeeded is every contributing invocation's Success, ANDed. It decides
	// whether a mutation counts as committed. It is not the same question as
	// whether Error is empty -- the server reports the two separately -- so it is
	// tracked rather than inferred.
	succeeded bool
}

// Report is the JSON/table result of a write command.
type Report struct {
	Command   string    `json:"command"`
	Space     string    `json:"space,omitempty"`
	DryRun    bool      `json:"dryRun"`
	Outcomes  []Outcome `json:"outcomes"`
	Mutated   int       `json:"mutated"`
	Committed int       `json:"committed"`
}

// ReportMutations renders one or more cubapi function Results as JSON or a
// table, aggregated per Unit -- so a command that runs several functions over
// the same Unit shows a single row, mutated if any function changed it. On a dry
// run the mutated count is what would change; on a commit it is what did.
func ReportMutations(cmd *cobra.Command, command, space string, dryRun bool, output string, results ...*cubapi.Result) error {
	rep := Summarize(command, space, dryRun, results...)
	if output != managerkit.OutputTable {
		return cliutil.PrintJSON(cmd.OutOrStdout(), rep)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "UNIT\tMUTATED\tERROR")
	for _, o := range rep.Outcomes {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", o.Unit, cliutil.YesNo(o.Mutated), cliutil.Dash(o.Error))
	}
	_ = tw.Flush()
	verb := "changed"
	if dryRun {
		verb = "would change"
	}
	cliutil.Fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%s: %d of %d Unit(s) %s%s",
		command, rep.Mutated, len(rep.Outcomes), verb, DryRunSuffix(dryRun)))
	return nil
}

// Summarize aggregates results per Unit without rendering them, for a command
// that folds the outcome into a report of its own.
func Summarize(command, space string, dryRun bool, results ...*cubapi.Result) Report {
	byUnit := map[string]*Outcome{}
	var order []string
	for _, res := range results {
		if res == nil {
			continue
		}
		for _, o := range res.Outcomes {
			out, ok := byUnit[o.UnitSlug]
			if !ok {
				out = &Outcome{Unit: o.UnitSlug, succeeded: true}
				byUnit[o.UnitSlug] = out
				order = append(order, o.UnitSlug)
			}
			if o.HasMutations {
				out.Mutated = true
			}
			if !o.Success {
				out.succeeded = false
			}
			if o.Error != "" && out.Error == "" {
				out.Error = o.Error
			}
		}
	}
	sort.Strings(order)

	rep := Report{Command: command, Space: space, DryRun: dryRun}
	for _, slug := range order {
		out := byUnit[slug]
		rep.Outcomes = append(rep.Outcomes, *out)
		if out.Mutated {
			rep.Mutated++
			if !dryRun && out.succeeded {
				rep.Committed++
			}
		}
	}
	return rep
}

// DryRunSuffix is the reminder a dry run's summary line ends with.
func DryRunSuffix(dryRun bool) string {
	if dryRun {
		return " (dry-run — pass --commit --change-desc to write)"
	}
	return ""
}
