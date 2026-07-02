// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// cutUnitArg splits a "<space>/<unit>" argument, requiring both halves.
func cutUnitArg(arg string) (space, unit string, ok bool) {
	space, unit, ok = strings.Cut(arg, "/")
	if !ok || space == "" || unit == "" {
		return "", "", false
	}
	return space, unit, true
}

// unitRef is a resolved <space>/<unit> target for a single-Unit write command.
type unitRef struct {
	spaceID   goclientnew.UUID
	spaceSlug string
	unitSlug  string
}

func (u unitRef) selector() cubapi.Selector {
	return cubapi.Selector{Where: fmt.Sprintf("SpaceID = '%s' AND Slug = '%s'", u.spaceID.String(), u.unitSlug)}
}

func parseUnitRef(ctx context.Context, c *cubapi.Client, arg string) (unitRef, error) {
	space, unit, ok := cutUnitArg(arg)
	if !ok {
		return unitRef{}, fmt.Errorf("target must be <space>/<unit>, got %q", arg)
	}
	sp, err := cubapi.ResolveSpace(ctx, c, space)
	if err != nil {
		return unitRef{}, fmt.Errorf("resolve space %q: %w", space, err)
	}
	return unitRef{spaceID: sp.SpaceID, spaceSlug: space, unitSlug: unit}, nil
}

// changeOf turns dry-run/description into a cubapi.Change (empty = dry-run).
func changeOf(changeDesc string, dryRun bool) cubapi.Change {
	if dryRun {
		return cubapi.Change{}
	}
	return cubapi.Change{Description: changeDesc}
}

type mutationOutcome struct {
	Unit    string `json:"unit"`
	Mutated bool   `json:"mutated"`
	Error   string `json:"error,omitempty"`
}

type mutationReport struct {
	Command   string            `json:"command"`
	Space     string            `json:"space,omitempty"`
	DryRun    bool              `json:"dryRun"`
	Outcomes  []mutationOutcome `json:"outcomes"`
	Mutated   int               `json:"mutated"`
	Committed int               `json:"committed"`
}

// reportMutation renders one or more cubapi function Results, aggregated per Unit.
func reportMutation(cmd *cobra.Command, command, space string, dryRun bool, output string, results ...*cubapi.Result) error {
	byUnit := map[string]*mutationOutcome{}
	var order []string
	for _, res := range results {
		if res == nil {
			continue
		}
		for _, o := range res.Outcomes {
			mo, ok := byUnit[o.UnitSlug]
			if !ok {
				mo = &mutationOutcome{Unit: o.UnitSlug}
				byUnit[o.UnitSlug] = mo
				order = append(order, o.UnitSlug)
			}
			if o.HasMutations {
				mo.Mutated = true
			}
			if o.Error != "" && mo.Error == "" {
				mo.Error = o.Error
			}
		}
	}
	sort.Strings(order)

	rep := mutationReport{Command: command, Space: space, DryRun: dryRun}
	for _, slug := range order {
		mo := byUnit[slug]
		rep.Outcomes = append(rep.Outcomes, *mo)
		if mo.Mutated {
			rep.Mutated++
			if !dryRun && mo.Error == "" {
				rep.Committed++
			}
		}
	}

	if output == outputTable {
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "UNIT\tMUTATED\tERROR")
		for _, o := range rep.Outcomes {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", o.Unit, yesNo(o.Mutated), dash(o.Error))
		}
		_ = tw.Flush()
		verb := "would change"
		if !dryRun {
			verb = "changed"
		}
		fprintln(cmd.OutOrStdout(), fmt.Sprintf("\n%s: %d of %d Unit(s) %s%s",
			command, rep.Mutated, len(rep.Outcomes), verb, dryRunSuffix(dryRun)))
		return nil
	}
	return printJSON(cmd.OutOrStdout(), rep)
}

func dryRunSuffix(dryRun bool) string {
	if dryRun {
		return " (dry-run — pass --commit --change-desc to write)"
	}
	return ""
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
