package argo

import (
	"fmt"
	"strings"
)

// ArgobotVersion is the argobot release argobot.sh installs unless told
// otherwise. It has to be one that runs as a service account on a Channel,
// which argobot does from v0.2.0; v0.2.3 is the first built against the SDK
// that signs in at the OAuth token endpoint. It also finds a moved estate's
// Applications by the Space their source reads, since they keep Argo's names.
const ArgobotVersion = "v0.2.3"

// argobotChannel is the Channel argobot claims, in the Targets' Space.
const argobotChannel = "argobot"

// ArgobotScript writes argobot.sh, which runs argobot beside Argo CD as the
// service account the Targets grant access to, on a Channel beside them.
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
	add("# argobot connects to ConfigHub as the service account %s, the one every", targets)
	add("# Target here grants access to, so it acts for exactly these Targets. It")
	add("# listens on the Channel %s/%s and does two things:", targets, argobotChannel)
	add("#")
	add("# - When a variant's Space publishes a release, it hard-refreshes the")
	add("#   Applications reading that Space. Argo CD caches the digest a tag resolved")
	add("#   to, so without this a release waits for Argo's own poll.")
	add("# - It records each Application's live state (sync, health, operation) on the")
	add("#   Release the Application synced, which the Healthy gate and the ConfigHub")
	add("#   UI read.")
	add("#")
	add("# Both find an Application by the Space its source reads. argobot.sh needs")
	add("# argobot v0.2.0 or later, the first that runs as a service account.")
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
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not signed in to ConfigHub: run 'cub auth login', or 'cub auth login --private-key=<key>' on a server with no identity provider"; exit 1; }`)
	add("")
	add("# argobot records each release's live status on the Release, which takes")
	add("# EditChildren on the Target it was published to, and finds the Release")
	add("# through View and ViewChildren. apply.sh grants these on the Targets it")
	add("# creates; this covers the ones an earlier apply.sh granted to a worker.")
	add("cub serviceaccount create %s --org-role none --allow-exists --quiet", targets)
	add(`sa="serviceaccount:%s"`, targets)
	add("# Read the Targets first: a failed list inside the loop's word list would not")
	add("# stop the script, and argobot would be installed without the grant.")
	add(`target_slugs=$(cub target list --space %s -o jq='.[].Target.Slug' | tr -d '"') || { echo "could not list the Targets in %s"; exit 1; }`, targets, targets)
	add(`[ -n "$target_slugs" ] || { echo "%s holds no Targets: run apply.sh first"; exit 1; }`, targets)
	add(`for t in $target_slugs; do`)
	add(`  cub target update "$t" --space %s --permission "View:${sa}" --permission "ViewChildren:${sa}" --permission "EditChildren:${sa}" --quiet`, targets)
	add("done")
	add("# The Channel argobot claims, subscribed to what `argobot --print-subscriptions`")
	add("# prints: every published release, of which argobot acts on those of Spaces")
	add("# its Applications read.")
	add(`printf '%%s\n' '[{"EventTypes":["release.published"]}]' |`)
	add(`  cub channel create %s --space %s --subscriptions - --permission "Claim:${sa}" --allow-exists --quiet`, argobotChannel, targets)
	add("")
	add("# Its Namespace, ServiceAccount, RBAC and Deployment, as argobot publishes them.")
	add(`k apply -f "https://raw.githubusercontent.com/confighub/argobot/$version/manifests/argobot.yaml"`)
	add("# Its credential: a key of the service account, in the Secret the manifest")
	add("# reads. A Secret that already holds one is left as it is, so running this")
	add("# again keeps the key argobot has. The private half is written to a directory")
	add("# of its own and removed as the script ends.")
	add(`have=$(k -n argobot get secret argobot-secrets -o jsonpath='{.data.CONFIGHUB_AUTH_PRIVATE_KEY}' 2>/dev/null || true)`)
	add(`if [ -z "$have" ]; then`)
	add(`  keys=$(mktemp -d)`)
	add(`  trap 'rm -rf "$keys"' EXIT`)
	add(`  cub serviceaccount key add %s --generate "$keys/argobot.jwk" --description "argobot on $ctx" >/dev/null`, targets)
	add(`  k -n argobot create secret generic argobot-secrets --from-file=CONFIGHUB_AUTH_PRIVATE_KEY="$keys/argobot.jwk" --dry-run=client -o yaml | k apply -f -`)
	add("fi")
	add(`k -n argobot set env deployment/argobot CONFIGHUB_URL="$url" CONFIGHUB_CHANNEL=%s/%s ARGO_NAMESPACE="$ns"`, targets, argobotChannel)
	add(`k -n argobot set image deployment/argobot argobot="ghcr.io/confighub/argobot:$version"`)
	add(`k -n argobot rollout status deployment/argobot --timeout=5m`)
	add("")
	add(`echo "argobot $version runs as the service account %s, on the Channel %s/%s. What it does:"`, targets, targets, argobotChannel)
	add(`echo "  kubectl --context $ctx -n argobot logs deploy/argobot"`)
	add(`echo "Each release an Application syncs carries its live status:"`)
	add(`echo "  cub release list --space <variant Space>"`)
	return strings.Join(L, "\n") + "\n"
}
