// Package cmd is the `cub flux` command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/cub-flux/internal/flux"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// hub is the plugin's connection to ConfigHub, through the SDK. It connects
// when a command first asks it something.
var hub = flux.NewHub(version)

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

  apply  writes the plan as files beside its scripts: apply.sh, which fills
         ConfigHub and touches no cluster; handover.sh, which puts one root on
         a cluster running the layers from Git and lets it take each layer
         over; join.sh, which does the same for a new cluster; and cleanup.sh.
         Nothing runs until you run them.

  status reports what each layer applied to ConfigHub as live status.
  watch  proposes each cluster added to the fleet repository.
  check  compares what a layer applied with what ConfigHub holds.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var opts flux.Options
	var stages string
	var asJSON bool
	var planFormat, exportedAt string
	plan := &cobra.Command{
		Use:   "plan <fleet-repo-dir>",
		Short: "Show the fleet ConfigHub would govern; changes nothing",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if exportedAt != "" && planFormat != "preview-json" {
				return fmt.Errorf("--exported-at is only valid with --format preview-json")
			}
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
			if planFormat != "ascii" && planFormat != "json" && planFormat != "preview-json" {
				return fmt.Errorf("unknown plan format %q (want ascii, json, or preview-json)", planFormat)
			}
			if planFormat == "preview-json" {
				if asJSON {
					return fmt.Errorf("--json and --format preview-json cannot be combined")
				}
				stamp := exportedAt
				if stamp == "" {
					stamp = time.Now().UTC().Format(time.RFC3339Nano)
				} else if _, err := time.Parse(time.RFC3339, stamp); err != nil {
					return fmt.Errorf("--exported-at must be an RFC3339 timestamp: %w", err)
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(flux.PreviewPlan(p, version, stamp)); err != nil {
					return err
				}
			} else if asJSON || planFormat == "json" {
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
	plan.Flags().StringSliceVar(&opts.Require, "require", nil, "what each stage after the first also waits for in the stage before it: Healthy, which cub flux status reports")
	plan.Flags().BoolVar(&asJSON, "json", false, "print the plan as JSON")
	plan.Flags().StringVar(&planFormat, "format", "ascii", "plan output format: ascii, json (same schema as --json), or preview-json (plugin-preview v1)")
	plan.Flags().StringVar(&exportedAt, "exported-at", "", "RFC3339 export timestamp for --format preview-json (default: current UTC time)")

	var af flux.Options
	var applyStages, out string
	apply := &cobra.Command{
		Use:   "apply <fleet-repo-dir> --out <dir>",
		Short: "Write the plan's files and the apply, handover, join and cleanup scripts; runs nothing",
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
	apply.Flags().StringSliceVar(&af.Require, "require", nil, "what each stage after the first also waits for in the stage before it: Healthy, which cub flux status reports")
	apply.Flags().StringVar(&out, "out", "", "directory for the files and the scripts")

	var ckNS, ckName, ckSpace, ckUnit, ckTarget, kubeContext, ckCluster, ckRelease string
	var ckJSON, ckDeep, ckRecord bool
	var ckOpts flux.Options
	check := &cobra.Command{
		Use:   "check [fleet-repo-dir]",
		Short: "Compare what each layer applied with what ConfigHub holds; changes nothing unless --record",
		Long: `Compare the fleet on a cluster with what ConfigHub holds for it.

Given the same input as plan, it works out every layer to check and checks
them all. Given --kustomization, it checks that one.

It answers two questions. Would swapping the source add or remove an object:
Flux's own status.inventory against the release. And with --fields, would it
change one: every field the release sets against the object on the cluster,
naming who has written it, which is how a hand edit is told from the source
moving on.

Nothing on a cluster is changed. With --record it writes one thing, to
ConfigHub: each verdict as a LiveCheck attestation.

One cluster at a time: pass --kube-context for the cluster to read.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if ckRecord && !ckDeep {
				return fmt.Errorf("--record needs --fields: a LiveCheck claims the cluster runs this release, which rests on every field it sets")
			}
			flux.KubeContext = kubeContext
			flux.SetControllerNamespace(ckNS)
			checks, err := layersFor(c, args, ckName, ckSpace, ckUnit, ckTarget, ckCluster, ckOpts)
			if err != nil {
				return err
			}
			for i := range checks {
				checks[i].Release = ckRelease
			}
			w := c.OutOrStdout()
			bad := 0
			var results []flux.Result
			for _, ck := range checks {
				r, err := flux.RunCheck(flux.Run, hub, ck, ckDeep)
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
				if ckRecord {
					id, err := flux.RecordCheck(hub, r)
					if err != nil {
						return err
					}
					r.Recorded = id
				}
				if ckJSON {
					results = append(results, r)
					continue
				}
				fmt.Fprintf(w, "%s: release %d (%s) holds %s at revision %d\n", ck.Kustomization, r.Release.Num, r.Release.ManifestDigest, ck.Unit, r.Release.UnitRevision)
				if r.Recorded != "" {
					verdict := "a Pass"
					if !r.OK() {
						verdict = "a rejection"
					}
					fmt.Fprintf(w, "  recorded %s: LiveCheck attestation %s on %s/%s revision %d\n", verdict, r.Recorded, ck.Space, ck.Unit, r.Release.UnitRevision)
				}
				if a := r.Release.HeadAhead(); a != "" {
					fmt.Fprintf(w, "  note: %s\n", a)
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
	check.Flags().StringVar(&ckRelease, "release", "", "the release to compare against, by manifest digest (sha256:...); without it, the newest published release")
	check.Flags().StringVar(&ckTarget, "target-namespace", "", "the layer's targetNamespace, where objects without one land")
	check.Flags().StringVar(&kubeContext, "kube-context", "", "the kubectl context of the cluster to read; without it kubectl's current context is used, which may be another cluster")
	check.Flags().BoolVar(&ckJSON, "json", false, "print the comparison as JSON")
	check.Flags().BoolVar(&ckRecord, "record", false, "record each verdict in ConfigHub as a LiveCheck attestation on the revision the release bundled: a Pass, or a rejection naming what differs (needs --fields)")

	var stNS, stName, stSpace, stUnit, stTarget, stContext, stCluster string
	var stJSON, stWatch, stDry bool
	var stInterval, stRefresh time.Duration
	var stOpts flux.Options
	status := &cobra.Command{
		Use:   "status [fleet-repo-dir]",
		Short: "Report what each layer applied as ConfigHub live status",
		Long: `Report what Flux applied on a cluster as ConfigHub live status.

For each layer that reads its ConfigHub Space, it reads the Kustomization and
writes the Space's confighub.com/live-status: the words ConfigHub's Healthy
gate, its change orders and its UI read.

  Synced     only when the digest Flux applied is the newest published
             release of that Space; an older one is OutOfSync
  Healthy    only when Flux checked the workloads (spec.wait or
             healthChecks), or the layer runs none; otherwise Unknown
  revision   the digest Flux applied, never one inferred

A layer still reading Git, or another Space, is not reported. A read that
fails writes nothing. It writes only when a reading changes, or when the one
ConfigHub holds is older than --refresh, which shows the reporter is alive.

One cluster at a time, like check. It writes as the cub user it runs as.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			flux.KubeContext = stContext
			flux.SetControllerNamespace(stNS)
			layers, err := layersFor(c, args, stName, stSpace, stUnit, stTarget, stCluster, stOpts)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			w := c.OutOrStdout()
			for {
				outs, err := reportOnce(layers, stRefresh, stDry)
				if stJSON {
					enc := json.NewEncoder(w)
					enc.SetIndent("", "  ")
					_ = enc.Encode(outs)
				} else {
					flux.PrintOutcomes(w, outs)
				}
				if err != nil {
					if !stWatch {
						return err
					}
					// A watch outlives a failed read: that pass wrote nothing
					// for the layer it could not read, and the next pass tries
					// again.
					fmt.Fprintf(c.ErrOrStderr(), "%s\n", err)
				}
				if !stWatch {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(stInterval):
				}
			}
		},
	}
	status.Flags().StringVar(&stCluster, "cluster", "", "the cluster being reported, when reading a fleet repository")
	status.Flags().StringVar(&stOpts.Prefix, "prefix", "flux", "the prefix the plan used in ConfigHub")
	status.Flags().StringVar(&stOpts.ClustersDir, "clusters", "clusters", "the directory holding one directory per cluster")
	status.Flags().StringVar(&stOpts.RepoRoot, "repo-root", "", "the checkout Flux paths are relative to")
	status.Flags().StringVar(&stNS, "namespace", "flux-system", "the namespace the Kustomizations live in")
	status.Flags().StringVar(&stName, "kustomization", "", "one layer to report")
	status.Flags().StringVar(&stSpace, "space", "", "the ConfigHub Space the layer reads")
	status.Flags().StringVar(&stUnit, "unit", "", "the unit in that Space")
	status.Flags().StringVar(&stTarget, "target-namespace", "", "the layer's targetNamespace")
	status.Flags().StringVar(&stContext, "kube-context", "", "the kubectl context of the cluster to read; without it kubectl's current context is used, which may be another cluster")
	status.Flags().BoolVar(&stWatch, "watch", false, "keep reporting")
	status.Flags().DurationVar(&stInterval, "interval", 30*time.Second, "how often to report, with --watch")
	status.Flags().DurationVar(&stRefresh, "refresh", 10*time.Minute, "write an unchanged reading again once the one ConfigHub holds is this old")
	status.Flags().BoolVar(&stDry, "dry-run", false, "show what would be written, and write nothing")
	status.Flags().BoolVar(&stJSON, "json", false, "print what was read and done as JSON")

	var wOpts flux.Options
	var wStages, wOut string
	var wInterval time.Duration
	var wOnce, wPull bool
	watch := &cobra.Command{
		Use:   "watch <fleet-repo-dir> --out <dir>",
		Short: "Propose each cluster that joins the fleet, and release it once a person approves",
		Long: `Propose each cluster that joins the fleet, and release it once a person approves.

A cluster joins a Flux fleet when its directory appears under clusters/ in the
fleet repository. Every --interval this reads the repository again (with
--pull, after git pull --ff-only), plans it with the options the fleet was
planned with, and for a cluster ConfigHub does not have yet it writes the plan
to --out and runs apply.sh with PROPOSE_ONLY=1: the cluster's Target, variants
and layers Units are made, its first release is promoted, and it waits in
ConfigHub for a person to approve it:

  cub variant approve --change-order <base>/<order> --stage <stage>

It approves nothing itself. On its next look it runs apply.sh again, which
publishes what was approved and, once every variant of the cluster is
released, its layers Space. The cluster is then ready, and join.sh brings it
on. The watcher does not touch any cluster. It needs CONFIGHUB_OCI, as
apply.sh does for the layers Spaces. --out/watch.log keeps what each run of
apply.sh printed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if wOut == "" {
				return fmt.Errorf("watch needs --out, the directory apply wrote for this fleet")
			}
			if os.Getenv("CONFIGHUB_OCI") == "" {
				return fmt.Errorf("watch needs CONFIGHUB_OCI, the gateway host the clusters reach, to make each joining cluster's layers Space")
			}
			wOpts.Stages = split(wStages)
			plan := func() (*flux.Plan, error) {
				if wPull {
					repo := wOpts.RepoRoot
					if repo == "" {
						repo = args[0]
					}
					if out, err := exec.Command("git", "-C", repo, "pull", "--ff-only", "--quiet").CombinedOutput(); err != nil {
						return nil, fmt.Errorf("git pull in %s: %s", repo, strings.TrimSpace(string(out)))
					}
				}
				in, err := flux.Load(c.InOrStdin(), args)
				if err != nil {
					return nil, err
				}
				return flux.Build(in, wOpts)
			}
			w := flux.NewWatcher(hub, wOpts.Prefix, wOut, plan)
			ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			for {
				if err := w.Once(c.OutOrStdout()); err != nil {
					if wOnce {
						return err
					}
					fmt.Fprintf(c.ErrOrStderr(), "%s\n", err)
				}
				if wOnce {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(wInterval):
				}
			}
		},
	}
	watch.Flags().StringVar(&wOut, "out", "", "the directory apply wrote for this fleet; the watcher writes the plan there again")
	watch.Flags().StringVar(&wOpts.Prefix, "prefix", "flux", "the prefix the plan used in ConfigHub")
	watch.Flags().StringVar(&wStages, "stages", "", "the stage order the fleet was planned with")
	watch.Flags().StringVar(&wOpts.ClustersDir, "clusters", "clusters", "the directory holding one directory per cluster")
	watch.Flags().StringVar(&wOpts.RepoRoot, "repo-root", "", "the checkout Flux paths are relative to")
	watch.Flags().StringSliceVar(&wOpts.Require, "require", nil, "what the fleet was planned to require: Healthy")
	watch.Flags().DurationVar(&wInterval, "interval", time.Minute, "how often to look")
	watch.Flags().BoolVar(&wOnce, "once", false, "look once and stop")
	watch.Flags().BoolVar(&wPull, "pull", false, "git pull --ff-only in the repository before each look")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the plugin version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "cub flux %s (%s, %s)\n", version, commit, date)
		},
	}

	root.AddCommand(plan, apply, check, status, watch, versionCmd)
	return root
}

// layersFor is the layers a command acts on: the one --kustomization names, or
// every layer the plan finds for one cluster, so check and status take the
// same input as plan.
func layersFor(c *cobra.Command, args []string, name, space, unit, target, cluster string, opts flux.Options) ([]flux.Check, error) {
	if name != "" {
		if space == "" || unit == "" {
			return nil, fmt.Errorf("--kustomization needs --space and --unit; or pass the fleet repository instead and every layer is used")
		}
		return []flux.Check{{Kustomization: name, Space: space, Unit: unit, Namespace: target}}, nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("pass the fleet repository to work out the layers, or --kustomization with --space and --unit")
	}
	in, err := flux.Load(c.InOrStdin(), args)
	if err != nil {
		return nil, err
	}
	p, err := flux.Build(in, opts)
	if err != nil {
		return nil, err
	}
	// Every kubectl call reads one context, so a run covers one cluster: a
	// fleet-wide run would read one cluster and report it as all of them.
	if cluster == "" && len(p.Clusters) > 1 {
		var names []string
		for _, cl := range p.Clusters {
			names = append(names, cl.Name)
		}
		return nil, fmt.Errorf("this fleet has %d clusters and a run reads one: pass --cluster, one of %s, with --kube-context for it", len(p.Clusters), strings.Join(names, ", "))
	}
	checks := flux.ChecksFor(p)
	// A handed-over cluster's layers are Units in its layers Space, not files
	// in this repository.
	for _, cl := range p.Clusters {
		if cl.LayersSpace == "" || (cluster != "" && cl.Name != cluster) {
			continue
		}
		more, err := flux.ChecksFromLayersSpace(hub, cl.Name, cl.LayersSpace)
		if err != nil {
			return nil, err
		}
		checks = append(checks, more...)
	}
	if cluster != "" {
		var keep []flux.Check
		for _, x := range checks {
			if x.Cluster == cluster {
				keep = append(keep, x)
			}
		}
		checks = keep
	}
	if len(checks) == 0 {
		return nil, fmt.Errorf("no layers; a fleet is read one cluster at a time, so pass --cluster with one of the plan's clusters")
	}
	return checks, nil
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

// reportOnce reads every layer and writes what changed. A layer that cannot
// be read is left as ConfigHub holds it, and the others are still reported.
func reportOnce(layers []flux.Check, refresh time.Duration, dryRun bool) ([]flux.Outcome, error) {
	now := time.Now()
	var readings []flux.Reading
	var errs []string
	for _, l := range layers {
		r, err := flux.ReadStatus(flux.Run, hub, l, now)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		readings = append(readings, r)
	}
	outs, err := flux.ReportStatus(hub, readings, refresh, dryRun, now)
	if err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return outs, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return outs, nil
}
