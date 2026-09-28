package argo

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
// live Application, deleting these Spaces takes its source away; the script
// says so and stops unless it is told the estate still reads Git.
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
	for _, s := range p.controlSpaces(prefix) {
		spaces = append(spaces, s.Space)
	}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				spaces = append(spaces, v.Space)
			}
		}
	}
	for _, c := range p.Components {
		spaces = append(spaces, c.Base)
	}
	spaces = append(spaces, targets)

	add("#!/usr/bin/env bash")
	add("# Undo what apply.sh created in ConfigHub. Written by `cub argo apply`.")
	add("#")
	add("#   bash cleanup.sh")
	add("#")
	add("# This is the way back from onboarding, while Argo CD is still reading Git and")
	add("# ConfigHub holds a copy nothing reads. It removes only what apply.sh made.")
	add("#")
	add("# It is NOT the way back from a handover. If handover.sh has already")
	add("# repointed a Application at the gateway, deleting these Spaces takes its")
	add("# source away.")
	add("set -uo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`gone() { cub space get "$1" >/dev/null 2>&1 && return 1 || return 0; }`)
	add("")
	add(`step "1/3 Check that nothing on a cluster still reads ConfigHub"`)
	add("# A repointed Application whose Space is deleted has no source at all, and")
	add("# with prune on that empties the cluster. This will not guess; it asks.")
	add(`if [ "${I_HAVE_PUT_THE_SOURCES_BACK:-}" != yes ]; then`)
	add(`  echo "If handover.sh has run, put every Application's source back to Git first:"`)
	add(`  echo "  kubectl -n argocd get applications -o custom-columns=NAME:.metadata.name,SOURCE:.spec.source.repoURL"`)
	add(`  echo "Then re-run with I_HAVE_PUT_THE_SOURCES_BACK=yes bash cleanup.sh"`)
	add(`  echo`)
	add(`  echo "If handover.sh has not run, nothing on any cluster reads these Spaces and"`)
	add(`  echo "you can say so now."`)
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
	for _, s := range spaces {
		add(`cub space delete %s --recursive --detach --quiet 2>&1 | grep -v '^$' || true`, s)
	}
	add("")

	add(`step "3/3 The Components those Spaces belonged to"`)
	for _, s := range p.controlSpaces(prefix) {
		add("cub component delete %s --quiet 2>/dev/null", s.Space)
	}
	for _, c := range p.Components {
		add("cub component delete %s-%s --quiet 2>/dev/null", prefix, c.Name)
	}
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
