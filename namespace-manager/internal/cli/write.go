// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
)

// mutationOutcome is one Unit's result from a mutating function invocation.
type mutationOutcome struct {
	Unit    string `json:"unit"`
	Mutated bool   `json:"mutated"`
	Error   string `json:"error,omitempty"`
}

// mutationReport is the JSON/table result of a write command.
type mutationReport struct {
	Command   string            `json:"command"`
	Space     string            `json:"space,omitempty"`
	DryRun    bool              `json:"dryRun"`
	Outcomes  []mutationOutcome `json:"outcomes"`
	Mutated   int               `json:"mutated"`
	Committed int               `json:"committed"`
}

// reportMutation renders a cubapi function Result as JSON or a table. On dry-run
// the mutated count is what *would* change; on commit it is what did.
func reportMutation(cmd *cobra.Command, res *cubapi.Result, command, space string, dryRun bool, output string) error {
	rep := mutationReport{Command: command, Space: space, DryRun: dryRun}
	for _, o := range res.Outcomes {
		rep.Outcomes = append(rep.Outcomes, mutationOutcome{Unit: o.UnitSlug, Mutated: o.HasMutations, Error: o.Error})
		if o.HasMutations {
			rep.Mutated++
			if !dryRun && o.Success {
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
