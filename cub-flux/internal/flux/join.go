package flux

import (
	"fmt"
	"strings"
)

// JoinScript brings a cluster that has Flux and none of the fleet's layers
// onto ConfigHub from the start: the gateway credential and the root, and
// every layer then arrives from the cluster's layers Space. It is what
// `cub cluster up` does for Argo CD, for a cluster that already exists and
// runs Flux. A cluster already running the layers from Git is handed over
// with handover.sh instead.
func JoinScript(p *Plan, prefix string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	add("#!/usr/bin/env bash")
	add("# Join a new cluster: it runs this fleet's layers from ConfigHub from the")
	add("# start. Written by `cub flux apply`. Read it, then run it:")
	add("#")
	add("#   FLUX_CONTEXT=<kubectl context> CLUSTER=<cluster> CONFIGHUB_OCI=<gateway> bash join.sh")
	add("#")
	add("# The cluster needs Flux (flux install) and none of the layers yet; a cluster")
	add("# already running them from Git is handed over with handover.sh. Run")
	add("# apply.sh with CONFIGHUB_OCI set first: it fills the layers Space this reads.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`ctx=${FLUX_CONTEXT:-$(kubectl config current-context 2>/dev/null || true)}`)
	add(`[ -n "$ctx" ] || { echo "no kubectl context: set FLUX_CONTEXT to the cluster joining"; exit 1; }`)
	add(`k() { kubectl --context "$ctx" "$@"; }`)
	L = append(L, credentialHelpers()...)
	add(`ns=${FLUX_NAMESPACE:-flux-system}`)
	add(`cluster=${CLUSTER:-}`)
	add(`case "$cluster" in %s) ;; *) echo "set CLUSTER=<one of: %s>"; exit 1 ;; esac`,
		strings.ReplaceAll(clusterNames(p), ", ", "|"), clusterNames(p))
	for _, c := range p.Clusters {
		if c.LayersSpace != "" {
			add(`[ "$cluster" != %s ] || { echo "%s is handed over already: its root reads %s, so there is nothing to join."; exit 1; }`, q(c.Name), c.Name, c.LayersSpace)
		}
	}
	add(`addr=${CONFIGHUB_OCI:-}`)
	add("# Every use below writes the scheme itself, so take it off if it was given.")
	add(`addr=${addr#oci://}`)
	add(`[ -n "$addr" ] || { echo "set CONFIGHUB_OCI to the gateway host this cluster reaches (CONFIGHUB_OCI_PLAIN_HTTP=1 if it serves plain HTTP)"; exit 1; }`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add("")

	add(`step "1/3 Check before changing anything"`)
	add(`echo "  every kubectl call below uses context $ctx"`)
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not signed in to ConfigHub: run 'cub auth login', or 'cub auth login --private-key=<key>' on a server with no identity provider"; exit 1; }`)
	add(`k get crd kustomizations.kustomize.toolkit.fluxcd.io >/dev/null || { echo "Flux is not installed on $ctx: run flux install first"; exit 1; }`)
	add("# A layer already here is one to hand over, not to create: the root would")
	add("# take it over without the checks handover.sh makes first.")
	add(`present=""`)
	add(`layer_here() { k -n "$ns" get kustomization "$1" >/dev/null 2>&1 && present="$present $1" || true; }`)
	for _, step := range p.Order {
		for _, name := range step {
			add(`layer_here %s`, q(name))
		}
	}
	add(`[ -z "$present" ] || { echo "these layers already run here:$present. Hand this cluster over with handover.sh instead."; exit 1; }`)
	add("")

	add(`step "2/3 The credential and the root"`)
	L = append(L, pullSecretLines(prefix, targets)...)
	L = append(L, rootLines(prefix)...)
	add(`layers_still_current`)
	add(`k apply -f "bootstrap/$cluster/%s.yaml"`, RootName)
	add("")

	add(`step "3/3 Every layer arrives from ConfigHub"`)
	add(`k -n "$ns" wait --for=condition=Ready "kustomization/%s" --timeout=5m`, RootName)
	add(`root_applied_checked`)
	add(`arrive() {`)
	add(`  local i; for i in $(seq 1 36); do k -n "$ns" get kustomization "$1" >/dev/null 2>&1 && break; sleep 5; done`)
	add(`  k -n "$ns" wait --for=condition=Ready "kustomization/$1" --timeout=5m >/dev/null`)
	add(`  echo "  $1 Ready at $(k -n "$ns" get kustomization "$1" -o jsonpath='{.status.lastAppliedRevision}')"`)
	add(`}`)
	on := layersByCluster(p)
	for _, step := range p.Order {
		for _, name := range step {
			if cs := on[name]; len(cs) > 0 && len(cs) < len(p.Clusters) {
				add(`case "$cluster" in %s) arrive %s ;; esac`, strings.Join(cs, "|"), q(name))
			} else {
				add(`arrive %s`, q(name))
			}
		}
	}
	add(`echo`)
	add(`echo "Joined. Commit bootstrap/$cluster/ into this cluster's flux-system path so the root survives a reconcile."`)
	add(`echo "Report its status from the cluster itself: FLUX_CONTEXT=$ctx CLUSTER=$cluster CONFIGHUB_URL=<ConfigHub address> bash fluxbot.sh"`)
	add(`echo "or from here, for as long as you keep it running: cub flux status <fleet> --cluster $cluster --kube-context $ctx --watch"`)
	return strings.Join(L, "\n") + "\n"
}

// rootLines write the root for the cluster being handed over or joined into
// bootstrap/<cluster>/, with the gateway filled in.
func rootLines(prefix string) []string {
	return []string{
		`mkdir -p "bootstrap/$cluster"`,
		`insecure=false; [ -z "${CONFIGHUB_OCI_PLAIN_HTTP:-}" ] || insecure=true`,
		`layers_space=` + strings.Replace(q(DeliverySpace(prefix, "__CLUSTER__")), "__CLUSTER__", `'"$cluster"'`, 1),
		// The root reads the layers Space's latest release, so the release it
		// is checked against is pinned here: read once, confirmed still the
		// newest just before the root goes on, and confirmed to be what the
		// root applied. A republish in between would otherwise go out as if
		// checked, and a delivery Unit decides prune and targetNamespace.
		`layers_digest=$(cub release get --space "$layers_space" --oci-reference latest -o jq=.Release.ManifestDigest 2>/dev/null | tr -d '"') || true`,
		`[ -n "$layers_digest" ] || { echo "$layers_space has no published release: run apply.sh with CONFIGHUB_OCI set first"; exit 1; }`,
		`echo "  $layers_space is at $layers_digest"`,
		`layers_still_current() { local now; now=$(cub release get --space "$layers_space" --oci-reference latest -o jq=.Release.ManifestDigest | tr -d '"'); [ "$now" = "$layers_digest" ] || { echo "  $layers_space was republished since it was read ($layers_digest, now $now); run again to check what is published now" >&2; return 1; }; }`,
		`root_applied_checked() { local a; a=$(k -n "$ns" get kustomization ` + RootName + ` -o jsonpath='{.status.lastAppliedRevision}'); [ "${a##*@}" = "$layers_digest" ] || { echo "  the root applied $a, not the checked $layers_digest" >&2; return 1; }; }`,
		fmt.Sprintf(`cat <<'YAML' | sed -e "s|%s|$addr|" -e "s|%s|$insecure|" -e "s|%s|$ns|" -e "s|__CLUSTER__|$cluster|" > "bootstrap/$cluster/%s.yaml"`, gatewayMarker, insecureMarker, namespaceMarker, RootName),
		strings.TrimRight(RootManifests(prefix, "__CLUSTER__"), "\n"),
		"YAML",
	}
}

// pullSecretLines write the Secret Flux pulls from the gateway with, in the
// cluster being handed over or joined: a token of the fleet's service account,
// the username being the word token. A dockerconfigjson Secret, which is what
// an OCIRepository reads. A token the Secret holds is kept while ConfigHub
// lists it as live; otherwise a new one is issued, and the cluster's earlier
// ones are revoked once the Secret holds it. The Secret is written with create
// or replace, not apply, which would keep a copy of the token in an
// annotation, and the token reaches it through a file descriptor, on no
// command line.
func pullSecretLines(prefix, targets string) []string {
	account := deliveryAccount(prefix)
	secret := "confighub-" + targets
	return []string{
		fmt.Sprintf("# A token of %s the Secret already holds is kept while ConfigHub lists it", account),
		"# as live; otherwise a new one is issued, and this cluster's others are",
		"# revoked once the Secret holds it. The token goes from cub into the Secret",
		"# through variables and a file descriptor, never to disk or a command line.",
		`token_desc="Flux, cluster $cluster"`,
		fmt.Sprintf(`held_jti=$(k -n "$ns" get secret %s -o jsonpath='{.metadata.annotations.confighub\.com/token-jti}' 2>/dev/null || true)`, secret),
		`token=""; jti=""`,
		fmt.Sprintf(`if [ -n "$held_jti" ] && live_tokens %s "$token_desc" | grep -xF "$held_jti" >/dev/null; then`, account),
		fmt.Sprintf(`  token=$(k -n "$ns" get secret %s -o jsonpath='{.data.\.dockerconfigjson}' | base64 --decode | sed -n 's/.*"password":"\([^"]*\)".*/\1/p') || token=""`, secret),
		fmt.Sprintf(`  [ -z "$token" ] || { jti=$held_jti; echo "the pull Secret's token of %s is live: kept"; }`, account),
		`fi`,
		`if [ -z "$token" ]; then`,
		fmt.Sprintf(`  issued=$(cub serviceaccount token create %s --description "$token_desc" -o jq='.UserToken.Jti + " " + .Token' | tr -d '"\n') || issued=""`, account),
		`  jti=${issued%% *}; token=${issued#* }`,
		fmt.Sprintf(`  [ -n "$jti" ] && [ -n "$token" ] && [ "$jti" != "$token" ] || { echo "could not issue a token for %s"; exit 1; }`, account),
		`fi`,
		`auth=$(printf 'token:%s' "$token" | base64 | tr -d '\n')`,
		fmt.Sprintf(`k -n "$ns" create secret generic %s --type=kubernetes.io/dockerconfigjson \`, secret),
		`  --from-file=.dockerconfigjson=<(printf '{"auths":{"%s":{"username":"token","password":"%s","auth":"%s"}}}' "$addr" "$token" "$auth") \`,
		fmt.Sprintf(`  --dry-run=client -o yaml | k annotate --local -f - confighub.com/token-jti="$jti" -o yaml | write_secret "$ns" %s`, secret),
		`unset token issued auth`,
		fmt.Sprintf(`live_tokens %s "$token_desc" | { while read -r old; do`, account),
		`  [ "$old" != "$jti" ] || continue`,
		fmt.Sprintf(`  cub serviceaccount token revoke %[1]s "$old" --quiet && echo "revoked an earlier token of %[1]s" || echo "could not revoke an earlier token of %[1]s: cub serviceaccount token revoke %[1]s $old"`, account),
		fmt.Sprintf(`done; } || echo "could not list the tokens of %s, so earlier ones are not revoked"`, account),
	}
}

// layersByCluster is which clusters have each layer.
func layersByCluster(p *Plan) map[string][]string {
	on := map[string][]string{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				name := v.Kustomization[strings.Index(v.Kustomization, "/")+1:]
				on[name] = append(on[name], v.Cluster)
			}
		}
	}
	return on
}
