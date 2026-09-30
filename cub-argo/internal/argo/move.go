package argo

import (
	"fmt"
	"strings"
)

// MoveScript writes move-applications.sh: the last step of a handover, done
// through ConfigHub. Each Application a retired ApplicationSet made becomes a
// Unit named after its variant's Space, in the control Space that holds the
// ApplicationSet, reading that Space; publishing the control Space is what
// repoints it, one stage at a time. After it, every variant has its delivery
// object in ConfigHub, as a variant made by `cub variant create` on a
// cub-cluster target does.
//
// It returns "" when no ApplicationSet is a Unit in a control Space: then no
// parent syncs one from ConfigHub, and handover.sh prints the kubectl patches.
func MoveScript(p *Plan, prefix string) string {
	targets := prefix + "-targets"
	homes := p.unitHomes(prefix)
	parentOf := map[string]string{}
	for _, s := range p.controlSpaces(prefix) {
		parentOf[s.Space] = s.Parent
	}
	type move struct {
		c    *Component
		home unitHome
		v    Variant
	}
	byStage := map[string][]move{}
	var sets []*Component
	var byHand []string
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		home, known := homes["ApplicationSet/"+c.Source]
		if !known || parentOf[home.Space] == "" {
			continue
		}
		sets = append(sets, c)
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Path == "(multi-source)" {
					byHand = append(byHand, v.Application)
					continue
				}
				byStage[st.Name] = append(byStage[st.Name], move{c, home, v})
			}
		}
	}
	if len(sets) == 0 {
		return ""
	}

	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }
	add("#!/usr/bin/env bash")
	add("# Deliver each Application an ApplicationSet made from its own Space, through")
	add("# ConfigHub. Written by `cub argo apply`. Read it, then run it:")
	add("#")
	add("#   ARGOCD_CONTEXT=<kubectl context of the cluster Argo CD runs on> \\")
	add("#   CONFIGHUB_OCI=<gateway host the cluster reaches> bash move-applications.sh [stage...]")
	add("#")
	add("# With no stage named it moves every stage, in order: %s.", strings.Join(p.Stages, ", "))
	add("# Name one to move only that one, and check it before the next.")
	add("#")
	add("# Run handover.sh first, and retire each ApplicationSet as its step 5 says.")
	add("# Each Application becomes a Unit named after its variant's Space, in the")
	add("# control Space that holds its ApplicationSet. It is the Application as Argo")
	add("# CD runs it now, with its source pointed at that Space, and the sync option")
	add("# Prune=false, so no parent ever deletes it. Publishing the control Space is")
	add("# what repoints it: a reviewed change, where handover.sh printed a kubectl")
	add("# patch. The Application keeps its name and its UID; nothing is recreated.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`ctx=${ARGOCD_CONTEXT:-$(kubectl config current-context 2>/dev/null || true)}`)
	add(`[ -n "$ctx" ] || { echo "no kubectl context: set ARGOCD_CONTEXT to the cluster Argo CD runs on"; exit 1; }`)
	add(`echo "every kubectl call below uses context $ctx"`)
	add(`k() { kubectl --context "$ctx" "$@"; }`)
	add(`ns=${ARGOCD_NAMESPACE:-argocd}`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`addr=${CONFIGHUB_OCI:-$(cub target get --space %s argocd -o jq=.Target.Parameters.OCIRepository 2>/dev/null || true)}`, targets)
	add(`addr=${addr#oci://}`)
	add(`[ -n "$addr" ] || { echo "set CONFIGHUB_OCI to the gateway address the cluster reaches, as handover.sh used"; exit 1; }`)
	add(`digest() { cub release get --space "$1" --oci-reference latest -o jq=.Release.ManifestDigest | tr -d '"'; }`)
	add("")
	add("# What each Application read before it moved, and its UID, recorded once:")
	add("# its whole source, in handover-state/source-<application>.json, since a")
	add("# template can set more than the repository, path and revision (such as")
	add("# kustomize.version). The way back restores exactly this.")
	add(`state=handover-state/applications.txt; mkdir -p handover-state render; touch "$state"; moved=""; finished=0`)
	add(`record() {`)
	add(`  grep -q "^$1|" "$state" && return 0`)
	add(`  [ "${4:-}" = new ] && { printf '%%s|%%s|%%s|new|||
' "$1" "$2" "$3" >> "$state"; return 0; }`)
	add("  # Already on the gateway (moved by hand, say): there is no Git source to record.")
	add(`  case "$(k -n "$ns" get application "$1" -o jsonpath='{.spec.source.repoURL}')" in oci://*) printf '%%s|%%s|%%s|unknown|||\n' "$1" "$2" "$3" >> "$state"; return 0 ;; esac`)
	add(`  k -n "$ns" get application "$1" -o jsonpath='{.spec.source}' > "handover-state/source-$1.json"`)
	add(`  printf '%%s|%%s|%%s|%%s\n' "$1" "$2" "$3" "$(k -n "$ns" get application "$1" -o jsonpath='{.metadata.uid}{"|"}{.spec.source.repoURL}{"|"}{.spec.source.path}{"|"}{.spec.source.targetRevision}')" >> "$state"`)
	add(`}`)
	add("# The way back, per Application: take its Unit out and publish, which leaves")
	add("# the Application where it is (Prune=false), then put its source back. A")
	add("# replace, not a merge: a merge would keep what the move added.")
	add(`way_back() {`)
	add(`  local app space home uid url path rev`)
	add(`  while IFS='|' read -r app space home uid url path rev; do`)
	add(`    case " $moved " in *" $app "*) ;; *) [ "${1:-}" = all ] || continue ;; esac`)
	add(`    echo "  cub unit delete --space $home $space && cub release publish $home"`)
	add(`    if [ "$uid" = new ]; then`)
	add(`      echo "  # $app was made here for a cluster that joined later; there is no source to go back to."`)
	add(`      echo "  # Deleting it removes what it deployed, if it carries the resources finalizer:"`)
	add(`      echo "  kubectl --context '$ctx' -n '$ns' delete application $app"`)
	add(`      continue`)
	add(`    fi`)
	add(`    if [ "$uid" = unknown ]; then`)
	add(`      echo "  # $app read the gateway already when it was moved, so its Git source was never recorded: put it back by hand."`)
	add(`      continue`)
	add(`    fi`)
	add(`    if [ -s "handover-state/source-$app.json" ]; then`)
	add(`      echo "  kubectl --context '$ctx' -n '$ns' patch application $app --type json -p '[{\"op\":\"replace\",\"path\":\"/spec/source\",\"value\":$(sed "s/'/'\\\\''/g" "handover-state/source-$app.json")}]'"`)
	add(`    else`)
	add(`      echo "  kubectl --context '$ctx' -n '$ns' patch application $app --type json -p '[{\"op\":\"replace\",\"path\":\"/spec/source\",\"value\":{\"repoURL\":\"$url\",\"path\":\"$path\",\"targetRevision\":\"$rev\"}}]'"`)
	add(`    fi`)
	add(`  done < "$state"`)
	add(`}`)
	add(`stopped() {`)
	add(`  local rc=$?`)
	add(`  [ "$finished" = 1 ] && return`)
	add(`  [ -n "$moved" ] || { echo "Stopped (exit $rc) before any Application was delivered from its Space." >&2; return; }`)
	add(`  echo >&2; echo "STOPPED (exit $rc) on context $ctx. These Applications are Units now:$moved" >&2`)
	add(`  echo "Nothing is rolled back automatically. The way back:" >&2`)
	add(`  way_back >&2`)
	add(`}`)
	add(`trap stopped EXIT`)
	add("")

	add(`step "Check before changing anything"`)
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	seenParent := map[string]bool{}
	for _, c := range sets {
		home := homes["ApplicationSet/"+c.Source]
		parent := parentOf[home.Space]
		if !seenParent[parent] {
			seenParent[parent] = true
			add("# %s applies what %s holds, so it has to read it already.", parent, home.Space)
			add(`k -n "$ns" get application %s -o jsonpath='{.spec.source.repoURL}' | grep -q '^oci://' || { echo "%s still reads Git: run handover.sh first"; exit 1; }`, parent, parent)
		}
		add("# A live generator reverts any change to what it made, within a second; and a")
		add("# create-only one that still generates takes back each Application once it")
		add("# stands alone, which it does after its move. Measured on Argo CD v3.5.3.")
		add(`[ "$(k -n "$ns" get applicationset %s -o jsonpath='{.spec.syncPolicy.applicationsSync}')" = create-only ] || { echo "ApplicationSet %s is not retired yet. Retire it through its Unit in %s, as handover.sh step 5 prints, then run this again."; exit 1; }`, c.Source, c.Source, home.Space)
		add(`[ "$(k -n "$ns" get applicationset %s -o jsonpath='{.spec.generators}')" = '[{"list":{"elements":[]}}]' ] || { echo "ApplicationSet %s still generates. Set its generators to [{list: {elements: []}}] through its Unit in %s, as handover.sh step 5 prints, then run this again."; exit 1; }`, c.Source, c.Source, home.Space)
	}
	add("")

	add("# deliver <application> <space> <control space>: the Application as a Unit")
	add("# there. A Unit already there is left as it is, so a re-run carries on. An")
	add("# Application not on the cluster is one whose cluster joined after the")
	add("# retirement: it is made from what the template renders for that cluster,")
	add("# apps/<space>.yaml, which cub argo apply wrote beside this script.")
	add(`deliver() {`)
	add(`  if cub unit get --space "$3" "$2" >/dev/null 2>&1; then echo "  $1 is a Unit in $3 already"; return 0; fi`)
	add(`  if k -n "$ns" get application "$1" >/dev/null 2>&1; then`)
	add(`    record "$1" "$2" "$3"`)
	add(`    cub argo application-unit --kube-context "$ctx" --namespace "$ns" --application "$1" --space "$2" --gateway "$addr" > "render/app-$2.yaml"`)
	add(`  else`)
	add(`    [ -s "apps/$2.yaml" ] || { echo "  $1 is not on the cluster, and apps/$2.yaml is missing: plan and apply again with the cluster in the input. If the plan notes templatePatch, which it does not render, make this Application by hand" >&2; return 1; }`)
	add(`    echo "  $1 is not on the cluster: its cluster joined after the retirement, so it is made from what the template renders for it"`)
	add(`    record "$1" "$2" "$3" new`)
	add(`    cub argo application-unit --rendered "apps/$2.yaml" --namespace "$ns" --space "$2" --gateway "$addr" > "render/app-$2.yaml"`)
	add(`  fi`)
	add(`  cub unit create --space "$3" "$2" "render/app-$2.yaml" --target %s/argocd --change-desc "Deliver $1 from its Space $2" --quiet`, targets)
	add(`  moved="$moved $1"`)
	add(`  echo "  $1 is a Unit in $3, reading $2"`)
	add(`}`)
	add("# publish <control space> <parent>: and wait until the parent has synced it.")
	add(`publish() {`)
	add(`  cub release publish "$1" --quiet 2>&1 | grep -v -e 'no changes were made' -e '^$' || true`)
	add(`  local want got=""; want=$(digest "$1")`)
	add(`  k -n "$ns" annotate application "$2" argocd.argoproj.io/refresh=hard --overwrite >/dev/null`)
	add(`  for _ in $(seq 1 36); do`)
	add(`    got=$(k -n "$ns" get application "$2" -o jsonpath='{.status.sync.revision}')`)
	add(`    [ "$got" = "$want" ] && break`)
	add(`    sleep 5`)
	add(`  done`)
	add(`  [ "$got" = "$want" ] || { echo "  $2 synced ${got:-nothing it reports}, not $1's newest release $want" >&2; return 1; }`)
	add(`  echo "  $2 applied $1 at $want"`)
	add(`}`)
	add("# arrived <application> <space>: it reads its Space, has synced that Space's")
	add("# newest release, and is the same object it was.")
	add(`arrived() {`)
	add(`  local want got="" url uid was`)
	add(`  want=$(digest "$2"); was=$(grep "^$1|" "$state" | cut -d'|' -f4 || true)`)
	add(`  k -n "$ns" annotate application "$1" argocd.argoproj.io/refresh=hard --overwrite >/dev/null`)
	add(`  for _ in $(seq 1 36); do`)
	add(`    url=$(k -n "$ns" get application "$1" -o jsonpath='{.spec.source.repoURL}')`)
	add(`    got=$(k -n "$ns" get application "$1" -o jsonpath='{.status.sync.revision}')`)
	add(`    [ "$url" = "oci://$addr/space/$2" ] && [ "$got" = "$want" ] && break`)
	add(`    sleep 5`)
	add(`  done`)
	add(`  [ "$url" = "oci://$addr/space/$2" ] || { echo "  $1 reads $url, not its Space $2" >&2; return 1; }`)
	add(`  [ "$got" = "$want" ] || { echo "  $1 synced ${got:-nothing it reports}, not $2's newest release $want" >&2; return 1; }`)
	add(`  uid=$(k -n "$ns" get application "$1" -o jsonpath='{.metadata.uid}')`)
	add(`  [ -z "$was" ] || [ "$was" = new ] || [ "$was" = unknown ] || [ "$uid" = "$was" ] || { echo "  $1 is a new object (UID $uid, was $was): it was recreated, not moved" >&2; return 1; }`)
	add(`  local same="UID unchanged"; [ "$was" = new ] && same="new, for a cluster that joined"; [ "$was" = unknown ] && same="moved before"`)
	add(`  echo "  $1 reads $2 at $got ($(k -n "$ns" get application "$1" -o jsonpath='{.status.sync.status}/{.status.health.status}'), $same)"`)
	add(`}`)
	add("")

	add("# settle <application> <space> <control space>: once it reads its Space, its")
	add("# Unit drops Replace=true. The replace took off what the template set on the")
	add("# source; left on, every sync of the parent would replace the Application")
	add("# again and erase its status and history.")
	add(`settle() {`)
	add(`  cub argo application-unit --settled --kube-context "$ctx" --namespace "$ns" --application "$1" --space "$2" --gateway "$addr" > "render/app-$2.yaml"`)
	add(`  cub unit update --space "$3" "$2" "render/app-$2.yaml" --change-desc "$1 reads $2: stop replacing it on every sync" --quiet`)
	add(`}`)
	add("# ready <space>...: every Space in a stage has a release, checked before anything")
	add("# is made, so a stage is moved whole or not at all.")
	add(`ready() {`)
	add(`  local s missing=""`)
	add(`  for s in "$@"; do cub release get --space "$s" --oci-reference latest -o jq=.Release.ManifestDigest >/dev/null 2>&1 || missing="$missing $s"; done`)
	add(`  [ -z "$missing" ] || { echo "  no release yet in:$missing. Release them (apply.sh does, stage by stage), then run this again." >&2; return 1; }`)
	add(`}`)
	add("")
	add(`[ $# -gt 0 ] || set -- %s`, strings.Join(p.Stages, " "))
	add(`for stage in "$@"; do`)
	add(`  case "$stage" in`)
	for _, st := range p.Stages {
		ms := byStage[st]
		add(`  %s)`, q(st))
		add(`    step "Stage %s"`, st)
		if len(ms) == 0 {
			add(`    echo "  nothing an ApplicationSet made runs in %s"`, st)
			add(`    ;;`)
			continue
		}
		touched := map[string]bool{}
		var order []string
		var spaces []string
		for _, m := range ms {
			spaces = append(spaces, q(m.v.Space))
		}
		add(`    ready %s`, strings.Join(spaces, " "))
		for _, m := range ms {
			add(`    deliver %s %s %s`, q(m.v.Application), q(m.v.Space), q(m.home.Space))
			if !touched[m.home.Space] {
				touched[m.home.Space] = true
				order = append(order, m.home.Space)
			}
		}
		for _, s := range order {
			add(`    publish %s %s`, q(s), q(parentOf[s]))
		}
		for _, m := range ms {
			add(`    arrived %s %s`, q(m.v.Application), q(m.v.Space))
		}
		for _, m := range ms {
			add(`    settle %s %s %s`, q(m.v.Application), q(m.v.Space), q(m.home.Space))
		}
		for _, s := range order {
			add(`    publish %s %s`, q(s), q(parentOf[s]))
		}
		for _, w := range p.Windows {
			if w.Kind != "deny" {
				continue
			}
			var held []string
			for _, m := range ms {
				for _, a := range w.Applications {
					if a == m.v.Application {
						held = append(held, a)
					}
				}
			}
			if len(held) > 0 {
				add(`    echo %s`, q(fmt.Sprintf("  note: AppProject %s denies syncing %s on schedule %q for %s; inside that window each reads its Space but applies it only once the window closes",
					w.Project, strings.Join(held, ", "), w.Schedule, w.Duration)))
			}
		}
		add(`    ;;`)
	}
	add(`  *) echo "no stage $stage; the stages are: %s"; exit 1 ;;`, strings.Join(p.Stages, " "))
	add(`  esac`)
	add(`done`)
	add(`finished=1`)
	add("")
	for _, a := range byHand {
		add("echo %s", q(fmt.Sprintf("%s has several sources, so it was left for you: point it at its Space by hand.", a)))
	}
	add("echo")
	add("echo %s", q("Each Application moved is a Unit now, named after its variant's Space. The way"))
	add("echo %s", q("back, for every Application ever moved by this script:"))
	add(`way_back all`)
	return strings.Join(L, "\n") + "\n"
}
