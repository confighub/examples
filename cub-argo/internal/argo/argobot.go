package argo

import (
	"fmt"
	"strings"
)

// ArgobotVersion is the argobot release argobot.sh installs unless told
// otherwise. It has to be one that finds its Targets by grant: a Target no
// longer names a worker, and an argobot that asks for Targets by worker is
// refused by the server and exits. It also finds a moved estate's Applications
// by the Space their source reads, since they keep Argo's names.
const ArgobotVersion = "v0.1.9"

// argobotBySource is the first release that refreshes by source. An install of
// an earlier one, through ARGOBOT_VERSION, reports status only.
const argobotBySource = "v0.1.9"

// ArgobotScript writes argobot.sh, which runs argobot beside Argo CD with the
// Targets' server worker as its identity, the one `cub cluster up` gives it.
// argobot is what records each Application's live status on the Release it
// synced, and what makes a published release land at once.
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
	add("# - It records each Application's live state (sync, health, operation) on the")
	add("#   Release the Application synced, which the Healthy gate and the ConfigHub")
	add("#   UI read.")
	add("#")
	add("# Both find an Application by the Space its source reads, which needs argobot")
	add("# %s or later; an earlier one looks for an Application named after the Space,", argobotBySource)
	add("# which a moved estate does not have.")
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
	add("# the heredoc, never to disk or the command line; read first, so a failed read")
	add("# stops here rather than installing an empty credential.")
	add(`id=$(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '"\n')`, targets)
	add(`secret=$(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '"\n')`, targets)
	add(`[ -n "$id" ] && [ -n "$secret" ] || { echo "could not read the credentials of %s/server-worker"; exit 1; }`, targets)
	add(`cat <<YAML | k apply -f -`)
	add("apiVersion: v1")
	add("kind: Secret")
	add("metadata:")
	add("  name: argobot-secrets")
	add("  namespace: argobot")
	add("stringData:")
	add(`  CONFIGHUB_WORKER_ID: "$id"`)
	add(`  CONFIGHUB_WORKER_SECRET: "$secret"`)
	add("YAML")
	add("# argobot records each release's live status on the Release, which takes")
	add("# EditChildren on the Target it was published to. apply.sh grants it on the")
	add("# Targets it creates; this covers the ones an earlier apply.sh made without it.")
	add(`bot_user=$(cub worker get --space %s server-worker -o jq=.BridgeWorker.UserID | tr -d '"\n')`, targets)
	add(`[ -n "$bot_user" ] || { echo "could not read the bot user of %s/server-worker"; exit 1; }`, targets)
	add("# Read the Targets first: a failed list inside the loop's word list would not")
	add("# stop the script, and argobot would be installed without the grant.")
	add(`target_slugs=$(cub target list --space %s -o jq='.[].Target.Slug' | tr -d '"') || { echo "could not list the Targets in %s"; exit 1; }`, targets, targets)
	add(`[ -n "$target_slugs" ] || { echo "%s holds no Targets: run apply.sh first"; exit 1; }`, targets)
	add(`for t in $target_slugs; do`)
	add(`  cub target update "$t" --space %s --permission "EditChildren:${bot_user}" --quiet`, targets)
	add("done")
	add(`k -n argobot set env deployment/argobot CONFIGHUB_URL="$url" ARGO_NAMESPACE="$ns"`)
	add(`k -n argobot set image deployment/argobot argobot="ghcr.io/confighub/argobot:$version"`)
	add(`k -n argobot rollout status deployment/argobot --timeout=5m`)
	add("")
	add(`echo "argobot $version runs as %s/server-worker. What it does:"`, targets)
	add(`echo "  kubectl --context $ctx -n argobot logs deploy/argobot"`)
	add(`echo "Each release an Application syncs carries its live status:"`)
	add(`echo "  cub release list --space <variant Space>"`)
	return strings.Join(L, "\n") + "\n"
}
