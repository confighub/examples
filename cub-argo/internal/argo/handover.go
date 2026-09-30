package argo

import (
	"fmt"
	"path/filepath"
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
	add("# with DEST_CONTEXT_<cluster>=<kubectl context> for each cluster Argo deploys")
	add("# to other than its own: step 0 names any that are missing.")
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
	// The context is fixed once, here: a context switched in another terminal
	// mid-run would otherwise move the rest of the handover to another cluster.
	add(`ctx=${ARGOCD_CONTEXT:-$(kubectl config current-context 2>/dev/null || true)}`)
	add(`[ -n "$ctx" ] || { echo "no kubectl context: set ARGOCD_CONTEXT to the cluster Argo CD runs on"; exit 1; }`)
	add(`echo "every kubectl call below uses context $ctx"`)
	add(`k() { kubectl --context "$ctx" "$@"; }`)
	// show prints a kubectl command for a person to run later, naming the
	// context and namespace this run used rather than leaving them to whatever
	// is current when it is pasted.
	show := func(rest string) {
		// <gateway> is filled in once step 1 knows the address.
		add(`printf '  kubectl --context %%s -n %%s %%s\n' "$ctx" "$ns" "$(printf '%%s' %s | sed "s|<gateway>|${addr:-<gateway>}|")"`, q(rest))
	}
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`ns=${ARGOCD_NAMESPACE:-argocd}`)
	add("# The comparison before the template repoint renders from the repository.")
	if repoRel == "" {
		repoRel = "."
	}
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRel))
	if p.inflatesHelm() {
		// Rendered the way apply.sh rendered it; without the flag the
		// comparison fails on the chart before it compares anything.
		add(`KUSTOMIZE_FLAGS=${KUSTOMIZE_FLAGS:---enable-helm}`)
	}
	add("")
	add("# The gateway address a repointed source reads. cub reports it for a Target.")
	add("gateway() { cub target get --space %s \"$1\" -o jq=.Target.Parameters.OCIRepository 2>/dev/null || true; }", targets)
	add("")

	add("# The state of this run: each parent's source as it was, recorded before it")
	add("# moves. If anything stops the script after a parent moved, the way back is")
	add("# printed; nothing is rolled back automatically.")
	add(`state=handover-state/argo.txt; mkdir -p handover-state; touch "$state"; moved=""; finished=0`)
	add("# While a parent still reads Git, what it reads now is its original, and it")
	add("# replaces any earlier record: a handover rolled back and started again may")
	add("# find its source changed. Once it reads the gateway, the record stands, for")
	add("# the way back from an interrupted run.")
	add(`record_source() {`)
	add(`  local was; was=$(k -n "$ns" get application "$1" -o jsonpath='{.metadata.name}{"|"}{.spec.source.repoURL}{"|"}{.spec.source.path}{"|"}{.spec.source.targetRevision}')`)
	add(`  case "$was" in *"|oci://"*) return 0 ;; esac`)
	add(`  grep -v "^$1|" "$state" > "$state.new" || true; mv "$state.new" "$state"`)
	add(`  printf '%%s\n' "$was" >> "$state"`)
	add(`}`)
	add("# way_back: leaves first, then parents. Measured on Argo CD v3.5.3: moving")
	add("# root back alone puts everything back, every UID intact, but restores the Git")
	add("# AppProjects while a generated Application still reads the gateway, which is")
	add("# refused until it moves too. Undoing from the bottom avoids that.")
	add(`way_back() {`)
	add(`  local name url path rev`)
	add(`  [ -s handover-state/applications.txt ] && echo "  0. First the way back move-applications.sh prints, for each Application it made a Unit: restoring an ApplicationSet while its Applications are Units sets it and their parent against each other."`)
	add(`  echo "  1. Any ApplicationSet you retired (the 'Retire each ApplicationSet' step): if it is a Unit, restore that Unit to the revision before create-only and publish its Space; if it was applied by hand, set its spec.syncPolicy.applicationsSync back to what it was on the cluster. Either way the controller then puts its Applications back on the template's Git source."`)
	add(`  echo "  2. Any app of apps repointed through its Unit: restore that Unit to the revision before the repoint, and publish its Space."`)
	add(`  echo "  3. Then each parent patched here:"`)
	add(`  while IFS='|' read -r name url path rev; do`)
	add(`    case " $moved " in *" $name "*) ;; *) [ "${1:-}" = all ] || continue ;; esac`)
	add(`    echo "  kubectl --context '$ctx' -n '$ns' patch application $name --type merge -p '{\"spec\":{\"source\":{\"repoURL\":\"$url\",\"path\":\"$path\",\"targetRevision\":\"$rev\"}}}'"`)
	add(`  done < "$state"`)
	add(`}`)
	add(`stopped() {`)
	add(`  local rc=$?`)
	add(`  [ "$finished" = 1 ] && return`)
	add(`  [ -n "$moved" ] || { echo "Stopped (exit $rc) before any parent's source was changed." >&2; return; }`)
	add(`  echo >&2; echo "HANDOVER STOPPED (exit $rc) on context $ctx. These parents read ConfigHub:$moved" >&2`)
	add(`  echo "Nothing is rolled back automatically. The way back, leaves first:" >&2`)
	add(`  way_back >&2`)
	add(`  echo "Each parent's original source is in $state." >&2`)
	add(`}`)
	add(`trap stopped EXIT`)
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
	// The field comparison reads each Application's objects on the cluster it
	// deploys to. For every cluster other than Argo's own, that takes a context
	// this script cannot guess, so each is asked for by name up front.
	server := map[string]string{}
	for _, c := range p.Clusters {
		server[c.Name] = c.Server
	}
	var remote []string
	seenRemote := map[string]bool{}
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Path == "" || v.Path == "(multi-source)" || v.Cluster == "in-cluster" || seenRemote[v.Cluster] {
					continue
				}
				seenRemote[v.Cluster] = true
				remote = append(remote, v.Cluster)
			}
		}
	}
	if len(remote) > 0 {
		add("# Each Application's objects are read on the cluster it deploys to, so each")
		add("# cluster other than Argo CD's own needs its kubectl context:")
		add(`missing=""`)
		for _, cl := range remote {
			add(`[ -n "${%s:-}" ] || missing="$missing %s"`, destVar(cl), destVar(cl))
		}
		add(`[ -z "$missing" ] || { echo "set the kubectl context of each cluster Argo deploys to:$missing"; exit 1; }`)
	}

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
	// repo-creds, not repository. Argo matches a "repository" Secret to an
	// Application by its url, and these repoURLs carry a /space/<space> path
	// that oci://<host> does not match -- so the credential was ignored, Argo
	// fell back to anonymous https, and it failed with "cannot get digest for
	// revision latest" over a scheme nobody asked for. repo-creds is the
	// prefix form: one credential for every Space under the gateway. Measured
	// on Argo CD v3.5.3.
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
	add("    argocd.argoproj.io/secret-type: repo-creds")
	add("stringData:")
	add("  type: oci")
	add(`  url: oci://${addr}`)
	add(`  username: "$(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '\n')"`, targets)
	add(`  password: "$(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '\n')"`, targets)
	add(`${plain_http}`)
	add("YAML")
	add("")

	cs := p.controlSpaces(prefix)
	// Which control Space holds each parent as a Unit, once its own parent has
	// been repointed. A parent in this map is synced from ConfigHub by the time
	// its turn comes, so it cannot be patched on the cluster.
	ownedBy := map[string]string{}
	parentOf := map[string]string{}
	unitOf := map[string]string{}
	for _, outer := range cs {
		for _, f := range outer.Files {
			unit := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			for _, inner := range cs {
				if inner.Parent == unit || strings.HasPrefix(unit, inner.Parent+"-") {
					ownedBy[inner.Parent] = outer.Space
					parentOf[inner.Parent] = outer.Parent
					// The Unit is named for the file, which is not always the
					// Application's own name: storefront lives in
					// storefront-app-of-apps.yaml.
					unitOf[inner.Parent] = unit
				}
			}
		}
	}
	step := 2
	for _, s := range cs {
		add(`step "%d/5 Repoint %s at %s"`, step, s.Parent, s.Space)
		add("# Its children are Units there by now, released to the argocd Target.")
		// A parent that is itself a child of an already-repointed parent is no
		// longer ours to patch: the grandparent syncs it from ConfigHub with
		// prune on, so a kubectl patch here is applied and then reverted on the
		// next reconcile -- measured, and it looks like it worked for about a
		// minute. The change has to be made to the Unit that defines it.
		if ownedBy[s.Parent] != "" {
			add(`echo "%s is a Unit in %s now, which %s syncs from ConfigHub."`, s.Parent, ownedBy[s.Parent], parentOf[s.Parent])
			add(`echo "Patching it here would be undone on the next reconcile. Change it where it"`)
			add(`echo "is defined, then let it flow down:"`)
			add(`echo "  cub unit data --space %s %s > %s.yaml"`, ownedBy[s.Parent], unitOf[s.Parent], unitOf[s.Parent])
			add(`echo "  # set spec.source.repoURL to oci://${addr}/space/%s, path '.', targetRevision latest"`, s.Space)
			add(`echo "  cub unit update --space %s %s %s.yaml"`, ownedBy[s.Parent], unitOf[s.Parent], unitOf[s.Parent])
			add(`echo "  cub release publish %s"`, ownedBy[s.Parent])
			add(`echo "That is a reviewed change, which is the point of it being a Unit."`)
			step++
			continue
		}
		// addr was resolved once in step 1, from CONFIGHUB_OCI or the Target.
		// Re-reading it here overwrote a good CONFIGHUB_OCI with the Target's
		// empty answer, and the repoint went in as oci:///space/<space> -- a
		// URL with no host, which Argo rejects as "not permitted in project"
		// rather than as malformed.
		// Its source is recorded before it moves, once, so a re-run cannot
		// overwrite the original with the half-moved state; the way back
		// restores exactly this.
		add(`record_source %s`, q(s.Parent))
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
		add(`moved="$moved %s"`, s.Parent)
		add(`k -n "$ns" annotate application %s argocd.argoproj.io/refresh=hard --overwrite`, s.Parent)
		// Not --for=Synced. A parent reports OutOfSync while any child still
		// differs, and during a handover its children are exactly what is being
		// moved -- so waiting for Synced here waits for something that cannot
		// happen yet, and times out on a repoint that worked. What matters is
		// that Argo could READ the new source: sync.status leaves Unknown.
		add(`for _ in $(seq 1 36); do`)
		add(`  st=$(k -n "$ns" get application %s -o jsonpath='{.status.sync.status}' 2>/dev/null)`, s.Parent)
		add(`  [ -n "$st" ] && [ "$st" != Unknown ] && break`)
		add(`  sleep 5`)
		add(`done`)
		add(`st=$(k -n "$ns" get application %s -o jsonpath='{.status.sync.status}')`, s.Parent)
		add(`if [ "$st" = Unknown ] || [ -z "$st" ]; then`)
		add(`  echo "%s could not read %s:" >&2`, s.Parent, s.Space)
		add(`  k -n "$ns" get application %s -o jsonpath='{.status.conditions[*].message}' >&2; echo >&2`, s.Parent)
		add(`  exit 1`)
		add("fi")
		// The digest Argo synced is the release it read: it has to be the one
		// ConfigHub holds as the newest, or a release went out unchecked.
		// Argo reconciles after the patch in its own time, and its status can
		// still name the Git revision it synced before: poll for the checked
		// digest rather than read once.
		add(`want=$(cub release get --space %s --oci-reference latest -o jq=.Release.ManifestDigest | tr -d '"')`, s.Space)
		add(`got=""; for _ in $(seq 1 36); do`)
		add(`  got=$(k -n "$ns" get application %s -o jsonpath='{.status.sync.revision}')`, s.Parent)
		add(`  [ "$got" = "$want" ] && break`)
		add(`  sleep 5`)
		add(`done`)
		add(`if [ "$got" != "$want" ]; then`)
		add(`  echo "  %s synced ${got:-nothing it reports}, not %s's newest release $want" >&2; exit 1`, s.Parent, s.Space)
		add(`fi`)
		add(`echo "  %s reads %s at $got ($st; a parent reads OutOfSync until its children move too)"`, s.Parent, s.Space)
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
				dest := ""
				if v.Cluster != "in-cluster" {
					// The plan knows which Argo destination this cluster is, so
					// the context is bound to it rather than trusted by name.
					dest = fmt.Sprintf(` --destination-context "$%s" --destination %s`, destVar(v.Cluster), q(server[v.Cluster]))
				}
				// Bound to one release by digest, so the check says which bytes
				// it passed, and the repoint below can be compared with them.
				add(`%s=$(cub release get --space %s --oci-reference latest -o jq=.Release.ManifestDigest | tr -d '"')`, digestVar(v.Space), q(v.Space))
				add(`cub argo check --kube-context "$ctx" --fields --namespace "$ns" --application %s --space %s --unit %s --destination-namespace %s%s --release "$%s"`,
					q(v.Application), q(v.Space), q(c.Name), q(v.Namespace), dest, digestVar(v.Space))
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
				// Kubernetes puts kube-root-ca.crt in every namespace, so it is
				// never something a handover leaves behind.
				add(`  cub scout map list -q %s || true`, q("owner=Native AND namespace="+v.Namespace+" AND name!=kube-root-ca.crt"))
			}
		}
	}
	add("else")
	add(`  echo "  cub-scout is not installed; skipping the unclaimed-resource cross-check"`)
	add("fi")
	add("")
	step++

	add(`step "%d/5 Retire each ApplicationSet, then point its Applications at their Spaces"`, step)
	add("# An ApplicationSet generates one Application per cluster from ONE shared")
	add("# template, so it cannot give each cluster its own Space by editing that")
	add("# template with a literal. ConfigHub already holds the fleet enumerated --")
	add("# one variant Space per cluster, staged -- so the generator has nothing left")
	add("# to generate. It is retired, and each Application it made is pointed at its")
	add("# own Space: a literal address, one cluster at a time, which is what makes a")
	add("# staged rollout possible at all.")
	add("#")
	add("# Measured on Argo CD v3.5.3: patching a generated Application while its")
	add("# ApplicationSet is live is reverted in under a second, so the generator has")
	add("# to stand down FIRST. applicationsSync: create-only is what stands it down;")
	add("# the controller then creates but never updates or deletes what it made.")
	homes := p.unitHomes(prefix)
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		home, known := homes["ApplicationSet/"+c.Source]
		add("")
		add("echo %s", q(fmt.Sprintf("-- %s", c.Source)))
		if !known {
			add("echo %s", q(fmt.Sprintf(
				"ApplicationSet %s is not a Unit in any control Space, so it is still yours to edit on the cluster. Set spec.syncPolicy.applicationsSync to create-only before the patches below.", c.Source)))
		} else {
			add("echo %s", q(fmt.Sprintf(
				"%s is a Unit in %s. Retire it where ConfigHub keeps it, so the change is reviewed and survives the next sync:", c.Source, home.Space)))
			add("echo %s", q(fmt.Sprintf("  cub unit data --space %s %s > %s.yaml", home.Space, home.Unit, home.Unit)))
			add("echo %s", q("  # set spec.syncPolicy.applicationsSync: create-only"))
			add("echo %s", q("  #   and spec.generators: [{list: {elements: []}}], so it generates nothing:"))
			add("echo %s", q("  #   create-only never deletes, and one that still generates takes each"))
			add("echo %s", q("  #   Application back once it stands alone (measured on Argo CD v3.5.3)"))
			add("echo %s", q(fmt.Sprintf("  #   and annotate it %s: \"retired at handover; see cleanup.sh\"", retiredAnnotation)))
			add("echo %s", q(fmt.Sprintf("  cub unit update --space %s %s %s.yaml", home.Space, home.Unit, home.Unit)))
			add("echo %s", q(fmt.Sprintf("  cub release publish %s", home.Space)))
			add("echo %s", q("  # then wait for the parent to sync it down before patching anything:"))
			show(fmt.Sprintf("get applicationset %s -o jsonpath='{.spec.syncPolicy.applicationsSync}'", c.Source))
		}
		if known {
			add("echo %s", q("Once it reads create-only, deliver each Application from its own Space, one stage at a time:"))
			add("echo %s", q("  bash move-applications.sh <stage>"))
			add("echo %s", q("It makes each one a Unit in "+home.Space+", reading its Space, and checks it arrived. By hand, it is:"))
		} else {
			add("echo %s", q("Once it reads create-only, point each Application at its own Space:"))
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				// A multi-source Application has no /spec/source to replace:
				// its sources go, and one source takes their place.
				ops := `{"op":"replace","path":"/spec/source","value":{"repoURL":"oci://<gateway>/space/%s","path":".","targetRevision":"latest"}}`
				if v.Path == "(multi-source)" {
					ops = `{"op":"remove","path":"/spec/sources"},{"op":"add","path":"/spec/source","value":{"repoURL":"oci://<gateway>/space/%s","path":".","targetRevision":"latest"}}`
				}
				show(fmt.Sprintf("patch application %s --type json -p '["+ops+"]'   # stage %s",
					v.Application, v.Space, st.Name))
				if v.Path != "" && v.Path != "(multi-source)" {
					add(`echo "    # checked at ${%s:-?}; once synced, its status.sync.revision should name that digest, or a newer release went out unchecked"`, digestVar(v.Space))
				}
			}
		}
	}
	add("")
	add("echo %s", q("Each Application keeps its name and its UID, so nothing is orphaned and no"))
	add("echo %s", q("workload restarts. The retired ApplicationSets stay in place, inert. Removing"))
	add("echo %s", q("them is a separate, later decision: cleanup.sh has the one safe way."))

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
	show("get applications -o custom-columns=NAME:.metadata.name,SOURCE:.spec.source.repoURL,SYNC:.status.sync.status")
	add(`finished=1`)
	add("echo %s", q("Nothing was deleted. The way back, leaves first:"))
	add(`way_back all`)
	add("echo")
	add("echo %s", q("One more thing, measured on Argo CD v3.5.3: publishing a new release does NOT"))
	add("echo %s", q("reach the cluster on its own. Argo caches the digest it resolved for the tag, and"))
	add("echo %s", q("was still serving the previous release 90 seconds later. A hard refresh re-resolves"))
	add("echo %s", q("the tag to a digest and the release lands:"))
	show("annotate application <name> argocd.argoproj.io/refresh=hard --overwrite")
	add("echo %s", q("argobot does this for you: argobot.sh runs it beside Argo CD with the Targets' worker,"))
	add("echo %s", q("so it refreshes each Application reading a Space when that Space publishes, and writes"))
	add("echo %s", q("each Application's live status back to its Space. Without argobot, keep this running"))
	add("echo %s", q("beside the cluster instead: it asks for that refresh once per release, and writes the"))
	add("echo %s", q("same live status, which is what the Healthy gate and change orders read:"))
	add(`echo "  cub argo status <what you planned, with the same flags> --prefix %s --kube-context $ctx --watch --hard-refresh"`, prefix)
	return strings.Join(L, "\n") + "\n"
}

// retiredAnnotation marks an ApplicationSet that a handover has stood down. It
// goes on the object, so it is visible both in ConfigHub and to whoever next
// reads the cluster and wonders why a generator is generating nothing.
const retiredAnnotation = "argo.confighub.com/retired"

// destVar is the environment variable naming the kubectl context of a cluster
// Argo deploys to.
func destVar(cluster string) string {
	return "DEST_CONTEXT_" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, cluster)
}

// digestVar is the shell variable holding the release digest a variant was
// checked at.
func digestVar(space string) string {
	return "digest_" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, space)
}
