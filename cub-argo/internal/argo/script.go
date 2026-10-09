package argo

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// The scripts below follow `cub sveltos`'s apply.sh, which is the recorded
// shape for this model: every cub call takes --allow-exists and --quiet, every
// step checks what ConfigHub already says before changing it, and the whole
// script is safe to re-run.
//
// Targets are plain OCI, as Sveltos's are: ConfigHub publishes a release and
// the controller pulls it. The ArgoCDOCI provider is the other direction, a
// worker writing Applications into the cluster, which the handover here does
// not use.

// q quotes a string for bash single quotes.
func q(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// short is a stable short id for a change order, so a re-run finds its own.
func short(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:4])
}

// controlSpace is one app-of-apps parent's Space and the files it would hold.
type controlSpace struct {
	Parent string
	Space  string
	Files  []string
}

// controlSpaces walks the tree for the layers a handover would repoint.
func (p *Plan) controlSpaces(prefix string) []controlSpace {
	var out []controlSpace
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Kind == "Application" && len(n.Children) > 0 {
			seen := map[string]bool{}
			var files []string
			for _, c := range n.Children {
				if c.File != "" && !seen[c.File] {
					seen[c.File] = true
					files = append(files, c.File)
				}
			}
			sort.Strings(files)
			out = append(out, controlSpace{
				Parent: n.Name,
				Space:  fmt.Sprintf("%s-%s-children", prefix, n.Name),
				Files:  files,
			})
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range p.Tree {
		walk(n)
	}
	return out
}

// Workflow is the stages a component's changes move through, written beside
// the script as change-workflow.yaml.
func Workflow(c *Component) string {
	var b strings.Builder
	b.WriteString("# The stages a change to this component moves through, and what each waits\n")
	b.WriteString("# for. A release is published into a stage only once the change is approved\n")
	b.WriteString("# there, and a stage is entered only once the stage ahead has released it.\n")
	b.WriteString("#\n")
	b.WriteString("# approval is not one of the built-in gates (Validated, Released, Healthy),\n")
	b.WriteString("# so it is declared here as the Attestation each stage's release waits for.\n")
	b.WriteString("# AllowAuthors belongs to that declaration, not to the workflow: it lets\n")
	b.WriteString("# whoever promoted a change also approve it, which one person trying this\n")
	b.WriteString("# out needs.\n")
	b.WriteString("#\n")
	b.WriteString("# As written this is a SINGLE-OPERATOR workflow. An approval recorded under\n")
	b.WriteString("# it shows the change was approved, not that anyone but its author looked.\n")
	b.WriteString("# To require a separate reviewer, set AllowAuthors: false (and, if wanted,\n")
	b.WriteString("# Count: 2 or FromUserIDs), then replace the live workflow with this file:\n")
	b.WriteString("#   cub changeworkflow update --space <base Space> rollout --filename change-workflow.yaml\n")
	b.WriteString("# It governs change orders created after that, not one already under way.\n")
	b.WriteString("Slug: rollout\n")
	b.WriteString("AttestationPrerequisites:\n")
	b.WriteString("  - Name: approval\n")
	b.WriteString("    AllowAuthors: true\n")
	b.WriteString("Stages:\n")
	for i, st := range c.Stages {
		b.WriteString(fmt.Sprintf("  - Name: %s\n", st.Name))
		b.WriteString(fmt.Sprintf("    WhereSpace: Labels.Stage = '%s'\n", st.Name))
		if i > 0 {
			b.WriteString("    Prerequisites:\n      - Released\n")
		}
		b.WriteString("    ReleasePrerequisites:\n      - approval\n")
	}
	return b.String()
}

// stagesJSON is the patch that adds a stage a joining cluster needs. Only the
// stages are patched, so approval settings made since are kept.
func stagesJSON(c *Component) string {
	type stage struct {
		Name                 string   `json:"Name"`
		WhereSpace           string   `json:"WhereSpace"`
		Prerequisites        []string `json:"Prerequisites,omitempty"`
		ReleasePrerequisites []string `json:"ReleasePrerequisites"`
	}
	var stages []stage
	for i, st := range c.Stages {
		s := stage{
			Name:                 st.Name,
			WhereSpace:           fmt.Sprintf("Labels.Stage = '%s'", st.Name),
			ReleasePrerequisites: []string{"approval"},
		}
		if i > 0 {
			s.Prerequisites = []string{"Released"}
		}
		stages = append(stages, s)
	}
	out, _ := json.Marshal(map[string]any{"Stages": stages})
	return string(out)
}

func stageNames(c *Component) string {
	var names []string
	for _, st := range c.Stages {
		names = append(names, st.Name)
	}
	return strings.Join(names, ",")
}

// ApplyScript writes what fills ConfigHub. It changes nothing on any cluster:
// after it, ConfigHub holds a complete copy that nothing reads yet, and
// handover.sh is what moves the estate onto it.
func ApplyScript(p *Plan, prefix, repoRel string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	add("#!/usr/bin/env bash")
	add("# Fill ConfigHub with this Argo CD estate: one base per component, one")
	add("# variant per cluster, one Target per cluster. Written by `cub argo apply`.")
	add("# Read it, then run it:")
	add("#")
	add("#   bash apply.sh")
	add("#")
	add("# Nothing here touches a cluster. Argo goes on syncing Git throughout, and")
	add("# ConfigHub ends up holding a parallel copy that nothing reads. handover.sh")
	add("# is what moves the estate onto it; until you run that, nothing on any")
	add("# cluster reads what this creates, and cleanup.sh beside this script takes")
	add("# it all back out. ConfigHub refuses a Space while a Target, a worker, a")
	add("# Release or a Tag still points at it, so the order matters and that")
	add("# script has it.")
	add("#")
	add("# cub uses its current context; set CUB_CONTEXT to choose another. Every")
	add("# step is safe to re-run, which is also how a cluster that joined since")
	add("# gets its variants.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add("# The overlays are rendered from the repository this plan read. Paths below")
	add("# are written from its top, so the script needs to know where that is.")
	if repoRel == "" {
		repoRel = "."
	}
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRel))
	add(`[ -d "$REPO_ROOT" ] || { echo "REPO_ROOT=$REPO_ROOT is not a directory: point it at the repository checkout"; exit 1; }`)
	add("")
	add("# Re-running picks up where ConfigHub says each first release stands: a")
	add("# finished change order is skipped, and a variant with nothing new is kept.")
	add(`rolled_out() { [ "$(cub changeorder get --space "${1%%/*}" "${1#*/}" -o jq=.ChangeOrder.Stage)" = Completed ]; }`)
	add("# A variant must hold every unit of its base before it is released: a")
	add("# reconciler removes from a cluster whatever a release no longer holds.")
	add("holds() {")
	add(`  local n; n=$(cub unit list --space "$1" -o jq=length)`)
	add(`  [ "$n" -ge "$2" ] || { echo "$1 holds $n of its $2 units; run this script again" >&2; return 1; }`)
	add("}")
	add("# A cluster that joins in a stage the workflow does not have yet adds that")
	add("# stage. Only the stages are patched, so approval settings made since stay.")
	add(`stages_are() { [ "$(cub changeworkflow get --space "$1" "$2" -o 'jq=[.ChangeWorkflow.Stages[].Name] | join(",")')" = "$3" ]; }`)
	add("# Promoting a large unit can outlast the request, and cub then reports no")
	add("# response while the server finishes. A promotion is idempotent, so it is")
	add("# asked again, and one with nothing left to promote has done it.")
	add("promote() {")
	add("  local out")
	add("  for _ in 1 2 3; do")
	add(`    out=$(cub variant promote --change-order "$1" --target-stage "$2" --quiet 2>&1 >/dev/null) && return 0`)
	add(`    case "$out" in *"nothing left to promote"*) return 0 ;; esac`)
	add("    sleep 15")
	add("  done")
	add(`  echo "$out" >&2; return 1`)
	add("}")
	add("publish() {")
	add("  local out")
	add(`  holds "$1" "$3" || return 1`)
	add(`  out=$(cub release publish "$1" --revision "ChangeOrder:$2" --quiet 2>&1) && return 0`)
	add(`  case "$out" in *"no changes were made since :latest bundle"*) echo "$1 already released" ;; *) echo "$out" >&2; return 1 ;; esac`)
	add("}")
	add("# Each source is rendered by kustomize here rather than by the plugin, so")
	add("# what ConfigHub stores is exactly what the delivery tool builds today.")
	for _, l := range buildFunc {
		add("%s", l)
	}
	add("# A variant cloned while its base held no unit holds none either, as after")
	add("# a first run that stopped part way. It gets a clone, linked to the base, so")
	add("# a change made on the base still reaches it.")
	add("has_unit() {")
	add(`  cub unit get --space "$1" "$2" >/dev/null 2>&1 && return 0`)
	add(`  cub unit create --space "$1" "$2" --upstream-space "$3" --upstream-unit "$2" --target "$4" --quiet`)
	add("}")
	add("render() {")
	add(`  mkdir -p render`)
	add(`  build "$REPO_ROOT/$1" "${3:-}" > "render/$2.yaml"`)
	add(`  [ -s "render/$2.yaml" ] || { echo "$REPO_ROOT/$1 rendered nothing" >&2; return 1; }`)
	add("}")
	add("")

	add(`step "0/5 Check before changing anything"`)
	// cub auth status, not a list of anything. A list call goes through the
	// entity API, which fails on a version skew between client and server --
	// measured: cub v0.6.2 against server v0.5.1 returns "field 'ComponentID'
	// does not exist on entity type Space" and exits 1 while the session is
	// perfectly good. Using that as the login check aborts this script on its
	// own first line and sends the reader to re-login, which cannot help.
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not signed in to ConfigHub: run 'cub auth login', or 'cub auth login --private-key <key>' on a server with no identity provider"; exit 1; }`)
	// The skew is worth saying out loud, because it is what later steps fail
	// on, and nothing else in the run would explain them.
	add(`cub auth status 2>&1 | grep -i '^Warning:' && echo "  Steps below may fail on that skew rather than on anything here."`)
	add(`command -v kustomize >/dev/null || { echo "kustomize is not on PATH; the overlays are rendered with it"; exit 1; }`)
	if p.inflatesHelm() {
		add("# A component here inflates a Helm chart through Kustomize, which needs the")
		add("# same flag Argo CD's repo server is configured with.")
		add(`KUSTOMIZE_FLAGS=${KUSTOMIZE_FLAGS:---enable-helm}`)
	}
	add("")

	add(`step "1/5 One named Target per cluster, in %s"`, targets)
	add("cub space create %s --allow-exists --quiet", targets)
	add("# A server-hosted worker has no process behind it and no role in the")
	add("# organization; it is the credential Argo reads with. Its bot user holds View")
	add("# and ViewChildren on each Target, to find it and pull its Releases, and")
	add("# EditChildren, which is what lets argobot record each Release's live status.")
	add("cub worker create --space %s server-worker --is-server-worker --org-role none --allow-exists --quiet", targets)
	add(`bot_user="$(cub worker get --space %s server-worker -o jq=.BridgeWorker.UserID | tr -d '\n')"`, targets)
	add("# The cluster Argo CD itself runs on: the control objects are released here.")
	add(`cub target create argocd --space %s --permission "View:${bot_user}" --permission "ViewChildren:${bot_user}" --permission "EditChildren:${bot_user}" --allow-exists --quiet`, targets)
	for _, c := range p.targetClusters() {
		add(`cub target create %s --space %s --permission "View:${bot_user}" --permission "ViewChildren:${bot_user}" --permission "EditChildren:${bot_user}" --allow-exists --quiet`, c, targets)
	}
	add("")

	// The control layers: one Space per app of apps, holding its children byte
	// for byte, so a handover can repoint the parent at it.
	cs := p.controlSpaces(prefix)
	if len(cs) > 0 {
		add(`step "2/5 One Space per app of apps, holding the objects it syncs"`)
		for _, s := range cs {
			add("cub component create %s --allow-exists --quiet", s.Space)
			add("cub space create %s --component %s --label Component=%s --label Role=control --allow-exists --quiet", s.Space, s.Space, s.Space)
			for _, f := range s.Files {
				unit := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
				add("cub unit create --space %s %s control/%s/%s --target %s/argocd --change-desc %s --allow-exists --quiet",
					s.Space, unit, s.Space, filepath.Base(f), targets,
					q(fmt.Sprintf("Onboard %s: what the %s app of apps syncs", unit, s.Parent)))
			}
			// A control Space needs a release Target of its own and a published
			// Release, or the parent repointed at it reads tag "latest" from a
			// repository that has none. Measured: without this the handover
			// fails with "<space>:latest: not found", which reads as a wrong
			// address rather than as an empty Space. Units carry the argocd
			// Target; the Space does not get one from that, and
			// `cub release publish` refuses a Space without a ReleaseTargetID.
			add(`tid=$(cub target get --space %s argocd -o jq=.Target.TargetID | tr -d '"')`, targets)
			add(`[ -n "$tid" ] || { echo "no argocd Target in %s: run step 1 first"; exit 1; }`, targets)
			add(`echo "{\"ReleaseTargetID\":\"$tid\"}" | cub space update --patch %s --from-stdin --quiet`, s.Space)
			// These Spaces are the app-of-apps tree itself, not a staged
			// rollout, so they have no ChangeWorkflow and publish directly.
			add(`cub release publish %s --quiet 2>&1 | grep -v 'no changes were made' || true`, s.Space)
		}
		// cub variant create adds an Application for a new variant when its
		// Target names the Space a root reads. The Application it writes
		// deploys to the cluster Argo CD runs on, so only that cluster's
		// Target is marked; a variant for another cluster gets its Application
		// from move-applications.sh, which keeps its destination.
		// Every Target the plan makes, so Argo CD's own cluster is marked
		// whether it has a cluster Secret or, as usual, none.
		server := map[string]string{}
		for _, c := range p.Clusters {
			server[c.Name] = c.Server
		}
		for _, name := range p.targetClusters() {
			if name == "in-cluster" || server[name] == localServer {
				add("# A variant made later for %s, Argo CD's own cluster, gets its Application", name)
				add("# in %s, which %s reads, from cub variant create itself.", cs[0].Space, cs[0].Parent)
				add(`echo '{"Annotations":{"confighub.com/argo-apps-space":"%s"}}' | cub target update --patch --space %s %s --from-stdin --quiet`, cs[0].Space, targets, name)
			}
		}
		add("")
	}

	add(`step "3/5 One component per app: a base holding what its overlay renders to, and a rollout workflow"`)
	for _, c := range p.Components {
		base := c.Base
		add("cub component create %s-%s --allow-exists --quiet", prefix, c.Name)
		add("cub space create %s --component %s-%s --label Component=%s-%s --label Role=base --allow-exists --quiet", base, prefix, c.Name, prefix, c.Name)
		if dir := c.baseDir(); dir != "" {
			add("render %s %s", q(dir), q(c.Name+"-base"))
			add("cub unit create --space %s %s render/%s-base.yaml --change-desc %s --allow-exists --quiet",
				base, c.Name, c.Name, q(fmt.Sprintf("Onboard %s from %s", c.Name, dir)))
		} else if v, ok := c.firstRendered(); ok {
			// The layout names no shared base, so the base starts as the first
			// variant's render. Each variant is then overwritten with its own;
			// without a unit here there would be nothing in them to overwrite.
			add("render %s %s%s", q(v.Path), q(c.Name+"-base"), v.recurseArg())
			add("cub unit create --space %s %s render/%s-base.yaml --change-desc %s --allow-exists --quiet",
				base, c.Name, c.Name, q(fmt.Sprintf("Onboard %s from %s, its first variant's source: the layout names no shared base", c.Name, v.Path)))
		}
		add("cub changeworkflow create --space %s rollout --filename %s/change-workflow.yaml --allow-exists --quiet", base, c.Name)
		add("stages_are %s rollout %s || echo %s | cub changeworkflow update --patch --space %s rollout --from-stdin --quiet",
			base, q(stageNames(c)), q(stagesJSON(c)), base)
	}
	add("")

	add(`step "4/5 One variant per cluster, each holding what its overlay renders to"`)
	add("# Each variant's Application is the one Argo CD runs already; after the")
	add("# handover, move-applications.sh makes it a Unit. So a cub that would add a")
	add("# second Application when it makes a variant for an Argo cluster is told not to.")
	add(`no_argo_app=; case "$(cub variant create --help 2>&1)" in *--no-argo-app*) no_argo_app=--no-argo-app ;; esac`)
	add("# Once handover.sh has begun, a variant with a release is ConfigHub's: changes")
	add("# to it are made there, through change orders, and re-rendering it from Git")
	add("# would publish Git over them. Measured: re-run for a joining cluster, that")
	add("# rolled every existing cluster back to Git. Before the handover Git is still")
	add("# what Argo applies, so every variant is rendered again, which is how a change")
	add("# in Git since the last run is picked up.")
	add(`released() {`)
	add(`  local out; out=$(cub release get --space "$1" --oci-reference latest -o jq=.Release.ReleaseNum 2>&1) && return 0`)
	add(`  case "$out" in *"not found"*|*"no release"*|*"404"*) return 1 ;; esac`)
	add(`  echo "could not tell whether $1 has a release: $out" >&2; exit 1`)
	add(`}`)
	add(`handed_over() { [ -s handover-state/argo.txt ]; }`)
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add("if handed_over && released %s; then echo %s; else", v.Space, q("  "+v.Space+" is released and handed over: ConfigHub holds it, so it is left as it is"))
				add("  cub variant create %s %s --stage %s --space-pattern template:%s --target %s/%s --space-label Role=deployment --space-label Cluster=%s $no_argo_app --allow-exists --quiet",
					v.Cluster, c.Base, st.Name, v.Space, targets, v.Cluster, v.Cluster)
				if v.Path != "" && v.Path != "(multi-source)" {
					add("  render %s %s%s", q(v.Path), q(v.Space), v.recurseArg())
					add("  has_unit %s %s %s %s/%s", v.Space, c.Name, c.Base, targets, v.Cluster)
					add("  cub unit update --space %s %s render/%s.yaml --change-desc %s --quiet",
						v.Space, c.Name, v.Space, q(fmt.Sprintf("What %s renders for %s", v.Path, v.Cluster)))
				}
				add("  holds %s 1", v.Space)
				add("fi")
			}
		}
	}
	add("")

	add(`step "5/5 Release each variant, stage by stage: promote, approve, publish"`)
	for _, c := range p.Components {
		var all []string
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				all = append(all, v.Space)
			}
		}
		if len(all) == 0 {
			continue
		}
		// A cluster joining after the handover goes through every stage: a
		// later stage cannot be promoted while an earlier one selects nothing
		// (measured). So its change order promotes into the clusters already
		// released too, and this script approves it; a change in the base
		// that one of them has not taken would reach it that way, unreviewed.
		// Stop instead.
		add("if handed_over; then")
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add(`  if released %s; then`, v.Space)
				add(`    up=$(cub unit get --space %s %s -o jq=.Unit.UpstreamRevisionNum)`, v.Space, c.Name)
				add(`    cmp -s <(cub revision data --space %s %s "$up") <(cub unit data --space %s %s) || { echo %s >&2; exit 1; }`, c.Base, c.Name, c.Base, c.Name,
					q(fmt.Sprintf("%s holds a change %s has not taken. A joining cluster's first release goes through every stage, so this run would promote that change into %s and approve it. Finish that change, or undo it in the base, then run this again.", c.Base, v.Space, v.Cluster)))
				add(`  fi`)
			}
		}
		add("fi")
		order := "onboard-" + short(c.Base, strings.Join(all, ","))
		add("cub changeorder create --space %s %s --change-workflow %s/rollout --description %s --allow-exists --quiet",
			c.Base, order, c.Base, q("First release of "+strings.Join(all, ", ")))
		add("if rolled_out %s/%s; then", c.Base, order)
		add("  echo %s", q(c.Name+": every variant in this plan is released"))
		add("else")
		for _, st := range c.Stages {
			add("  promote %s/%s %s", c.Base, order, st.Name)
			add("  cub variant approve --change-order %s/%s --stage %s --quiet", c.Base, order, st.Name)
			for _, v := range st.Variants {
				add("  publish %s %s/%s 1", v.Space, c.Base, order)
			}
		}
		add("fi")
	}
	add("")
	add("echo")
	add("echo %s", q("Done. ConfigHub holds this estate and nothing reads it yet."))
	add("echo %s", q("Argo is still syncing Git. Read handover.sh next: that is the step that moves it."))
	return strings.Join(L, "\n") + "\n"
}

// baseDir is the directory a component's base renders from: the shared base of
// its overlays, when every variant builds on the same one.
func (c *Component) baseDir() string {
	if len(c.Stages) == 0 {
		return ""
	}
	var first string
	for _, st := range c.Stages {
		for _, v := range st.Variants {
			d := commonKustomizeBase(v.Path)
			if d == "" {
				return ""
			}
			if first == "" {
				first = d
			} else if first != d {
				return ""
			}
		}
	}
	return first
}

// commonKustomizeBase guesses the base an overlay path builds on:
// apps/x/overlays/dev -> apps/x/base. Where the layout does not say, the
// component gets no base unit and each variant carries its whole render.
func commonKustomizeBase(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := len(parts) - 2; i >= 0; i-- {
		if parts[i] == "overlays" {
			return strings.Join(append(append([]string{}, parts[:i]...), "base"), "/")
		}
	}
	return ""
}

// unitHome is where an object that a handover has to edit lives in ConfigHub:
// the control Space holding it, and the Unit's name, which is the file's name
// and not always the object's own.
type unitHome struct {
	Space, Unit string
	// Parent is the app of apps that syncs the Space; File is the file the
	// Unit was made from, as the plan read it.
	Parent, File string
}

// unitHomes maps each object in the control tree to the Space and Unit that
// hold it, so a handover edits it where ConfigHub keeps it rather than on the
// cluster. Walking the tree rather than matching names: the Unit for
// ApplicationSet platform-addons is platform-addons-appset, and the Unit for
// Application storefront is storefront-app-of-apps, so a prefix match would be
// guessing where the tree already knows.
func (p *Plan) unitHomes(prefix string) map[string]unitHome {
	out := map[string]unitHome{}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Kind == "Application" && len(n.Children) > 0 {
			space := fmt.Sprintf("%s-%s-children", prefix, n.Name)
			for _, c := range n.Children {
				if c.File == "" {
					continue
				}
				unit := strings.TrimSuffix(filepath.Base(c.File), filepath.Ext(c.File))
				out[c.Kind+"/"+c.Name] = unitHome{Space: space, Unit: unit, Parent: n.Name, File: c.File}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range p.Tree {
		walk(n)
	}
	return out
}

// inflatesHelm reports whether a component inflates a Helm chart through
// Kustomize, which every render of it, in apply.sh and handover.sh alike, has
// to do with --enable-helm, as Argo CD's repo server does.
func (p *Plan) inflatesHelm() bool {
	for _, c := range p.Components {
		for _, n := range c.Notes {
			if strings.Contains(n, "enable-helm") {
				return true
			}
		}
	}
	return false
}

// targetClusters is every cluster a variant is addressed to: each cluster
// Secret, and Argo CD's own cluster, which has no Secret, when an Application
// deploys there.
func (p *Plan) targetClusters() []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range p.Clusters {
		seen[c.Name] = true
		out = append(out, c.Name)
	}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Cluster == "in-cluster" && !seen[v.Cluster] {
					seen[v.Cluster] = true
					out = append(out, v.Cluster)
				}
			}
		}
	}
	return out
}

// buildFunc is the shell function both scripts render a source with, so the
// render apply.sh stores and the one handover.sh compares it with are made the
// same way. A kustomization is built as Argo CD builds it. A plain directory
// is read as Argo CD reads one: every .yaml, .yml and .json file, at the top
// level or, with recurse, below it, hidden files aside. Those files are listed
// in a kustomization of their own, so the render is normalized exactly as an
// overlay's is, and a file that is not a Kubernetes object fails here as it
// would fail in Argo.
var buildFunc = []string{
	`build() {`,
	`  if [ -e "$1/kustomization.yaml" ] || [ -e "$1/kustomization.yml" ] || [ -e "$1/Kustomization" ]; then`,
	`    kustomize build ${KUSTOMIZE_FLAGS:-} "$1"; return`,
	`  fi`,
	`  local k dir rc=0 depth="-maxdepth 1"`,
	`  [ "${2:-}" = recurse ] && depth=""`,
	`  dir=$(cd "$1" && pwd) || return 1`,
	`  k=$(mktemp -d)`,
	`  { echo "resources:"`,
	`    (cd "$dir" && find . $depth -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) -not -path '*/.*') \`,
	`      | LC_ALL=C sort | sed "s|^\./|- $dir/|"; } > "$k/kustomization.yaml"`,
	`  kustomize build --load-restrictor LoadRestrictionsNone "$k" || rc=$?`,
	`  rm -rf "$k"; return $rc`,
	`}`,
}

// recurseArg is the build argument for a plain directory Argo reads
// recursively.
func (v Variant) recurseArg() string {
	if v.Recurse {
		return " recurse"
	}
	return ""
}

// firstRendered is the first variant with a source path to render.
func (c *Component) firstRendered() (Variant, bool) {
	for _, st := range c.Stages {
		for _, v := range st.Variants {
			if v.Path != "" && v.Path != "(multi-source)" && !strings.HasPrefix(v.Path, "(") {
				return v, true
			}
		}
	}
	return Variant{}, false
}
