// Package cmd is the `cub argo` command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/cub-flux/internal/flux"
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
		Use:   "flux",
		Short: "Read a Flux fleet repository and plan it into ConfigHub, one variant per cluster",
		Long: `Read a Flux fleet repository as it is and plan it into ConfigHub.

  plan   reads the repository's clusters/<name> directories and the Flux
         Kustomizations in each (its layers), infers each layer's shared base
         from what every cluster's overlay builds on, and shows the fleet
         ConfigHub would govern: one base per layer, one variant per cluster
         listing exactly what its overlay changes, a Target per cluster, and
         the stage order. Offline: no account, no cluster, nothing changes.

Not yet: apply (write the steps as a script) and takeover.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var opts flux.Options
	var stages string
	var asJSON bool
	plan := &cobra.Command{
		Use:   "plan <fleet-repo-dir>",
		Short: "Show the fleet ConfigHub would govern; changes nothing",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			in, err := flux.Load(c.InOrStdin(), args)
			if err != nil {
				return err
			}
			opts.Stages = split(stages)
			p, err := flux.Build(in, opts)
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
				fmt.Fprint(out, flux.Render(p))
			}
			if len(p.Problems) > 0 {
				return errProblems{}
			}
			return nil
		},
	}
	plan.Flags().StringVar(&opts.Prefix, "prefix", "flux", "prefix for everything the plan would create in ConfigHub")
	plan.Flags().StringVar(&stages, "stages", "", "the clusters in rollout order, by directory or cluster name, comma-separated (default dev, staging, prod, then the rest)")
	plan.Flags().StringVar(&opts.ClustersDir, "clusters", "clusters", "the directory holding one directory per cluster, relative to the input")
	plan.Flags().StringVar(&opts.RepoRoot, "repo-root", "", "the checkout Flux paths are relative to (default: the .git above the input)")
	plan.Flags().BoolVar(&asJSON, "json", false, "print the plan as JSON")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the plugin version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "cub flux %s (%s, %s)\n", version, commit, date)
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
