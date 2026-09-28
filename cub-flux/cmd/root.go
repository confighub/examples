// Package cmd is the `cub argo` command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

  apply  writes the plan as files beside two scripts: apply.sh, which fills
         ConfigHub and touches no cluster, and handover.sh, which swaps each
         layer's sourceRef, one cluster at a time. Nothing runs until you run
         them.`,
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

	var af flux.Options
	var applyStages, out string
	apply := &cobra.Command{
		Use:   "apply <fleet-repo-dir> --out <dir>",
		Short: "Write the plan's files, apply.sh and handover.sh to read and run; runs nothing",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if out == "" {
				return fmt.Errorf("apply needs --out <dir> for the files and the scripts it writes")
			}
			in, err := flux.Load(c.InOrStdin(), args)
			if err != nil {
				return err
			}
			af.Stages = split(applyStages)
			p, err := flux.Build(in, af)
			if err != nil {
				return err
			}
			w := c.OutOrStdout()
			if len(p.Problems) > 0 {
				fmt.Fprint(w, flux.Render(p))
				return errProblems{}
			}
			script, err := flux.WriteApply(p, af.Prefix, out)
			if err != nil {
				return err
			}
			shown := script
			if cwd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(cwd, script); err == nil {
					shown = rel
				}
			}
			fmt.Fprint(w, flux.Render(p))
			fmt.Fprintf(w, "\nWrote %s and the files it reads. Read it, then run it:\n  bash %s\n", shown, shown)
			fmt.Fprintf(w, "\nThat fills ConfigHub and changes no cluster. Then move one cluster at a time:\n  CLUSTER=<cluster> FLUX_CONTEXT=<kubectl context> bash %s\n", filepath.Join(filepath.Dir(shown), "handover.sh"))
			return nil
		},
	}
	apply.Flags().StringVar(&af.Prefix, "prefix", "flux", "prefix for everything the plan would create in ConfigHub")
	apply.Flags().StringVar(&applyStages, "stages", "", "the clusters in rollout order, comma-separated")
	apply.Flags().StringVar(&af.ClustersDir, "clusters", "clusters", "the directory holding one directory per cluster")
	apply.Flags().StringVar(&af.RepoRoot, "repo-root", "", "the checkout Flux paths are relative to")
	apply.Flags().StringVar(&out, "out", "", "directory for the files and the scripts")

	var ckNS, ckName, ckSpace, ckUnit, ckTarget string
	var ckJSON bool
	check := &cobra.Command{
		Use:   "check --kustomization <name> --space <space> --unit <unit>",
		Short: "Compare what a layer applied with what the release holds; changes nothing",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if ckName == "" || ckSpace == "" || ckUnit == "" {
				return fmt.Errorf("check needs --kustomization, --space and --unit")
			}
			owned, err := flux.LiveInventory(flux.Run, ckNS, ckName)
			if err != nil {
				return err
			}
			data, err := flux.Run("cub", "unit", "data", "--space", ckSpace, ckUnit)
			if err != nil {
				return err
			}
			held, err := flux.ObjectsIn(data)
			if err != nil {
				return err
			}
			cmp := flux.CompareInventory(owned, held, ckTarget)
			w := c.OutOrStdout()
			if ckJSON {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				if err := enc.Encode(cmp); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(w, "%s: %d objects match what the layer applied\n", ckName, cmp.Same)
				for _, l := range append(cmp.WouldPrune, cmp.WouldAdd...) {
					fmt.Fprintf(w, "  %s\n", l)
				}
			}
			if !cmp.OK() {
				return errProblems{}
			}
			return nil
		},
	}
	check.Flags().StringVar(&ckNS, "namespace", "flux-system", "the namespace the Kustomizations live in")
	check.Flags().StringVar(&ckName, "kustomization", "", "the layer to read the inventory of")
	check.Flags().StringVar(&ckSpace, "space", "", "the ConfigHub Space holding the variant")
	check.Flags().StringVar(&ckUnit, "unit", "", "the unit in that Space")
	check.Flags().StringVar(&ckTarget, "target-namespace", "", "the layer's targetNamespace, where objects without one land")
	check.Flags().BoolVar(&ckJSON, "json", false, "print the comparison as JSON")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the plugin version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "cub flux %s (%s, %s)\n", version, commit, date)
		},
	}

	root.AddCommand(plan, apply, check, versionCmd)
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
