package flux

import (
	"fmt"
	"strings"
)

// HandoverScript moves the fleet: each layer's Kustomization keeps its name
// and its inventory, and only its sourceRef changes, from the fleet
// GitRepository to an OCIRepository on the ConfigHub gateway.
//
// flux-system is never repointed. It reconciles gotk-components.yaml, which is
// the Flux controllers themselves, so putting an approval gate in front of it
// would put one between an operator and a Flux upgrade, and a broken
// source-controller would then be the thing that has to fix itself. It stays
// on Git as the recovery path, and the objects this handover needs before any
// layer can move go into it.
func HandoverScript(p *Plan, prefix, repoRel string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }
	// A layer is touched only on the clusters that have it. image-automation,
	// for one, runs on dev alone, and a Kustomization a cluster does not have
	// would stop the run with every earlier layer already moved.
	on := map[string][]string{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				name := v.Kustomization[strings.Index(v.Kustomization, "/")+1:]
				on[name] = append(on[name], v.Cluster)
			}
		}
	}
	onlyWhereLayer := func(name, line string) {
		if cs := on[name]; len(cs) > 0 && len(cs) < len(p.Clusters) {
			add(`case "$cluster" in %s) %s ;; esac`, strings.Join(cs, "|"), line)
		} else {
			L = append(L, line)
		}
	}

	add("#!/usr/bin/env bash")
	add("# Move this Flux fleet onto ConfigHub. Written by `cub flux apply`.")
	add("# Read it, then run it, one cluster at a time:")
	add("#")
	add("#   FLUX_CONTEXT=<kubectl context of one cluster> bash handover.sh")
	add("#")
	add("# Run apply.sh first. Each layer keeps its Kustomization, its name and its")
	add("# inventory; only the source changes, so nothing is reinstalled.")
	add("#")
	add("# Rehearsed end to end on Flux v2.8.6 against ConfigHub v0.6.2: every UID")
	add("# survived, including the Pod's, and the rollout revision did not change.")
	add("# Your fleet is not that one. Read every step before running it, and start")
	add("# with one non-production cluster.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	// The context is fixed once, here, and every kubectl call and every printed
	// recovery command names it. Left to kubectl, a context switched in another
	// terminal mid-run would move the rest of the handover to another cluster,
	// and a printed rollback would be run against whichever one is current.
	add(`ctx=${FLUX_CONTEXT:-$(kubectl config current-context 2>/dev/null || true)}`)
	add(`[ -n "$ctx" ] || { echo "no kubectl context: set FLUX_CONTEXT to the cluster to hand over"; exit 1; }`)
	add(`k() { kubectl --context "$ctx" "$@"; }`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`ns=${FLUX_NAMESPACE:-flux-system}`)
	add(`cluster=${CLUSTER:-}`)
	add("# The comparison below re-renders from the repository, so it needs it.")
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRelOr(repoRel)))
	add(`case "$cluster" in %s) ;; *) echo "set CLUSTER=<one of: %s> so the right Target is used"; exit 1 ;; esac`,
		strings.ReplaceAll(clusterNames(p), ", ", "|"), clusterNames(p))
	add("")

	add(`step "0/4 Check before changing anything"`)
	// cub auth status, not a list call: a list goes through the entity API and
	// fails on a client/server version skew while the session is fine. See the
	// note in ApplyScript.
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	add(`echo "  every kubectl call below uses context $ctx"`)
	add(`k get namespace "$ns" >/dev/null || { echo "no $ns namespace: is this the right cluster?"; exit 1; }`)
	add("# flux-system reconciles the Flux controllers themselves. It is the way back")
	add("# if anything here goes wrong, so it stays on Git and is never repointed.")
	for _, b := range p.Boundary {
		add("echo %s", q("Left alone: "+b))
	}
	// A layer a Kustomization applies belongs to that Kustomization. In a fleet
	// made with flux bootstrap, flux-system applies clusters/<name>/, which holds
	// every layer, and a kubectl patch of a layer lasts only until flux-system
	// reconciles. Measured on Flux v2.8.6: a patched sourceRef went back to the
	// Git value, and a patched field Git never sets was removed, on the next
	// reconcile. So the patch below would move a layer and then quietly move it
	// back. The change has to go where that owner reads, which is Git.
	add("# A layer another Kustomization applies cannot be moved with kubectl: its")
	add("# owner puts it back on its next reconcile. Find any before changing anything.")
	raw := func(s string) { L = append(L, s) }
	raw(`owned=""`)
	raw(`owner_of() { k -n "$ns" get kustomization "$1" -o jsonpath='{.metadata.labels.kustomize\.toolkit\.fluxcd\.io/name}' 2>/dev/null || true; }`)
	for _, step := range p.Order {
		for _, name := range step {
			onlyWhereLayer(name, fmt.Sprintf(`o=$(owner_of %s); [ -z "$o" ] || owned="$owned %s(applied-by-$o)"`, q(name), name))
		}
	}
	raw(`if [ -n "$owned" ]; then`)
	raw(`  echo "These layers are applied by another Kustomization:$owned" >&2`)
	raw(`  echo "A kubectl patch of them is undone when that Kustomization reconciles, so this" >&2`)
	raw(`  echo "script changes nothing. In a fleet made with flux bootstrap, hand them over in" >&2`)
	add(`  case "$cluster" in`)
	for _, c := range p.Clusters {
		add(`    %s) dir=%s ;;`, c.Name, q(c.Path))
	}
	add(`  esac`)
	raw(`  echo "Git instead: in the file under $dir/ that defines each layer, set" >&2`)
	raw(`  echo "  sourceRef: {kind: OCIRepository, name: <layer>}  and  path: ./" >&2`)
	raw(`  echo "and commit that together with step 2's bootstrap/$cluster/ files." >&2`)
	raw(`  exit 1`)
	raw(`fi`)
	add("")

	add(`step "1/4 Prove each layer holds what Git renders today"`)
	add("# Every layer prunes, so anything a release does not hold is deleted from")
	add("# the cluster. This renders from the repository now rather than trusting")
	add("# what apply.sh left in render/: Git may have moved since, and a stale")
	add("# comparison would pass while the cluster applies something else.")
	add(`command -v kustomize >/dev/null || { echo "kustomize is not on PATH; this step re-renders with it"; exit 1; }`)
	add(`[ -d "$REPO_ROOT" ] || { echo "REPO_ROOT=$REPO_ROOT is not a directory: point it at the repository checkout"; exit 1; }`)
	add(`echo "  rendering with $(kustomize version)"`)
	add(`if ! git -C "$REPO_ROOT" diff --quiet 2>/dev/null; then`)
	add(`  echo "  note: $REPO_ROOT has uncommitted changes, so this renders something Flux is not applying" >&2`)
	add("fi")
	add("# same <space> <unit> <path>")
	add("same() {")
	add(`  local fresh got rc=0`)
	add(`  fresh=$(mktemp); got=$(mktemp)`)
	add(`  kustomize build ${KUSTOMIZE_FLAGS:-} "$REPO_ROOT/$3" > "$fresh" || { rm -f "$fresh" "$got"; return 1; }`)
	add(`  cub unit data --space "$1" "$2" > "$got" || { rm -f "$fresh" "$got"; return 1; }`)
	add(`  if diff -q <(grep -v '^\s*#' "$fresh") <(grep -v '^\s*#' "$got") >/dev/null; then`)
	add(`    echo "  $1 holds what $3 renders today"`)
	add("  else")
	add(`    echo "  $1 DIFFERS from what $3 renders today. Swapping its source would prune the difference." >&2`)
	add(`    echo "  Re-run apply.sh to bring ConfigHub up to date, then read the diff before swapping." >&2`)
	add(`    diff -u <(grep -v '^\s*#' "$fresh") <(grep -v '^\s*#' "$got") | head -40 >&2`)
	add("    rc=1")
	add("  fi")
	add(`  rm -f "$fresh" "$got"; return $rc`)
	add("}")
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add(`[ "$cluster" = %s ] && same %s %s %s`, q(v.Cluster), q(v.Space), q(c.Name), q(v.Path))
			}
		}
	}
	add("")
	add("# And the question the comparison above cannot reach: does what ConfigHub")
	add("# would deliver equal what this layer has actually applied? Flux's own")
	add("# status.inventory is the record. Every layer here prunes, so an object it")
	add("# applied that the release does not hold is deleted the moment the source")
	add("# is swapped, whether or not it was ever in Git.")
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				ns, _, _ := strings.Cut(v.Kustomization, "/")
				name := v.Kustomization[strings.Index(v.Kustomization, "/")+1:]
				target := ""
				if v.TargetNamespace != "" {
					target = " --target-namespace " + q(v.TargetNamespace)
				}
				// The check is bound to one release, named by digest, and the
				// swap below confirms Flux fetched that same one before it moves
				// the layer: a release published in between would otherwise go
				// out unchecked.
				add(`if [ "$cluster" = %s ]; then`, q(v.Cluster))
				add(`  digest_%s=$(cub release get --space %s --oci-reference latest -o jq=.Release.ManifestDigest | tr -d '"')`, shellName(name), q(v.Space))
				add(`  cub flux check --kube-context "$ctx" --namespace %s --kustomization %s --space %s --unit %s%s --release "$digest_%s"`,
					q(ns), q(name), q(v.Space), q(c.Name), target, shellName(name))
				add(`fi`)
			}
		}
	}
	add("")
	add("# What no controller claims in this fleet, as a cross-check. cub-scout")
	add("# infers ownership where the inventory is Flux's own record, so this is")
	add("# not a gate: it finds what was applied by hand and would be left behind.")
	add(`if cub scout --help >/dev/null 2>&1; then`)
	add(`  cub scout map list -q "owner=Native" || true`)
	add("else")
	add(`  echo "  cub-scout is not installed; skipping the unclaimed-resource cross-check"`)
	add("fi")
	add("")

	add(`step "2/4 Add the gateway to flux-system: the credential and one OCIRepository per layer"`)
	add("# These cannot come from ConfigHub: a Kustomization cannot read a source that")
	add("# does not exist yet. They are committed to the bootstrap directory, which is")
	add("# the one part of this fleet that stays on Git.")
	add(`addr=${CONFIGHUB_OCI:-}`)
	add(`if [ -z "$addr" ]; then`)
	// The Target does not carry the gateway's host: apply.sh creates it with
	// empty parameters, and cub reports none. So this asks rather than guessing,
	// and says where the answer comes from for both kinds of installation.
	add(`  echo "Set CONFIGHUB_OCI to the gateway host this cluster reaches, without a scheme."`)
	add(`  echo "  ConfigHub cloud:  oci.hub.confighub.com"`)
	add(`  echo "  self-hosted:      the host and port of the confighub-oci-server service,"`)
	add(`  echo "                    as reachable FROM this cluster, not from your laptop."`)
	add(`  echo "Then: CONFIGHUB_OCI=<host> CLUSTER=$cluster bash handover.sh"; exit 1`)
	add("fi")
	// A docker-registry Secret, not a generic one. Measured against Flux
	// v2.8.6: an OCIRepository reads its secretRef as a dockerconfigjson, and
	// an Opaque Secret carrying username and password -- which is what a
	// GitRepository takes -- leaves it failing with
	//   failed to determine artifact digest: ... 401 Unauthorized
	// while the very same credentials work by hand. The error names the
	// registry, so it reads like a bad password rather than a wrong Secret
	// shape, which is what makes this worth a comment.
	add("# The ID and secret go from cub into the Secret through command substitution")
	add("# and are never written to disk. --dry-run | apply keeps this re-runnable.")
	add(`k create secret docker-registry confighub-%s --namespace "$ns" \`, targets)
	add(`  --docker-server="$addr" \`)
	add(`  --docker-username="$(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '\"\n')" \`, targets)
	add(`  --docker-password="$(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '\"\n')" \`, targets)
	add(`  --dry-run=client -o yaml | k apply -f -`)
	add("")
	add("# One OCIRepository per layer, each reading that layer's release for this")
	add("# cluster. Commit these to the bootstrap directory so flux-system keeps them.")
	add(`mkdir -p "bootstrap/$cluster"`)
	// The gateway serves one repository per Space, at /space/<space>. A layer's
	// Space differs per cluster, so each OCIRepository has to name the variant
	// Space for the cluster this run is handing over -- an address without it
	// is not a repository the gateway has, and the layer never becomes Ready.
	for _, c := range p.Components {
		v := shellName(c.Name)
		add(`case "$cluster" in`)
		for _, st := range c.Stages {
			for _, va := range st.Variants {
				add(`  %s) space_%s='%s' ;;`, va.Cluster, v, va.Space)
			}
		}
		// A cluster without this layer needs no source for it.
		add(`  *) space_%s='' ;;`, v)
		add(`esac`)
		add(`[ -z "$space_%s" ] || cat > "bootstrap/$cluster/ocirepository-%s.yaml" <<YAML`, v, c.Name)
		add("apiVersion: source.toolkit.fluxcd.io/v1")
		add("kind: OCIRepository")
		add("metadata:")
		add("  name: %s", c.Name)
		add(`  namespace: ${ns}`)
		add("spec:")
		add("  interval: 1m")
		add(`  url: oci://${addr}/space/${space_%s}`, v)
		add("  ref:")
		add("    tag: latest")
		add("  secretRef:")
		add("    name: confighub-%s", targets)
		add("YAML")
	}
	add(`k apply -f "bootstrap/$cluster/"`)
	add(`echo "Commit bootstrap/$cluster/ into this cluster's flux-system path so these survive a reconcile."`)
	add("")

	add(`step "3/4 Swap each layer's sourceRef, deepest dependency first"`)
	add("# The Kustomization keeps its name, so Flux keeps the inventory of what it")
	add("# applied, and nothing is recreated.")
	add("#")
	add("# Before a layer changes, its sourceRef and path are recorded, once: a re-run")
	add("# must not overwrite the original with a half-moved state. If anything below")
	add("# stops the script, the layers already moved are named, with the commands")
	add("# that put them back. Nothing is rolled back automatically.")
	raw(`state="handover-state/$cluster.txt"; log="handover-state/$cluster.log"`)
	raw(`mkdir -p handover-state; touch "$state"`)
	raw(`log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >> "$log"; }`)
	raw(`moved=""; current=""; finished=0; incomplete=""; held=""`)
	raw(`# record <layer>: name|kind|source|source namespace|path, as the cluster had it`)
	raw(`record() {`)
	raw(`  grep -q "^$1|" "$state" && return 0`)
	raw(`  local was`)
	raw(`  was=$(k -n "$ns" get kustomization "$1" -o jsonpath='{.metadata.name}{"|"}{.spec.sourceRef.kind}{"|"}{.spec.sourceRef.name}{"|"}{.spec.sourceRef.namespace}{"|"}{.spec.path}')`)
	raw(`  case "$was" in`)
	raw(`    *"|OCIRepository|"*) echo "  $1 already reads an OCIRepository, and no original was recorded for it here" >&2 ;;`)
	raw(`    *) printf '%s\n' "$was" >> "$state"; log "recorded $was" ;;`)
	raw(`  esac`)
	raw(`}`)
	raw(`# way_back [all]: the commands that restore BOTH fields, for the layers this`)
	raw(`# run moved, or with "all" for every layer recorded for this cluster.`)
	raw(`way_back() {`)
	raw(`  local name kind src srcns path ref`)
	raw(`  while IFS='|' read -r name kind src srcns path; do`)
	raw(`    case " $moved " in *" $name "*) ;; *) [ "${1:-}" = all ] || continue ;; esac`)
	raw(`    ref="\"kind\":\"$kind\",\"name\":\"$src\""`)
	raw(`    [ -z "$srcns" ] || ref="$ref,\"namespace\":\"$srcns\""`)
	raw(`    echo "  kubectl --context '$ctx' -n '$ns' patch kustomization $name --type merge -p '{\"spec\":{\"sourceRef\":{$ref},\"path\":\"$path\"}}'"`)
	raw(`  done < "$state"`)
	raw(`}`)
	raw(`stopped() {`)
	raw(`  local rc=$?`)
	raw(`  [ "$finished" = 1 ] && return`)
	raw(`  echo >&2`)
	raw(`  if [ -z "$moved" ]; then`)
	raw(`    echo "Stopped (exit $rc) before any layer's source was changed." >&2`)
	raw(`    return`)
	raw(`  fi`)
	raw(`  log "stopped at ${current:-?}, exit $rc"`)
	raw(`  echo "HANDOVER STOPPED at ${current:-?} (exit $rc), on context $ctx." >&2`)
	raw(`  echo "These layers were moved and now read ConfigHub:$moved" >&2`)
	raw(`  echo "Nothing is rolled back automatically. To put them back on Git:" >&2`)
	raw(`  way_back >&2`)
	raw(`  echo "Each layer's original is in $state; what happened is in $log." >&2`)
	raw(`  echo "The gateway Secret and OCIRepositories from step 2 stay; they feed nothing unless a layer reads them." >&2`)
	raw(`}`)
	raw(`trap stopped EXIT`)
	raw(`# swap <layer>`)
	raw(`swap() {`)
	raw(`  current=$1`)
	raw(`  echo "-- $1"`)
	raw(`  record "$1"`)
	raw(`  local var want fetched`)
	raw(`  var="digest_$(printf '%s' "$1" | tr -- '-./' '___')"; want=${!var:-}`)
	raw(`  [ -n "$want" ] || { echo "  $1: no checked release digest was recorded for it" >&2; return 1; }`)
	raw(`  k -n "$ns" wait --for=condition=Ready "ocirepository/$1" --timeout=2m`)
	raw(`  fetched=$(k -n "$ns" get ocirepository "$1" -o jsonpath='{.status.artifact.revision}')`)
	raw(`  if [ "${fetched##*@}" != "$want" ]; then`)
	raw(`    log "NOT moved $1: checked $want, OCIRepository fetched $fetched"`)
	raw(`    echo "  $1: the release was checked at $want, but Flux fetched $fetched. A release was published since; re-run to check that one." >&2`)
	raw(`    return 1`)
	raw(`  fi`)
	// spec.path moves with the source. It is a path inside the artifact, and a
	// Git artifact is the repository tree while a ConfigHub artifact is the
	// rendered manifests at its root. Leaving the Git path behind fails with,
	// measured on Flux v2.8.6:
	//   kustomization path not found: stat /tmp/kustomization-.../<git path>
	// which reads as a missing directory rather than as the one field the swap
	// forgot, and the layer never becomes Ready.
	raw(`  k -n "$ns" patch kustomization "$1" --type merge -p "{\"spec\":{\"sourceRef\":{\"kind\":\"OCIRepository\",\"name\":\"$1\"},\"path\":\"./\"}}"`)
	raw(`  moved="$moved $1"; log "patched $1"`)
	raw(`  if ! k -n "$ns" wait --for=condition=Ready "kustomization/$1" --timeout=5m; then`)
	raw(`    log "NOT Ready $1: $(k -n "$ns" get kustomization "$1" -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}' 2>&1 || true)"`)
	raw(`    return 1`)
	raw(`  fi`)
	raw(`  log "Ready $1 at $(k -n "$ns" get kustomization "$1" -o jsonpath='{.status.lastAppliedRevision}')"`)
	raw(`}`)
	for _, step := range p.Order {
		for _, name := range step {
			onlyWhereLayer(name, "swap "+q(name))
		}
	}
	raw(`current=""`)
	add("")

	add(`step "4/4 What is left, and what it means"`)
	if len(p.Automation) > 0 {
		add("# A patch is not a suspension until the cluster says so: read it back. And")
		add("# an object a Kustomization applies from Git is that Kustomization's, so a")
		add("# patch on the cluster may not outlast its next reconcile.")
		raw(`suspend_automation() {`)
		raw(`  local got owner where`)
		raw(`  if ! got=$(k -n "$ns" get imageupdateautomation "$1" -o name 2>&1); then`)
		raw(`    case "$got" in`)
		raw(`      *NotFound*|*"not found"*) echo "  ImageUpdateAutomation $1 is not on this cluster; nothing to suspend."; return 0 ;;`)
		raw(`    esac`)
		raw(`    echo "  ImageUpdateAutomation $1 could not be read, so it was NOT suspended: $got" >&2`)
		raw(`    incomplete="$incomplete $1"; log "NOT suspended $1: unreadable: $got"; return 0`)
		raw(`  fi`)
		raw(`  if ! k -n "$ns" patch imageupdateautomation "$1" --type merge -p '{"spec":{"suspend":true}}'; then`)
		raw(`    echo "  ImageUpdateAutomation $1 was NOT suspended: the patch failed. It still commits tags to Git." >&2`)
		raw(`    incomplete="$incomplete $1"; log "NOT suspended $1: patch failed"; return 0`)
		raw(`  fi`)
		raw(`  got=$(k -n "$ns" get imageupdateautomation "$1" -o jsonpath='{.spec.suspend}' 2>&1) || got="unreadable ($got)"`)
		raw(`  if [ "$got" != true ]; then`)
		raw(`    echo "  ImageUpdateAutomation $1 was NOT suspended: spec.suspend reads $got after the patch." >&2`)
		raw(`    incomplete="$incomplete $1"; log "NOT suspended $1: spec.suspend=$got"; return 0`)
		raw(`  fi`)
		raw(`  log "suspended $1"`)
		raw(`  owner=$(k -n "$ns" get imageupdateautomation "$1" -o jsonpath='{.metadata.labels.kustomize\.toolkit\.fluxcd\.io/name}' 2>/dev/null || true)`)
		raw(`  if [ -n "$owner" ]; then`)
		raw(`    case " $moved " in`)
		raw(`      *" $owner "*) where="its ConfigHub unit, which $owner now reads" ;;`)
		raw(`      *) where="Git, which $owner still reads" ;;`)
		raw(`    esac`)
		raw(`    echo "  ImageUpdateAutomation $1 reads suspended now, but Kustomization $owner applies it and will undo this when it next reconciles. To make it last, set spec.suspend: true in $where."`)
		raw(`    held="$held $1"`)
		raw(`  else`)
		raw(`    echo "  Suspended ImageUpdateAutomation $1: spec.suspend reads true."`)
		raw(`  fi`)
		raw(`}`)
		for _, a := range p.Automation {
			name := strings.Fields(a)[1]
			// Only where the plan found it running: elsewhere there is nothing
			// to suspend, and a failed patch there is not a failed handover.
			if _, on, ok := strings.Cut(a, ", running on "); ok {
				add(`case "$cluster" in %s) suspend_automation %s ;; esac`, strings.ReplaceAll(on, ", ", "|"), q(name))
			} else {
				add("suspend_automation %s", q(name))
			}
		}
		add("echo %s", q("Image automation committed tags to Git, which no longer feeds this cluster. A tag bump is now a change on the base, promoted like any other."))
	}
	for _, t := range p.Tenants {
		add("echo %s", q("Tenant left as it is: "+t+
			". Repointing it would move that team from Git access to ConfigHub access, which is a decision about delegation, not a step in a handover."))
	}
	for _, s := range p.Sources {
		if len(s.Branches) < 2 {
			continue
		}
		seen := map[string]bool{}
		for _, br := range s.Branches {
			seen[br] = true
		}
		if len(seen) > 1 {
			add("echo %s", q(fmt.Sprintf(
				"GitRepository %s promoted by branch. That is now the workflow's stages: a change is promoted and approved per cluster, not merged between branches.", s.Name)))
		}
	}
	add("")
	raw(`finished=1`)
	add("echo")
	add("echo %s", q("Every layer is Ready from an OCIRepository. To see them:"))
	raw(`echo "  kubectl --context '$ctx' -n $ns get kustomizations -o custom-columns=NAME:.metadata.name,SOURCE:.spec.sourceRef.kind,READY:.status.conditions[0].status"`)
	// The way back restores two fields, not one, and restores them to what the
	// cluster had rather than to what the plan guessed: layers can read
	// different GitRepositories, and only the cluster knows which.
	add("echo %s", q("Nothing was deleted. To go back, restore BOTH fields on each layer, as recorded before it moved:"))
	raw(`way_back all`)
	raw(`if [ -n "$held" ]; then`)
	raw(`  echo "Suspended on the cluster only, until Git says so too:$held"`)
	raw(`fi`)
	raw(`if [ -n "$incomplete" ]; then`)
	raw(`  echo "HANDOVER INCOMPLETE: every layer moved, but this image automation is still running:$incomplete" >&2`)
	raw(`  exit 1`)
	raw(`fi`)
	add("true")
	return strings.Join(L, "\n") + "\n"
}

func clusterNames(p *Plan) string {
	var n []string
	for _, c := range p.Clusters {
		n = append(n, c.Name)
	}
	return strings.Join(n, ", ")
}

func repoRelOr(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}

// shellName is a component name usable as a shell variable. Component names
// carry hyphens; shell variable names cannot.
func shellName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '.' || r == '/' {
			return '_'
		}
		return r
	}, s)
}
