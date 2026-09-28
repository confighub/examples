package flux

import (
	"fmt"
	"strings"
)

// CleanupScript undoes what apply.sh created, in the one order ConfigHub
// accepts. That order is not guessable: a Space refuses to go while a Target,
// a worker, a Release or a Tag still points at it, each refusal names only the
// first blocker, and the Space and its release Target reference each other, so
// the release Target has to be cleared before either can go.
//
// The order below was measured against ConfigHub v0.5.1, by deleting a Space
// made this way and following each refusal. Two things it teaches are worth
// repeating here: a Unit assigned to a Target blocks the Target, and every
// `cub ... list` wraps its rows in the entity name, so a Release id comes from
// .[].Release.ReleaseID and not .[].ReleaseID. A loop over the wrong path
// deletes nothing and says nothing.
//
// This undoes onboarding, not a handover. Once handover.sh has repointed a
// live Kustomization, deleting these Spaces takes its source away; the script
// says so and stops unless it is told the fleet still reads Git.
func CleanupScript(p *Plan, prefix string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	var spaces []string
	for _, c := range p.Components {
		spaces = append(spaces, c.Base)
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				spaces = append(spaces, v.Space)
			}
		}
	}

	add("#!/usr/bin/env bash")
	add("# Undo what apply.sh created in ConfigHub. Written by `cub flux apply`.")
	add("#")
	add("#   bash cleanup.sh")
	add("#")
	add("# This is the way back from onboarding, while Flux is still reading Git and")
	add("# ConfigHub holds a copy nothing reads. It removes only what apply.sh made.")
	add("#")
	add("# It is NOT the way back from a handover. If handover.sh has already")
	add("# repointed a Kustomization at the gateway, deleting these Spaces takes its")
	add("# source away, and a layer with spec.prune true then empties itself.")
	add("set -uo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`gone() { cub space get "$1" >/dev/null 2>&1 && return 1 || return 0; }`)
	add("")
	add(`step "0/7 Check that no Kustomization still reads ConfigHub"`)
	add("# A repointed layer whose Space is deleted has no source at all, and with")
	add("# prune on that empties the cluster. This will not guess; it asks.")
	add(`if [ "${I_HAVE_PUT_THE_SOURCES_BACK:-}" != yes ]; then`)
	add(`  echo "If handover.sh has run, put every layer's sourceRef back to its GitRepository first:"`)
	add(`  echo "  kubectl -n flux-system get kustomizations -o custom-columns=NAME:.metadata.name,KIND:.spec.sourceRef.kind,SOURCE:.spec.sourceRef.name"`)
	add(`  echo "Then re-run with I_HAVE_PUT_THE_SOURCES_BACK=yes bash cleanup.sh"`)
	add(`  echo`)
	add(`  echo "If handover.sh has not run, nothing on any cluster reads these Spaces and"`)
	add(`  echo "you can say so now."`)
	add(`  read -r -p "Has the handover been undone, or never run? [yes/no] " a`)
	add(`  [ "$a" = yes ] || { echo "Stopping. Nothing was deleted."; exit 1; }`)
	add("fi")
	add("")

	// Units first: a Target cannot go while a Unit is assigned to it.
	add(`step "1/7 Units"`)
	for _, s := range spaces {
		add(`for u in $(cub unit list --space %s -o jq='.[].Unit.Slug' 2>/dev/null | tr -d '"'); do cub unit delete --space %s "$u" --quiet 2>/dev/null; done`, s, s)
	}
	add("")

	// A Space and its release Target reference each other, so this comes before
	// either can be deleted.
	add(`step "2/7 Each Space's release Target, which the Space and the Target both hold"`)
	for _, s := range spaces {
		add(`echo '{"ReleaseTargetID":null}' | cub space update --patch %s --from-stdin --quiet 2>/dev/null`, s)
	}
	add("")

	add(`step "3/7 Targets"`)
	for _, c := range p.Clusters {
		add("cub target delete --space %s %s --quiet 2>/dev/null", targets, c.Name)
	}
	add("")

	add(`step "4/7 The server worker"`)
	add("cub worker delete --space %s server-worker --quiet 2>/dev/null", targets)
	add("")

	// A Release is found by its own id, wherever it lives.
	add(`step "5/7 Releases"`)
	for _, s := range spaces {
		add(`for r in $(cub release list --space %s -o jq='.[].Release.ReleaseID' 2>/dev/null | tr -d '"'); do cub release delete "$r" --quiet 2>/dev/null; done`, s)
	}
	add("")

	add(`step "6/7 Tags"`)
	for _, s := range spaces {
		add(`for t in $(cub tag list --space %s -o jq='.[].Tag.Slug' 2>/dev/null | tr -d '"'); do cub tag delete --space %s "$t" --quiet 2>/dev/null; done`, s, s)
	}
	add("")

	add(`step "7/7 Spaces, and the Components they belong to"`)
	for _, s := range append(spaces, targets) {
		add(`cub space delete %s --quiet 2>/dev/null`, s)
	}
	for _, c := range p.Components {
		add("cub component delete %s-%s --quiet 2>/dev/null", prefix, c.Name)
	}
	add("")

	// Saying it is done is not the same as checking, and a delete that was
	// refused printed to a stream this script sends to /dev/null.
	add(`step "Checking, rather than assuming"`)
	add("rest=0")
	for _, s := range append(spaces, targets) {
		add(`gone %s || { echo "  still there: %s"; rest=$((rest+1)); }`, s, s)
	}
	add(`if [ "$rest" -eq 0 ]; then`)
	add(`  echo "  every Space apply.sh made is gone."`)
	add("else")
	add(`  echo`)
	add(`  echo "$rest Space(s) remain. ConfigHub refuses a Space while anything still"`)
	add(`  echo "references it, and names only the first blocker each time. Ask it:"`)
	add(`  echo "  cub space delete <space>     # the message names what to remove next"`)
	add("fi")
	return strings.Join(L, "\n") + "\n"
}
