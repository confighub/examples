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
				fmt.Fprint(out, nextAfterPlan(len(p.Problems) > 0))
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
		Short: "Write the plan's files and the apply, handover and cleanup scripts; runs nothing",
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
			shown := shortestPath(script)
			fmt.Fprint(w, flux.Render(p))
			fmt.Fprintf(w, "\nWrote %s and the files it reads. Read it, then run it:\n  bash %s\n", shown, shown)
			fmt.Fprintf(w, "\nThat fills ConfigHub and changes no cluster. Then move one cluster at a time:\n  CLUSTER=<cluster> FLUX_CONTEXT=<kubectl context> bash %s\n", filepath.Join(filepath.Dir(shown), "handover.sh"))
			fmt.Fprintf(w, "\nBefore the handover, nothing reads what apply.sh made, and\n  bash %s\ntakes it all back out.\n", filepath.Join(filepath.Dir(shown), "cleanup.sh"))
			return nil
		},
	}
	apply.Flags().StringVar(&af.Prefix, "prefix", "flux", "prefix for everything the plan would create in ConfigHub")
	apply.Flags().StringVar(&applyStages, "stages", "", "the clusters in rollout order, comma-separated")
	apply.Flags().StringVar(&af.ClustersDir, "clusters", "clusters", "the directory holding one directory per cluster")
	apply.Flags().StringVar(&af.RepoRoot, "repo-root", "", "the checkout Flux paths are relative to")
	apply.Flags().StringVar(&out, "out", "", "directory for the files and the scripts")

	var ckNS, ckName, ckSpace, ckUnit, ckTarget, kubeContext, ckCluster string
	var ckJSON, ckDeep bool
	var ckOpts flux.Options
	check := &cobra.Command{
		Use:   "check [fleet-repo-dir]",
		Short: "Compare what each layer applied with what ConfigHub holds; changes nothing",
		Long: `Compare the fleet on a cluster with what ConfigHub holds for it.

Given the same input as plan, it works out every layer to check and checks
them all. Given --kustomization, it checks that one.

It answers two questions. Would swapping the source add or remove an object:
Flux's own status.inventory against the release. And with --fields, would it
change one: every field the release sets against the object on the cluster,
naming who has written it, which is how a hand edit is told from the source
moving on.

One cluster at a time: pass --kube-context for the cluster to read.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			flux.KubeContext = kubeContext
			flux.SetControllerNamespace(ckNS)
			var checks []flux.Check
			if ckName != "" {
				if ckSpace == "" || ckUnit == "" {
					return fmt.Errorf("--kustomization needs --space and --unit; or pass the fleet repository instead and every layer is checked")
				}
				checks = []flux.Check{{Kustomization: ckName, Space: ckSpace, Unit: ckUnit, Namespace: ckTarget}}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("check needs the fleet repository to work out what to check, or --kustomization with --space and --unit")
				}
				in, err := flux.Load(c.InOrStdin(), args)
				if err != nil {
					return err
				}
				p, err := flux.Build(in, ckOpts)
				if err != nil {
					return err
				}
				checks = flux.ChecksFor(p)
				if ckCluster != "" {
					var keep []flux.Check
					for _, x := range checks {
						if x.Cluster == ckCluster {
							keep = append(keep, x)
						}
					}
					checks = keep
				}
				if len(checks) == 0 {
					return fmt.Errorf("no layers to check; a fleet is checked one cluster at a time, so pass --cluster with one of the plan's clusters")
				}
			}
			w := c.OutOrStdout()
			bad := 0
			var results []flux.Result
			for _, ck := range checks {
				r, err := flux.RunCheck(flux.Run, ck, ckDeep)
				if err != nil {
					if ckJSON {
						return fmt.Errorf("%s: %w", ck.Kustomization, err)
					}
					fmt.Fprintf(w, "%s: %v\n", ck.Kustomization, err)
					bad++
					continue
				}
				if !r.OK() {
					bad++
				}
				if ckJSON {
					results = append(results, r)
					continue
				}
				fmt.Fprintf(w, "%s: %d objects match what the layer applied\n", ck.Kustomization, r.Inventory.Same)
				for _, l := range append(r.Inventory.WouldPrune, r.Inventory.WouldAdd...) {
					fmt.Fprintf(w, "  %s\n", l)
				}
				for _, l := range r.Stale {
					fmt.Fprintf(w, "  %s\n", l)
				}
				if f := r.Fields; f != nil {
					for _, u := range f.Unreadable {
						fmt.Fprintf(w, "  could not read %s\n", u)
					}
					if f.Compared < f.Total {
						fmt.Fprintf(w, "  fields compared on %d of the %d objects the release holds, so this is not a clean check\n", f.Compared, f.Total)
					} else if f.Clean() && r.Inventory.OK() && len(r.Stale) == 0 {
						fmt.Fprintf(w, "  and every field the release sets, on all %d objects, already has that value on the cluster\n", f.Total)
					}
					for _, d := range f.Diffs {
						fmt.Fprintf(w, "  %s\n", d)
						if h := flux.ByHand(d.Managers); len(h) > 0 {
							fmt.Fprintf(w, "    %s has written this object, so this is likely a hand edit rather than the source moving on\n", strings.Join(h, ", "))
						}
					}
				}
			}
			if ckJSON {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				if err := enc.Encode(results); err != nil {
					return err
				}
				if bad > 0 {
					return errProblems{}
				}
				return nil
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
	check.Flags().BoolVar(&ckDeep, "fields", false, "also compare every field the release sets with the object on the cluster, and say who has written it")
	check.Flags().StringVar(&ckCluster, "cluster", "", "the cluster being checked, when reading a fleet repository")
	check.Flags().StringVar(&ckOpts.Prefix, "prefix", "flux", "the prefix the plan used in ConfigHub")
	check.Flags().StringVar(&ckOpts.ClustersDir, "clusters", "clusters", "the directory holding one directory per cluster")
	check.Flags().StringVar(&ckOpts.RepoRoot, "repo-root", "", "the checkout Flux paths are relative to")
	check.Flags().StringVar(&ckNS, "namespace", "flux-system", "the namespace the Kustomizations live in")
	check.Flags().StringVar(&ckName, "kustomization", "", "the layer to read the inventory of")
	check.Flags().StringVar(&ckSpace, "space", "", "the ConfigHub Space holding the variant")
	check.Flags().StringVar(&ckUnit, "unit", "", "the unit in that Space")
	check.Flags().StringVar(&ckTarget, "target-namespace", "", "the layer's targetNamespace, where objects without one land")
	check.Flags().StringVar(&kubeContext, "kube-context", "", "the kubectl context of the cluster to read; without it kubectl's current context is used, which may be another cluster")
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
			b.WriteString("cub flux")
			continue
		}
		if a == "plan" {
			b.WriteString(" apply")
			continue
		}
		b.WriteString(" " + a)
	}
	b.WriteString(" --out ./flux-onboarding\n")
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
