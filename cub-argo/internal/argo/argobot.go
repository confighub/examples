package argo

import (
	"fmt"
	"strings"
)

// ArgobotVersion is the argobot release argobot.sh installs unless told
// otherwise. It reports status for a moved estate, but its refresh looks for
// an Application named after the Space, which a moved estate does not have;
// confighub/argobot#14 finds them by source, and is not in a release yet.
const ArgobotVersion = "v0.1.7"

// argobotBySource is the first release that refreshes by source; "" until
// there is one.
const argobotBySource = ""

// ArgobotScript writes argobot.sh, which runs argobot beside Argo CD with the
// Targets' server worker as its identity, the one `cub cluster up` gives it.
// argobot is what reports each Application's live status back to the Space it
// reads, and what makes a published release land at once.
func ArgobotScript(prefix string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }
	add("#!/usr/bin/env bash")
	add("# Run argobot beside Argo CD. Written by `cub argo apply`. Read it, then run it:")
	add("#")
	add("#   ARGOCD_CONTEXT=<kubectl context of the cluster Argo CD runs on> \\")
	add("#   CONFIGHUB_URL=<ConfigHub's address, as that cluster reaches it> bash argobot.sh")
	add("#")
	add("# argobot connects to ConfigHub as %s/server-worker, the worker every", targets)
	add("# Target here names, so it acts for exactly these Targets. It does two things:")
	add("#")
	add("# - When a variant's Space publishes a release, it hard-refreshes the")
	add("#   Applications reading that Space. Argo CD caches the digest a tag resolved")
	add("#   to, so without this a release waits for Argo's own poll.")
	add("# - It writes each Application's live state (sync, health, operation, the")
	add("#   revision it synced) back to the Space it reads, as confighub.com/live-status,")
	add("#   which the Healthy gate and the ConfigHub UI read.")
	add("#")
	add("# Both find an Application by the Space its source reads. For the refresh, that")
	add("# needs an argobot with confighub/argobot#14; an earlier one looks for an")
	add("# Application named after the Space, which a moved estate does not have, and")
	add("# reports status only.")
	add("set -euo pipefail")
	add(`ctx=${ARGOCD_CONTEXT:-$(kubectl config current-context 2>/dev/null || true)}`)
	add(`[ -n "$ctx" ] || { echo "no kubectl context: set ARGOCD_CONTEXT to the cluster Argo CD runs on"; exit 1; }`)
	add(`echo "every kubectl call below uses context $ctx"`)
	add(`k() { kubectl --context "$ctx" "$@"; }`)
	add(`url=${CONFIGHUB_URL:-}`)
	add(`[ -n "$url" ] || { echo "set CONFIGHUB_URL to ConfigHub's address as the cluster reaches it, e.g. https://hub.confighub.com"; exit 1; }`)
	add(`version=${ARGOBOT_VERSION:-%s}`, ArgobotVersion)
	add(`ns=${ARGOCD_NAMESPACE:-argocd}`)
	add(`[ "$ns" = argocd ] || echo "note: argobot's manifest grants it a Role in the argocd namespace; give it the same in $ns"`)
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	add("")
	add("# Its Namespace, ServiceAccount, RBAC and Deployment, as argobot publishes them.")
	add(`k apply -f "https://raw.githubusercontent.com/confighub/argobot/$version/manifests/argobot.yaml"`)
	add("# Its credential: the worker's ID and secret go from cub into the Secret through")
	add("# the heredoc, never to disk or the command line.")
	add(`cat <<YAML | k apply -f -`)
	add("apiVersion: v1")
	add("kind: Secret")
	add("metadata:")
	add("  name: argobot-secrets")
	add("  namespace: argobot")
	add("stringData:")
	add(`  CONFIGHUB_WORKER_ID: "$(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '"\n')"`, targets)
	add(`  CONFIGHUB_WORKER_SECRET: "$(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '"\n')"`, targets)
	add("YAML")
	add(`k -n argobot set env deployment/argobot CONFIGHUB_URL="$url" ARGO_NAMESPACE="$ns"`)
	add(`k -n argobot set image deployment/argobot argobot="ghcr.io/confighub/argobot:$version"`)
	add(`k -n argobot rollout status deployment/argobot --timeout=5m`)
	if argobotBySource == "" {
		add("# No argobot release refreshes by source yet (confighub/argobot#14). Set")
		add("# ARGOBOT_VERSION to one that does once there is one.")
		add(`[ -n "${ARGOBOT_VERSION:-}" ] || { echo; echo "note: argobot $version reports each Application's live status, but does NOT refresh these"; echo "Applications on a release: they keep Argo's names, and it looks for one named after the Space."; echo "Until a release has confighub/argobot#14, a published release waits for Argo's own poll, or:"; echo "  kubectl --context $ctx -n $ns annotate application <name> argocd.argoproj.io/refresh=hard --overwrite"; }`)
	}
	add("")
	add(`echo "argobot $version runs as %s/server-worker. What it does:"`, targets)
	add(`echo "  kubectl --context $ctx -n argobot logs deploy/argobot"`)
	add(`echo "Each Space an Application reads carries its live status once it syncs:"`)
	add(`echo "  cub space get <variant Space> -o json | jq -r '.Space.Annotations[\"confighub.com/live-status\"]'"`)
	return strings.Join(L, "\n") + "\n"
}
