package argo

import (
	"fmt"
	"strings"
)

// HandoverScript writes the step that moves the estate: each layer's source is
// repointed at ConfigHub, top down, and nothing is deleted. Run it only after
// apply.sh, because a parent pointed at a Space that holds nothing is a parent
// syncing an empty source, and it prunes what it applied.
func HandoverScript(p *Plan, prefix, repoRel string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	add("#!/usr/bin/env bash")
	add("# Move this Argo CD estate onto ConfigHub. Written by `cub argo apply`.")
	add("# Read it, then run it:")
	add("#")
	add("#   ARGOCD_CONTEXT=<kubectl context of the cluster Argo CD runs on> bash handover.sh")
	add("#")
	add("# Run apply.sh first. Every step here is a patch, never a delete: the")
	add("# objects, their names and Argo's tracking of what they own all survive, so")
	add("# no workload is recreated. It goes top down, because a parent pointed at a")
	add("# Space that holds nothing prunes the children it applied.")
	add("#")
	add("# Rehearsed on Argo CD v3.5.3: a repoint preserved every UID, including the")
	add("# Pod's, with the rollout revision unchanged. Your estate is not that one.")
	add("# Read every step before running it, and start with one non-production")
	add("# estate.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`k() { kubectl ${ARGOCD_CONTEXT:+--context "$ARGOCD_CONTEXT"} "$@"; }`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`ns=${ARGOCD_NAMESPACE:-argocd}`)
	add("# The comparison before the template repoint renders from the repository.")
	if repoRel == "" {
		repoRel = "."
	}
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRel))
	add("")
	add("# The gateway address a repointed source reads. cub reports it for a Target.")
	add("gateway() { cub target get --space %s \"$1\" -o jq=.Target.Parameters.OCIRepository 2>/dev/null || true; }", targets)
	add("")

	add(`step "0/5 Check before changing anything"`)
	// cub auth status, not a list call: a list goes through the entity API and
	// fails on a client/server version skew while the session is fine. See the
	// note in ApplyScript.
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	add(`image=$(k get deployment argocd-repo-server -n "$ns" -o jsonpath='{.spec.template.spec.containers[0].image}')`)
	add(`version=${image##*:}; version=${version#v}`)
	add(`if [ "$(printf '%%s\n' 3.1 "${version%%.*}.${version#*.}" | sort -V | head -1)" != 3.1 ]; then`)
	add(`  echo "Argo CD runs $version; an oci:// source is read natively from v3.1, so this handover is not available here"; exit 1`)
	add("fi")

	// The projects gate every repoint, so they are checked before anything moves.
	var blocked []string
	for _, line := range p.Handover {
		if strings.HasPrefix(line, "first, add the gateway's oci:// address to sourceRepos") {
			blocked = append(blocked, line)
		}
	}
	if len(blocked) > 0 {
		add("# Until the gateway is allowed as a source, every repoint below is refused.")
		// Read from the cluster, not from the plan. The plan's reading comes
		// from the repository, so it cannot tell that the operator has already
		// widened sourceRepos -- and then this stops a handover that would have
		// worked, every time it is run.
		add(`still_blocked=""`)
		for _, name := range p.RestrictedProjects {
			add(`k -n "$ns" get appproject %s -o jsonpath='{.spec.sourceRepos}' 2>/dev/null | grep -q -e 'oci://' -e '"\*"' || still_blocked="$still_blocked %s"`, name, name)
		}
		add(`if [ -n "$still_blocked" ]; then`)
		add("echo %s", q(blocked[0]))
		// Not "cub target get": apply.sh creates the Target with empty parameters
		// and cub reports no gateway address for it, so that command prints
		// nothing and reads as a broken install rather than as the wrong query.
		add(`echo "  The address is oci://<gateway host>, the same CONFIGHUB_OCI this script takes:"`)
		add(`echo "    ConfigHub cloud:  oci://oci.hub.confighub.com"`)
		add(`echo "    self-hosted:      oci:// plus the host and port of confighub-oci-server,"`)
		add(`echo "                      as reachable FROM this cluster, not from your laptop."`)
		// Measured on Argo CD v3.5.3: these AppProjects are themselves synced by
		// the root Application with selfHeal on, so a kubectl patch of
		// sourceRepos is reverted within minutes and the repoint is refused
		// again with no sign of why. The change has to go where Argo reads it.
		add(`echo`)
		add(`echo "Change it in GIT, not with kubectl: these AppProjects are synced by an"`)
		add(`echo "Application with selfHeal on, so a patch here is undone within minutes."`)
		add(`echo "Commit the oci:// entry to the file that defines them, let Argo sync it,"`)
		add(`echo "then re-run this script. It re-reads the cluster and will carry on."`)
		add(`read -r -p "Press Return once sourceRepos allows it, or Ctrl-C to stop. " _ || true`)
		add("else")
		add(`  echo "sourceRepos already allows an oci:// source on every AppProject this needs."`)
		add("fi")
	}
	add("")

	add(`step "1/5 The credential Argo reads the gateway with"`)
	add("# A repository Secret for the gateway, holding the Targets' server worker.")
	add("# The ID and secret go from cub into the Secret through file descriptors,")
	add("# never to disk, the command line, or the terminal.")
	add(`addr=${CONFIGHUB_OCI:-$(gateway argocd)}`)
	add(`if [ -z "$addr" ]; then`)
	add(`  echo "Could not read the gateway address from Target %s/argocd."`, targets)
	add(`  echo "Find it with: cub target get --space %s argocd"`, targets)
	add(`  echo "then re-run with CONFIGHUB_OCI=<address> bash handover.sh"; exit 1`)
	add("fi")
	add("# The repository Secret's shape was verified against Argo CD v3.5.3:")
	add("#   type: oci, url with the oci:// scheme, the worker as username and")
	add("#   password. The Application's repoURL needs the scheme too — without it")
	add("#   Argo treats the address as a git repository and fails on \"list refs\".")
	add("# insecureOCIForceHttp is only for a gateway served over plain HTTP.")
	// Not ${VAR:+  insecureOCIForceHttp: "true"} inline below: the alternate
	// word goes through quote removal, so the quotes are gone by the time the
	// heredoc sees it and the API server rejects the Secret with
	//   cannot unmarshal bool into Go struct field Secret.stringData
	// A variable's value is not re-quoted, so this form keeps them.
	add(`plain_http=""`)
	add(`[ -n "${CONFIGHUB_OCI_PLAIN_HTTP:-}" ] && plain_http='  insecureOCIForceHttp: "true"'`)
	add(`cat <<YAML | k apply -f -`)
	add("apiVersion: v1")
	add("kind: Secret")
	add("metadata:")
	add("  name: confighub-%s", targets)
	add(`  namespace: ${ns}`)
	add("  labels:")
	add("    argocd.argoproj.io/secret-type: repository")
	add("stringData:")
	add("  type: oci")
	add(`  url: oci://${addr}`)
	add(`  username: "$(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '\n')"`, targets)
	add(`  password: "$(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '\n')"`, targets)
	add(`${plain_http}`)
	add("YAML")
	add("")

	cs := p.controlSpaces(prefix)
	step := 2
	for _, s := range cs {
		add(`step "%d/5 Repoint %s at %s"`, step, s.Parent, s.Space)
		add("# Its children are Units there by now, released to the argocd Target.")
		// addr was resolved once in step 1, from CONFIGHUB_OCI or the Target.
		// Re-reading it here overwrote a good CONFIGHUB_OCI with the Target's
		// empty answer, and the repoint went in as oci:///space/<space> -- a
		// URL with no host, which Argo rejects as "not permitted in project"
		// rather than as malformed.
		add(`k -n "$ns" get application %s -o jsonpath='{.spec.source.repoURL}' | grep -q '^oci://' && echo %s || \`,
			s.Parent, q(s.Parent+" already reads ConfigHub"))
		// The gateway serves one repository per Space, at /space/<space>, so the
		// Space is part of the URL and not part of the host. Two parents are
		// repointed at two different Spaces here: one address without the Space
		// could only ever be right for one of them.
		add(`  k -n "$ns" patch application %s --type merge -p "{\"spec\":{\"source\":{\"repoURL\":\"oci://${addr}/space/%s\",\"path\":\".\",\"targetRevision\":\"latest\"}}}"`, s.Parent, s.Space)
		add("# Argo caches the digest it resolved for a tag, so a repoint alone can")
		add("# leave it serving the release it read before. A hard refresh re-resolves")
		add("# the tag, and is what argobot issues on every release.published.")
		add(`k -n "$ns" annotate application %s argocd.argoproj.io/refresh=hard --overwrite`, s.Parent)
		add(`k -n "$ns" wait --for=jsonpath='{.status.sync.status}'=Synced application/%s --timeout=3m`, s.Parent)
		step++
	}

	// Every Application prunes, so what a Target holds has to match what the
	// overlay renders today before its source is moved to that Target.
	add(`step "%d/5 Prove nothing on the clusters would change"`, step)
	add("# Two questions, and only the second can see the cluster.")
	add("#")
	add("# Does what ConfigHub holds equal what the overlay renders today? That")
	add("# compares two things both derived from Git, so it catches Git moving")
	add("# since apply.sh, and nothing else.")
	add(`command -v kustomize >/dev/null || { echo "kustomize is not on PATH; this step re-renders with it"; exit 1; }`)
	add(`[ -d "$REPO_ROOT" ] || { echo "REPO_ROOT=$REPO_ROOT is not a directory: point it at the repository checkout"; exit 1; }`)
	add(`echo "  rendering with $(kustomize version)"`)
	add(`if ! git -C "$REPO_ROOT" diff --quiet 2>/dev/null; then`)
	add(`  echo "  note: $REPO_ROOT has uncommitted changes, so this renders something Argo is not applying" >&2`)
	add("fi")
	add("same() {")
	add("  local fresh got rc=0")
	add(`  fresh=$(mktemp); got=$(mktemp)`)
	add(`  kustomize build ${KUSTOMIZE_FLAGS:-} "$REPO_ROOT/$3" > "$fresh" || { rm -f "$fresh" "$got"; return 1; }`)
	add(`  cub unit data --space "$1" "$2" > "$got" || { rm -f "$fresh" "$got"; return 1; }`)
	add(`  if diff -q <(grep -v '^\s*#' "$fresh") <(grep -v '^\s*#' "$got") >/dev/null; then`)
	add(`    echo "  $1 holds what $3 renders today"`)
	add("  else")
	add(`    echo "  $1 DIFFERS from what $3 renders today. Re-run apply.sh, then read the diff." >&2`)
	add(`    diff -u <(grep -v '^\s*#' "$fresh") <(grep -v '^\s*#' "$got") | head -40 >&2`)
	add("    rc=1")
	add("  fi")
	add(`  rm -f "$fresh" "$got"; return $rc`)
	add("}")
	add("")
	add("# And the one that matters: does what ConfigHub would deliver equal what")
	add("# Argo owns on the cluster right now? Argo's own status.resources is the")
	add("# record, so this sees objects that are on the cluster and not in Git at")
	add("# all. Where Argo prunes, those would be deleted the moment the source")
	add("# moves. No render-side check can see them.")
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Path == "" || v.Path == "(multi-source)" {
					continue
				}
				add("same %s %s %s", q(v.Space), q(c.Name), q(v.Path))
				add(`cub argo check ${ARGOCD_CONTEXT:+--kube-context "$ARGOCD_CONTEXT"} --fields --namespace "$ns" --application %s --space %s --unit %s --destination-namespace %s`,
					q(v.Application), q(v.Space), q(c.Name), q(v.Namespace))
			}
		}
	}
	add("")
	add("# What no controller claims on these namespaces, as a cross-check. This")
	add("# is cub-scout inferring ownership, not a gate: it finds what was applied")
	add("# by hand and would be left behind, which neither record above reports.")
	add(`if command -v cub-scout >/dev/null || cub scout --help >/dev/null 2>&1; then`)
	seen := map[string]bool{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Namespace == "" || seen[v.Namespace] {
					continue
				}
				seen[v.Namespace] = true
				add(`  cub scout map list -q %s || true`, q("owner=Native AND namespace="+v.Namespace))
			}
		}
	}
	add("else")
	add(`  echo "  cub-scout is not installed; skipping the unclaimed-resource cross-check"`)
	add("fi")
	add("")
	step++

	add(`step "%d/5 Point each ApplicationSet's template at its clusters' Targets"`, step)
	add("# The ApplicationSet goes on generating the same Applications under the same")
	add("# names, so Argo's tracking does not change and nothing is orphaned. Each")
	add("# generated Application reads its own cluster's Target.")
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		add("# %s generates:", c.Source)
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add("#   %-28s reads Space %s", v.Application, v.Space)
			}
		}
		// Naming the Target here was wrong: the gateway addresses a Space, not a
		// Target, and an ApplicationSet template needs a per-cluster Space. The
		// generator's own cluster value is what selects it.
		add("echo %s", q(fmt.Sprintf(
			"Edit the %s Unit's template: set spec.template.spec.source.repoURL to oci://<the gateway>/space/<this cluster's variant Space, listed above>, path '.', targetRevision latest. Promote it like any other change.",
			c.Source)))
	}
	for _, w := range p.Windows {
		if w.Kind != "deny" || len(w.Applications) == 0 {
			continue
		}
		add("echo %s", q(fmt.Sprintf(
			"Note: AppProject %s denies syncing %s on schedule %q for %s. The repoint lands, but its first sync waits for the window to close.",
			w.Project, strings.Join(w.Applications, ", "), w.Schedule, w.Duration)))
	}
	add("")
	add("echo")
	add("echo %s", q("Done once every Application reports Synced from an oci:// source:"))
	add("echo %s", q("  kubectl -n $ns get applications -o custom-columns=NAME:.metadata.name,SOURCE:.spec.source.repoURL,SYNC:.status.sync.status"))
	add("echo %s", q("Nothing was deleted. To go back, patch each source to its Git repoURL and path."))
	add("echo")
	add("echo %s", q("One more thing, measured on Argo CD v3.5.3: publishing a new release does NOT"))
	add("echo %s", q("reach the cluster on its own. Argo caches the digest it resolved for the tag, and"))
	add("echo %s", q("was still serving the previous release 90 seconds later. A hard refresh re-resolves"))
	add("echo %s", q("the tag to a digest and the release lands:"))
	add("echo %s", q("  kubectl -n $ns annotate application <name> argocd.argoproj.io/refresh=hard --overwrite"))
	add("echo %s", q("argobot does this for you, reacting to ConfigHub's release.published event. Without"))
	add("echo %s", q("it, or without that annotation, an approved release sits unread on the gateway."))
	return strings.Join(L, "\n") + "\n"
}
