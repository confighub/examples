// Package cmd is the `cub argo` command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

  apply  writes the plan as files beside two scripts: apply.sh, which fills
         ConfigHub and touches no cluster, and handover.sh, which repoints each
         layer's source at ConfigHub, top down. Nothing runs until you run them.`,
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

	var af argo.Options
	var applyStages, out string
	apply := &cobra.Command{
		Use:   "apply <dir|input.yaml|-> [more inputs] --out <dir>",
		Short: "Write the plan's files, apply.sh and handover.sh to read and run; runs nothing",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if out == "" {
				return fmt.Errorf("apply needs --out <dir> for the files and the scripts it writes")
			}
			in, err := argo.Load(c.InOrStdin(), args)
			if err != nil {
				return err
			}
			af.Stages = split(applyStages)
			p, err := argo.Build(in, af)
			if err != nil {
				return err
			}
			w := c.OutOrStdout()
			if len(p.Problems) > 0 {
				fmt.Fprint(w, argo.Render(p))
				return errProblems{}
			}
			script, err := argo.WriteApply(p, af.Prefix, out)
			if err != nil {
				return err
			}
			shown := script
			if cwd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(cwd, script); err == nil {
					shown = rel
				}
			}
			fmt.Fprint(w, argo.Render(p))
			fmt.Fprintf(w, "\nWrote %s and the files it reads. Read it, then run it:\n  bash %s\n", shown, shown)
			fmt.Fprintf(w, "\nThat fills ConfigHub and changes no cluster. Then move the estate onto it:\n  ARGOCD_CONTEXT=<kubectl context> bash %s\n", filepath.Join(filepath.Dir(shown), "handover.sh"))
			return nil
		},
	}
	apply.Flags().StringVar(&af.Prefix, "prefix", "argo", "prefix for everything the plan would create in ConfigHub")
	apply.Flags().StringVar(&af.StageLabel, "stage-label", "", "roll out in stages by the value of this cluster label")
	apply.Flags().StringVar(&applyStages, "stages", "", "the stages in order, comma-separated (with --stage-label)")
	apply.Flags().StringVar(&af.RepoRoot, "repo-root", "", "the checkout Applications' source paths are relative to")
	apply.Flags().StringVar(&out, "out", "", "directory for the files and the scripts")

	var checkNS, checkApp, checkSpace, checkUnit, checkDest, kubeContext string
	var checkDeep bool
	var checkJSON bool
	check := &cobra.Command{
		Use:   "check --application <name> --space <space> --unit <unit>",
		Short: "Compare what Argo owns on the cluster with what the release holds; changes nothing",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			argo.KubeContext = kubeContext
			if checkApp == "" || checkSpace == "" || checkUnit == "" {
				return fmt.Errorf("check needs --application, --space and --unit")
			}
			live, err := argo.LiveInventory(argo.Run, checkNS, checkApp)
			if err != nil {
				return err
			}
			stored, err := argo.Run("cub", "unit", "data", "--space", checkSpace, checkUnit)
			if err != nil {
				return err
			}
			held, err := argo.ObjectsIn(stored)
			if err != nil {
				return err
			}
			cmp := argo.CompareInventory(live, held, checkDest)
			var fields []argo.FieldDiff
			if checkDeep {
				fields, err = argo.CompareFields(argo.Run, checkDest, stored)
				if err != nil {
					return err
				}
			}
			w := c.OutOrStdout()
			argo.KubeContext = kubeContext
			if checkJSON {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				if err := enc.Encode(cmp); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(w, "%s: %d objects match what Argo owns\n", checkApp, cmp.Same)
				if checkDeep {
					if len(fields) == 0 {
						fmt.Fprintf(w, "  and every field the release sets already has that value on the cluster\n")
					}
					for _, d := range fields {
						fmt.Fprintf(w, "  %s\n", d)
						if h := argo.ByHand(d.Managers); len(h) > 0 {
							fmt.Fprintf(w, "    %s has written this object, so this is likely a hand edit rather than the source moving on\n", strings.Join(h, ", "))
						}
					}
				}
				for _, l := range cmp.WouldPrune {
					fmt.Fprintf(w, "  %s\n", l)
				}
				for _, l := range cmp.WouldAdd {
					fmt.Fprintf(w, "  %s\n", l)
				}
				for _, l := range cmp.Notes {
					fmt.Fprintf(w, "  note: %s\n", l)
				}
			}
			if !cmp.OK() || len(fields) > 0 {
				return errProblems{}
			}
			return nil
		},
	}
	check.Flags().BoolVar(&checkDeep, "fields", false, "also compare every field the release sets with the object on the cluster, and say who last wrote it")
	check.Flags().StringVar(&checkNS, "namespace", "argocd", "the namespace Argo CD's Applications live in")
	check.Flags().StringVar(&checkApp, "application", "", "the Application to read the owned objects of")
	check.Flags().StringVar(&checkSpace, "space", "", "the ConfigHub Space holding the variant")
	check.Flags().StringVar(&checkUnit, "unit", "", "the unit in that Space")
	check.Flags().StringVar(&checkDest, "destination-namespace", "", "the Application's destination namespace, where objects without one land")
	check.Flags().StringVar(&kubeContext, "kube-context", "", "the kubectl context of the cluster to read; without it kubectl's current context is used, which may be another cluster")
	check.Flags().BoolVar(&checkJSON, "json", false, "print the comparison as JSON")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the plugin version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "cub argo %s (%s, %s)\n", version, commit, date)
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
