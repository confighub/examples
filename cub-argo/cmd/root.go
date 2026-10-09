// Package cmd is the `cub argo` command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/confighub/examples/cub-argo/internal/argo"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// hub is the plugin's connection to ConfigHub, through the SDK. It connects
// when a command first asks it something.
var hub = argo.NewHub(version)

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

  plan    reads Applications, ApplicationSets, AppProjects and cluster Secrets,
          either a repository directory or 'kubectl get' output, and shows the
          fleet ConfigHub would govern: one base per ApplicationSet (or list
          element), one variant per Application it generates, addressed to that
          one cluster, and the control tree that stays as it is. Offline: no
          account, no cluster, nothing changes.

  apply   writes the plan as files beside its scripts: apply.sh, which fills
          ConfigHub and touches no cluster; handover.sh, which repoints each
          Application's source at ConfigHub, top down; move-applications.sh,
          which makes each generated Application a Unit reading its own Space,
          one stage at a time; argobot.sh, which runs argobot beside Argo CD;
          and cleanup.sh, the way back. Nothing runs until you run them.

  check   compares what Argo CD owns on the cluster with what ConfigHub holds,
          which is what handover.sh does before it moves anything. It changes
          nothing unless --record.

  status  reports what each handed-over Application synced as ConfigHub live
          status, once or with --watch: the route without argobot, so
          ConfigHub's Healthy gate and its change orders can read it.`,
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
		Short: "Write the plan's files and the apply, handover, move-applications, argobot and cleanup scripts; runs nothing",
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
	var destContext, destDeclared, checkRelease string
	var checkDeep bool
	var cf argo.Options
	var checkJSON, checkRecord bool
	check := &cobra.Command{
		Use:   "check [dir|input.yaml|-]",
		Short: "Compare what Argo owns on the cluster with what ConfigHub holds; changes nothing unless --record",
		Long: `Compare the estate on the cluster with what ConfigHub holds for it.

Given the same input as plan, it works out every Application to check and
checks them all, which is what handover.sh does before it moves anything.
Given --application, it checks that one.

It answers two questions. Would moving the source add or remove an object:
Argo's own status.resources against the release. And with --fields, would it
change one: every field the release sets against the object on the cluster,
naming who has written it, which is how a hand edit is told from the source
moving on.

Nothing on a cluster is changed either way. With --record it writes one
thing, to ConfigHub: each verdict as a LiveCheck attestation.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if checkRecord && !checkDeep {
				return fmt.Errorf("--record needs --fields: a LiveCheck claims the cluster runs this release, which rests on every field it sets")
			}
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
			for i := range checks {
				checks[i].Release = checkRelease
			}
			workloads := func(d argo.Destination) (argo.Runner, error) {
				ctx, err := argo.ResolveDestination(d, kubeContext,
					argo.DestinationAccess{Context: destContext, Declared: destDeclared}, argo.ServerOf)
				if err != nil {
					return nil, err
				}
				return argo.RunIn(ctx), nil
			}
			var results []argo.Result
			for _, ck := range checks {
				r, err := argo.RunCheck(argo.Run, hub, ck, checkDeep, workloads)
				if err != nil {
					if checkJSON {
						return fmt.Errorf("%s: %w", ck.Application, err)
					}
					fmt.Fprintf(w, "%s: %v\n", ck.Application, err)
					bad++
					continue
				}
				if !r.OK() {
					bad++
				}
				if checkRecord {
					id, err := argo.RecordCheck(hub, r)
					if err != nil {
						return err
					}
					r.Recorded = id
				}
				if checkJSON {
					results = append(results, r)
					continue
				}
				fmt.Fprintf(w, "%s: release %d (%s) holds %s at revision %d\n", ck.Application, r.Release.Num, r.Release.ManifestDigest, ck.Unit, r.Release.UnitRevision)
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
				fmt.Fprintf(w, "%s: %d objects match what Argo owns\n", ck.Application, r.Inventory.Same)
				for _, l := range append(r.Inventory.WouldPrune, r.Inventory.WouldAdd...) {
					fmt.Fprintf(w, "  %s\n", l)
				}
				for _, l := range r.Inventory.Notes {
					fmt.Fprintf(w, "  note: %s\n", l)
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
						if h := argo.ByHand(d.Managers); len(h) > 0 {
							fmt.Fprintf(w, "    %s has written this object, so this is likely a hand edit rather than the source moving on\n", strings.Join(h, ", "))
						}
					}
				}
			}
			if checkJSON {
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
	check.Flags().StringVar(&checkRelease, "release", "", "the release to compare against, by manifest digest (sha256:...); without it, the newest published release")
	check.Flags().StringVar(&destContext, "destination-context", "", "with --fields, the kubectl context of the cluster the Application deploys to, where its objects are read; not needed when Argo CD deploys to its own cluster")
	check.Flags().StringVar(&destDeclared, "destination", "", "the Argo destination (server address or cluster name) --destination-context reaches, when kubectl reaches it by another address")
	check.Flags().StringVar(&kubeContext, "kube-context", "", "the kubectl context of the cluster Argo CD runs on, where Applications are read; without it kubectl's current context is used, which may be another cluster")
	check.Flags().BoolVar(&checkJSON, "json", false, "print the comparison as JSON")
	check.Flags().BoolVar(&checkRecord, "record", false, "record each verdict in ConfigHub as a LiveCheck attestation on the revision the release bundled: a Pass, or a rejection naming what differs (needs --fields)")

	var stNS, stApp, stSpace, stContext, stStages string
	var stJSON, stWatch, stDry, stHard bool
	var stInterval, stRefresh time.Duration
	var stOpts argo.Options
	status := &cobra.Command{
		Use:   "status [dir|input.yaml|-]",
		Short: "Report what each handed-over Application synced as ConfigHub live status",
		Long: `Report what Argo CD synced as ConfigHub live status.

For each Application that reads its ConfigHub Space, it reads the Application
and records its live status on the Release it synced: what ConfigHub's Healthy
gate, its change orders and its UI read. argobot does the same where it runs;
this is for an estate that runs none. It needs ConfigHub v0.8.2 or newer, which
is where live status moved from the Space onto the Release.

  release    the published release whose digest Argo CD records in
             status.sync.revision. The Healthy gate reads the newest release
             only, so a reading of an older one does not pass it
  sync       Argo CD's own; Unknown while Argo CD reports an error
  health     Argo CD's own, which covers every resource it owns

Given the same input as plan, it reports every Application the plan governs:
each variant, and each app of apps whose children moved into a control Space.
An Application still reading Git is not reported. A read that fails writes
nothing. It writes only when a reading changes, or when the one the Release
holds is older than --refresh, which shows the reporter is alive. A reading
another reporter (argobot) wrote is left alone while it is fresher than
--refresh. Recording takes Edit on the Release: your own, or EditChildren on
its Target.

It writes as the cub user it runs as.

Argo CD caches the digest it resolved for "latest", so a newly published
release is not read on its own; argobot asks for a hard refresh on every
publish. With --hard-refresh this does the same: when an Application has
synced an older release than the newest published one, it annotates it
argocd.argoproj.io/refresh=hard, once per release. That is the one change it
makes on a cluster.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			argo.KubeContext = stContext
			argo.SetApplicationNamespace(stNS)
			var checks []argo.StatusCheck
			if stApp != "" {
				if stSpace == "" {
					return fmt.Errorf("--application needs --space; or pass the repository instead and every Application is reported")
				}
				checks = []argo.StatusCheck{{Application: stApp, Space: stSpace}}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("status needs the repository to work out what to report, or --application with --space")
				}
				in, err := argo.Load(c.InOrStdin(), args)
				if err != nil {
					return err
				}
				stOpts.Stages = split(stStages)
				p, err := argo.Build(in, stOpts)
				if err != nil {
					return err
				}
				checks = argo.StatusChecksFor(p, stOpts.Prefix)
				if len(checks) == 0 {
					return fmt.Errorf("the plan governs no Application; run plan to see why")
				}
			}
			ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			w := c.OutOrStdout()
			var refresher *argo.Refresher
			if stHard && !stDry {
				refresher = &argo.Refresher{Run: argo.Run}
			}
			for {
				outs, err := reportOnce(checks, stRefresh, stDry, refresher, c.ErrOrStderr())
				if stJSON {
					enc := json.NewEncoder(w)
					enc.SetIndent("", "  ")
					_ = enc.Encode(outs)
				} else {
					argo.PrintOutcomes(w, outs)
				}
				if err != nil {
					if !stWatch {
						return err
					}
					// A watch outlives a failed read: that pass wrote nothing
					// for the Application it could not read, and the next
					// pass tries again.
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
	status.Flags().StringVar(&stNS, "namespace", "argocd", "the namespace Argo CD's Applications live in")
	status.Flags().StringVar(&stOpts.Prefix, "prefix", "argo", "the prefix the plan used in ConfigHub")
	status.Flags().StringVar(&stOpts.StageLabel, "stage-label", "", "the cluster label the plan staged by")
	status.Flags().StringVar(&stStages, "stages", "", "the stages in order, comma-separated")
	status.Flags().StringVar(&stOpts.RepoRoot, "repo-root", "", "the checkout Applications' source paths are relative to")
	status.Flags().StringVar(&stApp, "application", "", "one Application to report")
	status.Flags().StringVar(&stSpace, "space", "", "the ConfigHub Space that Application reads")
	status.Flags().StringVar(&stContext, "kube-context", "", "the kubectl context of the cluster Argo CD runs on; without it kubectl's current context is used, which may be another cluster")
	status.Flags().BoolVar(&stWatch, "watch", false, "keep reporting")
	status.Flags().DurationVar(&stInterval, "interval", 30*time.Second, "how often to report, with --watch")
	status.Flags().DurationVar(&stRefresh, "refresh", 10*time.Minute, "write an unchanged reading again once the one ConfigHub holds is this old")
	status.Flags().BoolVar(&stDry, "dry-run", false, "show what would be written, and write nothing")
	status.Flags().BoolVar(&stHard, "hard-refresh", false, "ask Argo CD to read a newly published release it has not synced, as argobot does (annotates the Application)")
	status.Flags().BoolVar(&stJSON, "json", false, "print what was read and done as JSON")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the plugin version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "cub argo %s (%s, %s)\n", version, commit, date)
		},
	}

	var auCtx, auNS, auApp, auSpace, auGateway, auRendered string
	var auSettled bool
	appUnit := &cobra.Command{
		Use:   "application-unit",
		Short: "Print the Unit that delivers an Application from its Space; reads the cluster, changes nothing",
		Long: `Read an Application from the cluster Argo CD runs on and print it as the Unit
that delivers it from ConfigHub: the same name, project, destination and sync
policy, its source pointed at the variant's Space on the gateway, and the sync
option Prune=false, so no parent ever deletes it.

move-applications.sh runs this for each Application a retired ApplicationSet
made, and stores what it prints in the control Space the parent reads. For a
cluster that joined after the ApplicationSet was retired, nothing generated its
Application: --rendered makes it from what the template renders for that
cluster, which apply writes as apps/<space>.yaml.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if (auApp == "" && auRendered == "") || auSpace == "" || auGateway == "" {
				return fmt.Errorf("needs --application (or --rendered), --space and --gateway")
			}
			argo.KubeContext = auCtx
			argo.SetApplicationNamespace(auNS)
			var b []byte
			var err error
			if auRendered != "" {
				data, rerr := os.ReadFile(auRendered)
				if rerr != nil {
					return rerr
				}
				b, err = argo.ApplicationUnitRendered(data, auNS, auGateway, auSpace)
			} else if auSettled {
				b, err = argo.SettledApplicationUnit(argo.Run, auApp, auGateway, auSpace)
			} else {
				b, err = argo.ApplicationUnit(argo.Run, auApp, auGateway, auSpace)
			}
			if err != nil {
				return err
			}
			_, err = c.OutOrStdout().Write(b)
			return err
		},
	}
	appUnit.Flags().StringVar(&auCtx, "kube-context", "", "the kubectl context of the cluster Argo CD runs on")
	appUnit.Flags().StringVar(&auNS, "namespace", "argocd", "the namespace Argo CD's Applications live in")
	appUnit.Flags().StringVar(&auApp, "application", "", "the Application to deliver")
	appUnit.Flags().StringVar(&auSpace, "space", "", "the variant's Space, which it will read")
	appUnit.Flags().StringVar(&auGateway, "gateway", "", "the gateway address the cluster reaches, host[:port]")
	appUnit.Flags().BoolVar(&auSettled, "settled", false, "the Application reads its Space already: leave out Replace=true, which would erase its status on every sync of the parent")
	appUnit.Flags().StringVar(&auRendered, "rendered", "", "for an Application not on the cluster yet: the file apply wrote with what its ApplicationSet's template renders for its cluster (apps/<space>.yaml)")

	root.AddCommand(plan, apply, check, status, appUnit, versionCmd)
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

// reportOnce reads every Application and writes what changed. One that cannot
// be read is left as ConfigHub holds it, and the others are still reported.
func reportOnce(checks []argo.StatusCheck, refresh time.Duration, dryRun bool, refresher *argo.Refresher, log io.Writer) ([]argo.Outcome, error) {
	now := time.Now()
	var readings []argo.Reading
	var errs []string
	for _, ck := range checks {
		r, err := argo.ReadStatus(argo.Run, hub, ck, now)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		readings = append(readings, r)
	}
	outs, err := argo.ReportStatus(hub, readings, refresh, dryRun, now)
	if err != nil {
		errs = append(errs, err.Error())
	}
	if refresher != nil {
		did, err := refresher.Refresh(readings)
		for _, app := range did {
			fmt.Fprintf(log, "%s: asked Argo CD for a hard refresh, so it reads the newest release\n", app)
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return outs, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return outs, nil
}
