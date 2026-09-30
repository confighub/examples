package flux

import (
	"fmt"
	"strings"
)

// CleanupScript undoes what apply.sh created.
//
// This was once a seven-step ordering: Units, then each Space's release Target,
// then Targets, the worker, Releases, Tags, and only then the Spaces -- because
// ConfigHub refuses a Space while anything still references it and names only
// the first blocker each time. That order was measured against ConfigHub
// v0.5.1, and on v0.6.2 it deleted nothing at all: Attestations are a kind of
// content it did not know to remove, so every Space stayed.
//
// `cub space delete --recursive --detach` does the whole job in one call and
// cannot fall behind a new entity type the way a hand-written order does.
// --recursive removes the Space's contents; --detach clears the references
// other entities hold to it, which is what the Space and its release Target
// each holding the other needed. Measured on v0.6.2: seven Spaces, seven calls,
// all gone.
//
// This undoes onboarding, not a handover. Once handover.sh has repointed a
// live Kustomization, deleting these Spaces takes its source away; the script
// says so and stops unless it is told the fleet still reads Git.
func CleanupScript(p *Plan, prefix string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	// Order matters, in one way --detach does not cover: a variant's Release
	// carries a TagID pointing at a Tag in the base Space, so a base deleted
	// first is refused with
	//   Release 1 in Space <variant>: TagID references Tag <base>/<tag>
	// Variants first, then the bases they were promoted from, then the Targets
	// Space. Measured on v0.6.2: base-first left two Spaces behind.
	var spaces []string
	// The layers Spaces first: each is released to a Target in the Targets
	// Space, which cannot go while anything still releases to it.
	for _, cl := range p.Clusters {
		spaces = append(spaces, DeliverySpace(prefix, cl.Name))
	}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				spaces = append(spaces, v.Space)
			}
		}
	}
	// A handed-over cluster has no variants in this plan, but its variant
	// Spaces are still in ConfigHub, and their releases hold tags in the bases:
	// left behind, they stop the bases being deleted. They are named the way
	// every variant is; the ones a cluster never had are skipped.
	var maybe []string
	for _, cl := range p.Clusters {
		if cl.LayersSpace == "" {
			continue
		}
		for _, c := range p.Components {
			maybe = append(maybe, fmt.Sprintf("%s-%s-%s", prefix, c.Name, cl.Name))
		}
	}
	spaces = append(spaces, maybe...)
	for _, c := range p.Components {
		spaces = append(spaces, c.Base)
	}
	spaces = append(spaces, targets)

	add("#!/usr/bin/env bash")
	add("# Undo what apply.sh created in ConfigHub. Written by `cub flux apply`.")
	add("#")
	add("#   bash cleanup.sh")
	add("#")
	add("# This is the way back from onboarding, while Flux is still reading Git and")
	add("# ConfigHub holds a copy nothing reads. It removes only what apply.sh made.")
	add("#")
	add("# It is NOT the way back from a handover or a join. Once a cluster's root reads")
	add("# its layers Space, deleting these Spaces takes every layer's source away.")
	add("set -uo pipefail")
	// Every kubectl command printed below names a context: the one given, or a
	// placeholder that has to be filled in, never whichever happens to be current.
	add(`ctxflag="--context ${FLUX_CONTEXT:-<the cluster>}"`)
	add(`cd "$(dirname "$0")"`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`gone() { cub space get "$1" >/dev/null 2>&1 && return 1 || return 0; }`)
	add("")
	add(`step "1/3 Check that nothing on a cluster still reads ConfigHub"`)
	add("# A repointed layer whose Space is deleted has no source at all, and")
	add("# with prune on that empties the cluster. Given the clusters' contexts")
	add("# (FLUX_CONTEXT, or FLUX_CONTEXTS for several), it reads each one's")
	add("# OCIRepositories and refuses; without them it will not guess, and asks.")
	add(`contexts="${FLUX_CONTEXTS:-${FLUX_CONTEXT:-}}"`)
	add(`if [ -n "$contexts" ]; then`)
	add(`  for c in $contexts; do`)
	add(`    reading=$(kubectl --context "$c" get ocirepositories -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name} {.spec.url}{"\n"}{end}') || { echo "Could not read the OCIRepositories on $c. Nothing was deleted."; exit 1; }`)
	add(`    still=$(printf '%%s\n' "$reading" | grep -E '/space/%s-' | cut -d' ' -f1 | tr '\n' ' ')`, prefix)
	add(`    if [ -n "$still" ]; then`)
	add(`      echo "On $c, these still read a Space this would delete: $still"`)
	add(`      echo "Hand the layers back to Git first (handover.sh printed the way back), or remove a joined cluster's root with pruning off. Nothing was deleted."; exit 1`)
	add(`    fi`)
	add(`    echo "  nothing on $c reads these Spaces"`)
	add(`  done`)
	add(`elif [ "${I_HAVE_PUT_THE_SOURCES_BACK:-}" != yes ]; then`)
	add(`  echo "If handover.sh has run, put every layer's sourceRef back to its GitRepository first:"`)
	add(`  echo "  kubectl $ctxflag -n flux-system get kustomizations -o custom-columns=NAME:.metadata.name,KIND:.spec.sourceRef.kind,PATH:.spec.path"`)
	add(`  echo "  Both sourceRef and path have to go back: handover.sh printed the exact commands."`)
	add(`  echo "Then re-run with I_HAVE_PUT_THE_SOURCES_BACK=yes bash cleanup.sh"`)
	add(`  echo`)
	add(`  echo "A cluster that joined with join.sh reads ConfigHub from the start: remove its"`)
	add(`  echo "root first, with pruning off so the layers stay, or leave these Spaces alone:"`)
	add(`  echo "  kubectl $ctxflag -n flux-system patch kustomization confighub-root --type merge -p '{\"spec\":{\"suspend\":true,\"prune\":false}}'"`)
	add(`  echo`)
	add(`  echo "If handover.sh and join.sh have not run, nothing on any cluster reads these"`)
	add(`  echo "Spaces and you can say so now."`)
	add(`  read -r -p "Has the handover been undone, or never run? [yes/no] " a`)
	add(`  [ "$a" = yes ] || { echo "Stopping. Nothing was deleted."; exit 1; }`)
	add("fi")
	add("")

	// One call per Space. --recursive takes the Space's contents with it, so a
	// new kind of content needs no new step here; --detach clears references
	// held from outside, which is what a Space and its release Target each
	// holding the other needs.
	add(`step "2/3 Delete each Space with everything in it"`)
	add("# --recursive stops at a delete gate. If one blocks a Space, read what it")
	add("# guards before reaching for --recursive-force.")
	skipMissing := map[string]bool{}
	for _, m := range maybe {
		skipMissing[m] = true
	}
	for _, s := range spaces {
		if skipMissing[s] {
			add(`cub space get %s >/dev/null 2>&1 && { cub space delete %s --recursive --detach --quiet 2>&1 | grep -v '^$' || true; }`, s, s)
			continue
		}
		add(`cub space delete %s --recursive --detach --quiet 2>&1 | grep -v '^$' || true`, s)
	}
	add("")

	add(`step "3/3 The Components those Spaces belonged to, and the credential"`)
	for _, c := range p.Components {
		add("cub component delete %s-%s --quiet 2>/dev/null", prefix, c.Name)
	}
	add("# The pull Secret handover.sh or join.sh wrote holds the worker deleted")
	add("# above; left behind, a later onboarding under this prefix would read with it.")
	add(`if [ -n "$contexts" ]; then`)
	add(`  for c in $contexts; do`)
	add(`    kubectl --context "$c" -n "${FLUX_NAMESPACE:-flux-system}" delete secret confighub-%s --ignore-not-found`, targets)
	add(`  done`)
	add("else")
	add(`  echo "  on each cluster handed over or joined, remove the pull Secret: kubectl --context <cluster> -n flux-system delete secret confighub-%s --ignore-not-found"`, targets)
	add("fi")
	add("")

	// Saying it is done is not the same as checking. The seven-step version of
	// this script deleted nothing on v0.6.2 and only this told us.
	add(`step "Checking, rather than assuming"`)
	add("rest=0")
	for _, s := range spaces {
		add(`gone %s || { echo "  still there: %s"; rest=$((rest+1)); }`, s, s)
	}
	add(`if [ "$rest" -eq 0 ]; then`)
	add(`  echo "  every Space apply.sh made is gone."`)
	add("else")
	add(`  echo`)
	add(`  echo "$rest Space(s) remain. Ask ConfigHub what still blocks each one:"`)
	add(`  echo "  cub space delete <space> --recursive     # the message names the blocker"`)
	add("fi")
	return strings.Join(L, "\n") + "\n"
}
