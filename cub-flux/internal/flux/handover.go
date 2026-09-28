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
	add(`k() { kubectl ${FLUX_CONTEXT:+--context "$FLUX_CONTEXT"} "$@"; }`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`ns=${FLUX_NAMESPACE:-flux-system}`)
	add(`cluster=${CLUSTER:-}`)
	add("# The comparison below re-renders from the repository, so it needs it.")
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRelOr(repoRel)))
	add(`[ -n "$cluster" ] || { echo "set CLUSTER=<one of: %s> so the right Target is used"; exit 1; }`, clusterNames(p))
	add("")

	add(`step "0/4 Check before changing anything"`)
	// cub auth status, not a list call: a list goes through the entity API and
	// fails on a client/server version skew while the session is fine. See the
	// note in ApplyScript.
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	add(`k get namespace "$ns" >/dev/null || { echo "no $ns namespace: is this the right cluster?"; exit 1; }`)
	add("# flux-system reconciles the Flux controllers themselves. It is the way back")
	add("# if anything here goes wrong, so it stays on Git and is never repointed.")
	for _, b := range p.Boundary {
		add("echo %s", q("Left alone: "+b))
	}
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
				add(`[ "$cluster" = %s ] && cub flux check ${FLUX_CONTEXT:+--kube-context "$FLUX_CONTEXT"} --namespace %s --kustomization %s --space %s --unit %s%s`,
					q(v.Cluster), q(ns), q(name), q(v.Space), q(c.Name), target)
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
	add(`mkdir -p bootstrap`)
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
		add(`  *) echo "no %s variant for cluster $cluster"; exit 1 ;;`, c.Name)
		add(`esac`)
		add(`cat > bootstrap/ocirepository-%s.yaml <<YAML`, c.Name)
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
	add(`k apply -f bootstrap/`)
	add(`echo "Commit bootstrap/ into the flux-system path so these survive a reconcile."`)
	add("")

	add(`step "3/4 Swap each layer's sourceRef, deepest dependency first"`)
	add("# The Kustomization keeps its name, so Flux keeps the inventory of what it")
	add("# applied, and nothing is recreated.")
	for _, step := range p.Order {
		for _, name := range step {
			add(`echo "-- %s"`, name)
			// spec.path moves with the source. It is a path inside the
			// artifact, and a Git artifact is the repository tree while a
			// ConfigHub artifact is the rendered manifests at its root. Leaving
			// the Git path behind fails with, measured on Flux v2.8.6:
			//   kustomization path not found: stat /tmp/kustomization-.../<git path>
			// which reads as a missing directory rather than as the one field
			// the swap forgot, and the layer never becomes Ready.
			add(`k -n "$ns" patch kustomization %s --type merge -p '{"spec":{"sourceRef":{"kind":"OCIRepository","name":"%s"},"path":"./"}}'`, name, name)
			add(`k -n "$ns" wait --for=condition=Ready kustomization/%s --timeout=5m`, name)
		}
	}
	add("")

	add(`step "4/4 What is left, and what it means"`)
	for _, a := range p.Automation {
		name := strings.Fields(a)[1]
		add(`k -n "$ns" patch imageupdateautomation %s --type merge -p '{"spec":{"suspend":true}}' 2>/dev/null || true`, name)
		add("echo %s", q(fmt.Sprintf(
			"Suspended ImageUpdateAutomation %s: it committed tags to Git, which no longer feeds this cluster. A tag bump is now a change on the base, promoted like any other.", name)))
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
	add("echo")
	add("echo %s", q("Done once every layer is Ready from an OCIRepository:"))
	add("echo %s", q("  kubectl -n $ns get kustomizations -o custom-columns=NAME:.metadata.name,SOURCE:.spec.sourceRef.kind,READY:.status.conditions[0].status"))
	// The way back restores two fields, not one. Saying only "sourceRef" here
	// would leave a reader with a layer pointing at Git and a path of "./",
	// which finds no kustomization and is a worse place than they started.
	add("echo %s", q("Nothing was deleted. To go back, restore BOTH fields on each layer:"))
	src := "<your GitRepository>"
	if len(p.Sources) > 0 {
		src = p.Sources[0].Name
	}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				ns, name := "flux-system", v.Kustomization
				if i := strings.Index(name, "/"); i >= 0 {
					ns, name = name[:i], name[i+1:]
				}
				add(`[ "$cluster" = %s ] && echo %s`, v.Cluster,
					q(fmt.Sprintf(`  kubectl -n %s patch kustomization %s --type merge -p '{"spec":{"sourceRef":{"kind":"GitRepository","name":"%s"},"path":"%s"}}'`,
						ns, name, src, v.Path)))
			}
		}
	}
	// The last command decides the script's exit status, and a [ ] that is
	// false would make a wholly successful handover exit 1.
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
