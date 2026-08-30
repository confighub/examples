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
	// Space is where the Unit lives. A slug is unique only within a Space, so a
	// fleet-wide report has to carry it to name the Unit at all.
	Space   string `json:"space,omitempty"`
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
	if done, err := managerkit.Render(cmd.OutOrStdout(), output, rep); done || err != nil {
		return err
	}
	// A Space column only where the report spans Spaces: a single-Unit command
	// already names the Space in its header, and repeating it in every row of a
	// one-row table is noise.
	crossSpace := space == ""
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	if crossSpace {
		fmt.Fprintln(tw, "SPACE\tUNIT\tMUTATED\tERROR")
	} else {
		fmt.Fprintln(tw, "UNIT\tMUTATED\tERROR")
	}
	for _, o := range rep.Outcomes {
		if crossSpace {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				cliutil.Dash(o.Space), o.Unit, cliutil.YesNo(o.Mutated), cliutil.Dash(o.Error))
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", o.Unit, cliutil.YesNo(o.Mutated), cliutil.Dash(o.Error))
		}
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
//
// Aggregation is by UnitID, not by slug. A slug is unique only within a Space,
// and a bulk edit spans Spaces: keying by slug silently folds every `app` in the
// fleet into one row, so the count an operator reads off a dry run is the number
// of distinct names rather than the number of Units about to change.
func Summarize(command, space string, dryRun bool, results ...*cubapi.Result) Report {
	byUnit := map[string]*Outcome{}
	var order []string
	for _, res := range results {
		if res == nil {
			continue
		}
		for _, o := range res.Outcomes {
			out, ok := byUnit[o.UnitID]
			if !ok {
				out = &Outcome{Space: o.SpaceSlug, Unit: o.UnitSlug, succeeded: true}
				byUnit[o.UnitID] = out
				order = append(order, o.UnitID)
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
	// Sorted by where the Unit is, not by the id rows were keyed on.
	sort.Slice(order, func(i, j int) bool {
		a, b := byUnit[order[i]], byUnit[order[j]]
		if a.Space != b.Space {
			return a.Space < b.Space
		}
		if a.Unit != b.Unit {
			return a.Unit < b.Unit
		}
		return order[i] < order[j]
	})

	rep := Report{Command: command, Space: space, DryRun: dryRun}
	for _, id := range order {
		out := byUnit[id]
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
