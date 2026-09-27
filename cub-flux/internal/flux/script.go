package flux

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// The scripts here follow `cub sveltos`'s apply.sh, which is the recorded shape
// for this model: --allow-exists and --quiet throughout, each step checking
// what ConfigHub already says, and the whole script safe to re-run.

func q(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func short(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:4])
}

// Workflow is the stages a layer's changes move through.
func Workflow(c *Component) string {
	var b strings.Builder
	b.WriteString("# The stages a change to this layer moves through, and what each waits\n")
	b.WriteString("# for. A release is published into a stage only once the change is approved\n")
	b.WriteString("# there, and a stage is entered only once the stage ahead has released it.\n")
	b.WriteString("#\n")
	b.WriteString("# AllowAuthors lets whoever promoted a change also approve it, which one\n")
	b.WriteString("# person trying this out needs. Set it false once a second person can.\n")
	b.WriteString("Slug: rollout\nAllowAuthors: true\nStages:\n")
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

func stagesJSON(c *Component) string {
	type stage struct {
		Name                 string   `json:"Name"`
		WhereSpace           string   `json:"WhereSpace"`
		Prerequisites        []string `json:"Prerequisites,omitempty"`
		ReleasePrerequisites []string `json:"ReleasePrerequisites"`
	}
	var stages []stage
	for i, st := range c.Stages {
		s := stage{Name: st.Name, WhereSpace: fmt.Sprintf("Labels.Stage = '%s'", st.Name),
			ReleasePrerequisites: []string{"approval"}}
		if i > 0 {
			s.Prerequisites = []string{"Released"}
		}
		stages = append(stages, s)
	}
	out, _ := json.Marshal(map[string]any{"Stages": stages})
	return string(out)
}

func stageNames(c *Component) string {
	var n []string
	for _, st := range c.Stages {
		n = append(n, st.Name)
	}
	return strings.Join(n, ",")
}

// ApplyScript fills ConfigHub. It touches no cluster: Flux goes on
// reconciling Git throughout, and handover.sh is what moves the fleet.
func ApplyScript(p *Plan, prefix, repoRel string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	add("#!/usr/bin/env bash")
	add("# Fill ConfigHub with this Flux fleet: one base per layer, one variant per")
	add("# cluster, one Target per cluster. Written by `cub flux apply`. Read it,")
	add("# then run it:")
	add("#")
	add("#   bash apply.sh")
	add("#")
	add("# Nothing here touches a cluster. Flux goes on reconciling Git throughout,")
	add("# and ConfigHub ends up holding a parallel copy that nothing reads.")
	add("# handover.sh is what moves the fleet onto it; until you run that, deleting")
	add("# these Spaces puts you back exactly where you started.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add("# Flux paths are written from the top of the repository, so the script needs")
	add("# to know where that is.")
	if repoRel == "" {
		repoRel = "."
	}
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRel))
	add(`[ -d "$REPO_ROOT" ] || { echo "REPO_ROOT=$REPO_ROOT is not a directory: point it at the repository checkout"; exit 1; }`)
	add("")
	add(`rolled_out() { [ "$(cub changeorder get --space "${1%%/*}" "${1#*/}" -o jq=.ChangeOrder.Stage)" = Completed ]; }`)
	add("# A variant must hold every unit of its base before it is released: Flux")
	add("# prunes from a cluster whatever a release no longer holds.")
	add("holds() {")
	add(`  local n; n=$(cub unit list --space "$1" -o jq=length)`)
	add(`  [ "$n" -ge "$2" ] || { echo "$1 holds $n of its $2 units; run this script again" >&2; return 1; }`)
	add("}")
	add(`stages_are() { [ "$(cub changeworkflow get --space "$1" "$2" -o 'jq=[.ChangeWorkflow.Stages[].Name] | join(",")')" = "$3" ]; }`)
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
	add("# Each layer is rendered by kustomize here rather than by the plugin, so")
	add("# what ConfigHub stores is exactly what Flux builds today. postBuild")
	add("# substitution is Flux's, applied on the cluster, so it is left in place.")
	add("render() {")
	add("  mkdir -p render")
	add(`  kustomize build ${KUSTOMIZE_FLAGS:-} "$REPO_ROOT/$1" > "render/$2.yaml"`)
	add(`  [ -s "render/$2.yaml" ] || { echo "kustomize build $REPO_ROOT/$1 rendered nothing" >&2; return 1; }`)
	add("}")
	add("")

	add(`step "0/4 Check before changing anything"`)
	add(`cub space list --quiet >/dev/null || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	add(`command -v kustomize >/dev/null || { echo "kustomize is not on PATH; the layers are rendered with it"; exit 1; }`)
	add("")

	add(`step "1/4 One named Target per cluster, in %s"`, targets)
	add("cub space create %s --allow-exists --quiet", targets)
	add("# A server-hosted worker has no process behind it and no role in the")
	add("# organization; it holds the Targets and is the credential Flux reads with.")
	add("cub worker create --space %s server-worker --is-server-worker --org-role none --allow-exists --quiet", targets)
	for _, c := range p.Clusters {
		add("cub target create %s '{}' server-worker --space %s --provider OCI --toolchain Any --allow-exists --quiet", c.Name, targets)
	}
	add("")

	add(`step "2/4 One component per layer: a base holding what it renders to, and a rollout workflow"`)
	for _, c := range p.Components {
		add("cub component create %s-%s --allow-exists --quiet", prefix, c.Name)
		add("cub space create %s --component %s-%s --label Component=%s-%s --label Role=base --allow-exists --quiet", c.Base, prefix, c.Name, prefix, c.Name)
		if c.BaseDir != "" && !strings.Contains(c.BaseDir, ", ") {
			add("render %s %s", q(c.BaseDir), q(c.Name+"-base"))
			add("cub unit create --space %s %s render/%s-base.yaml --change-desc %s --allow-exists --quiet",
				c.Base, c.Name, c.Name, q(fmt.Sprintf("Onboard the %s layer from %s", c.Name, c.BaseDir)))
		}
		add("cub changeworkflow create --space %s rollout --filename %s/change-workflow.yaml --allow-exists --quiet", c.Base, c.Name)
		add("stages_are %s rollout %s || echo %s | cub changeworkflow update --patch --space %s rollout --from-stdin --quiet",
			c.Base, q(stageNames(c)), q(stagesJSON(c)), c.Base)
	}
	add("")

	add(`step "3/4 One variant per cluster, each holding what its own path renders to"`)
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add("cub variant create %s %s --stage %s --space-pattern template:%s --target %s/%s --space-label Role=deployment --space-label Cluster=%s --allow-exists --quiet",
					v.Cluster, c.Base, st.Name, v.Space, targets, v.Cluster, v.Cluster)
				add("render %s %s", q(v.Path), q(v.Space))
				add("cub unit update --space %s %s render/%s.yaml --change-desc %s --quiet",
					v.Space, c.Name, v.Space, q(fmt.Sprintf("What %s renders for %s", v.Path, v.Cluster)))
				add("holds %s 1", v.Space)
			}
		}
	}
	add("")

	add(`step "4/4 Release each variant, stage by stage: promote, approve, publish"`)
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
	add("echo %s", q("Done. ConfigHub holds this fleet and nothing reads it yet."))
	add("echo %s", q("Flux is still reconciling Git. Read handover.sh next: that is the step that moves it."))
	return strings.Join(L, "\n") + "\n"
}
