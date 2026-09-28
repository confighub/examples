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
				fmt.Fprint(out, nextAfterPlan(len(p.Problems) > 0))
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
		Short: "Write the plan's files and the apply, handover and cleanup scripts; runs nothing",
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
			shown := shortestPath(script)
			fmt.Fprint(w, argo.Render(p))
			fmt.Fprintf(w, "\nWrote %s and the files it reads. Read it, then run it:\n  bash %s\n", shown, shown)
			fmt.Fprintf(w, "\nThat fills ConfigHub and changes no cluster. Then move the estate onto it:\n  ARGOCD_CONTEXT=<kubectl context> bash %s\n", filepath.Join(filepath.Dir(shown), "handover.sh"))
			fmt.Fprintf(w, "\nBefore the handover, nothing reads what apply.sh made, and\n  bash %s\ntakes it all back out.\n", filepath.Join(filepath.Dir(shown), "cleanup.sh"))
			return nil
		},
	}
	apply.Flags().StringVar(&af.Prefix, "prefix", "argo", "prefix for everything the plan would create in ConfigHub")
	apply.Flags().StringVar(&af.StageLabel, "stage-label", "", "roll out in stages by the value of this cluster label")
	apply.Flags().StringVar(&applyStages, "stages", "", "the stages in order, comma-separated (with --stage-label)")
	apply.Flags().StringVar(&af.RepoRoot, "repo-root", "", "the checkout Applications' source paths are relative to")
	apply.Flags().StringVar(&out, "out", "", "directory for the files and the scripts")

	var checkNS, checkApp, checkSpace, checkUnit, checkDest, kubeContext, checkStages string
	var checkDeep bool
	var cf argo.Options
	var checkJSON bool
	check := &cobra.Command{
		Use:   "check [dir|input.yaml|-]",
		Short: "Compare what Argo owns on the cluster with what ConfigHub holds; changes nothing",
		Long: `Compare the estate on the cluster with what ConfigHub holds for it.

Given the same input as plan, it works out every Application to check and
checks them all, which is what handover.sh does before it moves anything.
Given --application, it checks that one.

It answers two questions. Would moving the source add or remove an object:
Argo's own status.resources against the release. And with --fields, would it
change one: every field the release sets against the object on the cluster,
naming who has written it, which is how a hand edit is told from the source
moving on.

Nothing is changed either way.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			argo.KubeContext = kubeContext
			argo.SetApplicationNamespace(checkNS)
			var checks []argo.Check
			if checkApp != "" {
				if checkSpace == "" || checkUnit == "" {
					return fmt.Errorf("--application needs --space and --unit; or pass the repository instead and every Application is checked")
				}
				checks = []argo.Check{{Application: checkApp, Space: checkSpace, Unit: checkUnit, Namespace: checkDest}}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("check needs the repository to work out what to check, or --application with --space and --unit")
				}
				in, err := argo.Load(c.InOrStdin(), args)
				if err != nil {
					return err
				}
				cf.Stages = split(checkStages)
				p, err := argo.Build(in, cf)
				if err != nil {
					return err
				}
				checks = argo.ChecksFor(p)
				if len(checks) == 0 {
					return fmt.Errorf("the plan has no variants to check; run plan to see why")
				}
			}
			w := c.OutOrStdout()
			bad := 0
			for _, ck := range checks {
				r, err := argo.RunCheck(argo.Run, ck, checkDeep)
				if err != nil {
					fmt.Fprintf(w, "%s: %v\n", ck.Application, err)
					bad++
					continue
				}
				fmt.Fprintf(w, "%s: %d objects match what Argo owns\n", ck.Application, r.Inventory.Same)
				for _, l := range append(r.Inventory.WouldPrune, r.Inventory.WouldAdd...) {
					fmt.Fprintf(w, "  %s\n", l)
				}
				for _, l := range r.Inventory.Notes {
					fmt.Fprintf(w, "  note: %s\n", l)
				}
				if checkDeep && len(r.Fields) == 0 && r.Inventory.OK() {
					fmt.Fprintf(w, "  and every field the release sets already has that value on the cluster\n")
				}
				for _, d := range r.Fields {
					fmt.Fprintf(w, "  %s\n", d)
					if h := argo.ByHand(d.Managers); len(h) > 0 {
						fmt.Fprintf(w, "    %s has written this object, so this is likely a hand edit rather than the source moving on\n", strings.Join(h, ", "))
					}
				}
				if !r.OK() {
					bad++
				}
			}
			if len(checks) > 1 {
				fmt.Fprintf(w, "\n%d of %d clean\n", len(checks)-bad, len(checks))
			}
			if bad > 0 {
				return errProblems{}
			}
			return nil
		},
	}
	check.Flags().BoolVar(&checkDeep, "fields", false, "also compare every field the release sets with the object on the cluster, and say who has written it")
	check.Flags().StringVar(&checkNS, "namespace", "argocd", "the namespace Argo CD's Applications live in")
	check.Flags().StringVar(&cf.Prefix, "prefix", "argo", "the prefix the plan used in ConfigHub")
	check.Flags().StringVar(&cf.StageLabel, "stage-label", "", "the cluster label the plan staged by")
	check.Flags().StringVar(&checkStages, "stages", "", "the stages in order, comma-separated")
	check.Flags().StringVar(&cf.RepoRoot, "repo-root", "", "the checkout Applications' source paths are relative to")
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

// nextAfterPlan names the command to run next, with the arguments this run was
// given. A plan that ends in a table leaves a reader to work out what to type;
// this is the one line that saves them doing it, and it repeats their own flags
// rather than a generic example so it can be pasted as it stands.
func nextAfterPlan(problems bool) string {
	if problems {
		return "\nNext: fix the problems above, then run this again. apply refuses a plan with any.\n"
	}
	var b strings.Builder
	b.WriteString("\nNext\n  ")
	for i, a := range os.Args {
		if i == 0 {
			b.WriteString("cub argo")
			continue
		}
		if a == "plan" {
			b.WriteString(" apply")
			continue
		}
		b.WriteString(" " + a)
	}
	b.WriteString(" --out ./argo-onboarding\n")
	b.WriteString("  That writes apply.sh, handover.sh and cleanup.sh. It runs nothing.\n")
	return b.String()
}

// shortestPath is the path a person would rather read. A relative path is
// usually shorter, but not when the output directory is nowhere near the
// working directory, where it becomes a run of ".." longer than the absolute.
func shortestPath(p string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil || len(rel) >= len(p) {
		return p
	}
	return rel
}
