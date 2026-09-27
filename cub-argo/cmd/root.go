// Package cmd is the `cub argo` command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/cub-argo/internal/argo"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Version is the plugin's version, set at release build time.
func Version() string { return version }

// errProblems makes the command exit non-zero after it has printed the plan.
type errProblems struct{}

func (errProblems) Error() string { return "the plan has problems to fix first" }

func split(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "argo",
		Short: "Read an Argo CD estate and plan it into ConfigHub, one variant per cluster",
		Long: `Read an Argo CD estate as it is and plan it into ConfigHub.

  plan   reads Applications, ApplicationSets, AppProjects and cluster Secrets,
         either a repository directory or 'kubectl get' output, and shows the
         fleet ConfigHub would govern: one base per ApplicationSet (or list
         element), one variant per Application it generates, addressed to that
         one cluster, and the control tree that stays as it is. Offline: no
         account, no cluster, nothing changes.

Not yet: apply (write the steps as a script) and handover.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var opts argo.Options
	var stages string
	var asJSON bool
	plan := &cobra.Command{
		Use:   "plan <dir|input.yaml|-> [more inputs]",
		Short: "Show the fleet ConfigHub would govern; changes nothing",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			in, err := argo.Load(c.InOrStdin(), args)
			if err != nil {
				return err
			}
			opts.Stages = split(stages)
			p, err := argo.Build(in, opts)
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(p); err != nil {
					return err
				}
			} else {
				fmt.Fprint(out, argo.Render(p))
			}
			if len(p.Problems) > 0 {
				return errProblems{}
			}
			return nil
		},
	}
	plan.Flags().StringVar(&opts.Prefix, "prefix", "argo", "prefix for everything the plan would create in ConfigHub")
	plan.Flags().StringVar(&opts.StageLabel, "stage-label", "", "roll out in stages by the value of this cluster label")
	plan.Flags().StringVar(&stages, "stages", "", "the stages in order, comma-separated (with --stage-label)")
	plan.Flags().StringVar(&opts.RepoRoot, "repo-root", "", "the checkout Applications' source paths are relative to (default: the .git above a directory input)")
	plan.Flags().BoolVar(&asJSON, "json", false, "print the plan as JSON")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the plugin version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "cub argo %s (%s, %s)\n", version, commit, date)
		},
	}

	root.AddCommand(plan, versionCmd)
	return root
}

// Execute runs the command tree and exits non-zero on failure.
func Execute() {
	if err := newRoot().Execute(); err != nil {
		if _, printed := err.(errProblems); !printed {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
