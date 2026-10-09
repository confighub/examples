package flux

import (
	"fmt"
	"strings"
)

// Flux has no reporter of its own. `cub flux status --watch` is one, for as
// long as someone keeps it running where kubectl and a signed-in cub are. This
// puts the same reporter on the cluster, as argobot is for Argo CD: one pod
// that reads the layers and records each release's live status, signed in as
// the Targets' worker.
//
// It is this plugin's own binary, at its own release, fetched by an init
// container and checked against the release's checksum, and run in the
// Kubernetes project's kubectl image, which gives it the kubectl it reads the
// cluster with. Nothing is built or published for it beyond the release.

// These are what the pod is made of. Each is pinned.
const (
	fluxbotName     = "fluxbot"
	fluxbotFetch    = "curlimages/curl:8.16.0"
	fluxbotRuntime  = "registry.k8s.io/kubectl:v1.34.1"
	fluxbotReleases = "https://github.com/confighub/examples/releases/download"
)

// PluginVersion is this plugin's version, which the command sets: fluxbot.sh
// fetches the binary of that release.
var PluginVersion = "dev"

// FluxbotScript writes fluxbot.sh, which installs the reporter on one cluster.
// version is this plugin's; a build that is no release has no binary to
// fetch, so the script then needs FLUXBOT_URL.
func FluxbotScript(prefix, version string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }
	raw := func(s string) { L = append(L, s) }

	add("#!/usr/bin/env bash")
	add("# Run a live-status reporter for Flux on a cluster: fluxbot.")
	add("#")
	add("#   FLUX_CONTEXT=<kubectl context> CLUSTER=<cluster, as plan names it> \\")
	add("#     CONFIGHUB_URL=<ConfigHub's address as the cluster reaches it> bash fluxbot.sh")
	add("#")
	add("# Flux has no reporter of its own, so without this something has to keep")
	add("# `cub flux status --watch` running for the cluster. fluxbot is that same")
	add("# command, on the cluster: one pod in the Flux namespace that")
	add("#")
	add("# - finds every Kustomization there reading a ConfigHub Space of this fleet,")
	add("# - reads what Flux says of it, and")
	add("# - records that as live status on the release Flux applied, which is what")
	add("#   the Healthy gate and the ConfigHub UI read.")
	add("#")
	add("# It changes nothing on the cluster but itself. What it adds:")
	add("#")
	add("# - in ConfigHub, EditChildren on this cluster's Target for the Targets'")
	add("#   worker, which is what lets it record a release's live status. That worker")
	add("#   is the credential Flux already pulls with;")
	add("# - on the cluster, in the Flux namespace: a Secret with that worker's ID and")
	add("#   secret, a ServiceAccount that may read Kustomizations and OCIRepositories")
	add("#   there and nothing else, and a Deployment of one pod.")
	add("#")
	add("# The pod runs this plugin's own release binary, which an init container")
	add("# fetches from GitHub and checks against the release's checksum, so the")
	add("# cluster has to reach github.com once, when the pod starts.")
	add("#")
	add("# Run it after handover.sh or join.sh: before that no layer reads ConfigHub and")
	add("# there is nothing to report. To take it out again, see the end.")
	add("set -euo pipefail")
	add(`ctx=${FLUX_CONTEXT:-$(kubectl config current-context 2>/dev/null || true)}`)
	add(`[ -n "$ctx" ] || { echo "no kubectl context: set FLUX_CONTEXT to the cluster to report on"; exit 1; }`)
	add(`echo "every kubectl call below uses context $ctx"`)
	add(`k() { kubectl --context "$ctx" "$@"; }`)
	add(`cluster=${CLUSTER:-}`)
	add(`[ -n "$cluster" ] || { echo "set CLUSTER to this cluster's name, as cub flux plan prints it: its Target is %s/<name>"; exit 1; }`, targets)
	add(`url=${CONFIGHUB_URL:-}`)
	add(`[ -n "$url" ] || { echo "set CONFIGHUB_URL to ConfigHub's address as the cluster reaches it, e.g. https://hub.confighub.com"; exit 1; }`)
	add(`ns=${FLUX_NAMESPACE:-flux-system}`)
	if version == "" || version == "dev" {
		add("# This script was written by a build that is no release, so there is no")
		add("# release binary to name: say where one is.")
		add(`from=${FLUXBOT_URL:-}`)
		add(`[ -n "$from" ] || { echo "set FLUXBOT_URL to where cub-flux-linux-<arch> and its .sha256 are served: this script came from an unreleased build"; exit 1; }`)
	} else {
		add(`from=${FLUXBOT_URL:-%s/cub-flux-v%s}`, fluxbotReleases, version)
	}
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not signed in to ConfigHub: run 'cub auth login', or 'cub auth login --private-key=<key>' on a server with no identity provider"; exit 1; }`)
	add(`cub target get --space %s "$cluster" >/dev/null 2>&1 || { echo "%s/$cluster is not a Target: is CLUSTER right, and has apply.sh run?"; exit 1; }`, targets, targets)
	add("")
	add("# The grant first: a reporter without it is refused on every write.")
	add(`bot_user=$(cub worker get --space %s server-worker -o jq=.BridgeWorker.UserID | tr -d '"\n')`, targets)
	add(`[ -n "$bot_user" ] || { echo "could not read the bot user of %s/server-worker"; exit 1; }`, targets)
	add(`cub target update "$cluster" --space %s --permission "EditChildren:${bot_user}" --quiet`, targets)
	add("")
	add("# Its credential: the worker's ID and secret go from cub into the Secret")
	add("# through the heredoc, never to disk or the command line; read first, so a")
	add("# failed read stops here rather than installing an empty credential.")
	add(`id=$(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '"\n')`, targets)
	add(`secret=$(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '"\n')`, targets)
	add(`[ -n "$id" ] && [ -n "$secret" ] || { echo "could not read the credentials of %s/server-worker"; exit 1; }`, targets)
	raw(`cat <<YAML | k apply -f - >/dev/null`)
	raw("apiVersion: v1")
	raw("kind: Secret")
	raw("metadata:")
	add("  name: %s", fluxbotName)
	raw(`  namespace: "$ns"`)
	raw("stringData:")
	raw(`  CONFIGHUB_WORKER_ID: "$id"`)
	raw(`  CONFIGHUB_WORKER_SECRET: "$secret"`)
	raw("YAML")
	add("")
	add("# What it may read, and the pod. The init container fetches the binary for the")
	add("# node's architecture and stops unless its checksum is the release's.")
	raw(`cat <<YAML | k apply -f -`)
	raw("apiVersion: v1")
	raw("kind: ServiceAccount")
	raw("metadata:")
	add("  name: %s", fluxbotName)
	raw(`  namespace: "$ns"`)
	raw("---")
	raw("apiVersion: rbac.authorization.k8s.io/v1")
	raw("kind: Role")
	raw("metadata:")
	add("  name: %s", fluxbotName)
	raw(`  namespace: "$ns"`)
	raw("rules:")
	raw(`  - apiGroups: ["kustomize.toolkit.fluxcd.io"]`)
	raw(`    resources: ["kustomizations"]`)
	raw(`    verbs: ["get", "list"]`)
	raw(`  - apiGroups: ["source.toolkit.fluxcd.io"]`)
	raw(`    resources: ["ocirepositories"]`)
	raw(`    verbs: ["get", "list"]`)
	raw("---")
	raw("apiVersion: rbac.authorization.k8s.io/v1")
	raw("kind: RoleBinding")
	raw("metadata:")
	add("  name: %s", fluxbotName)
	raw(`  namespace: "$ns"`)
	raw("roleRef:")
	raw("  apiGroup: rbac.authorization.k8s.io")
	raw("  kind: Role")
	add("  name: %s", fluxbotName)
	raw("subjects:")
	raw("  - kind: ServiceAccount")
	add("    name: %s", fluxbotName)
	raw(`    namespace: "$ns"`)
	raw("---")
	raw("apiVersion: apps/v1")
	raw("kind: Deployment")
	raw("metadata:")
	add("  name: %s", fluxbotName)
	raw(`  namespace: "$ns"`)
	add("  labels: {app.kubernetes.io/name: %s}", fluxbotName)
	raw("spec:")
	raw("  replicas: 1")
	raw("  strategy: {type: Recreate}")
	add("  selector: {matchLabels: {app.kubernetes.io/name: %s}}", fluxbotName)
	raw("  template:")
	raw("    metadata:")
	add("      labels: {app.kubernetes.io/name: %s}", fluxbotName)
	raw("    spec:")
	add("      serviceAccountName: %s", fluxbotName)
	raw("      securityContext: {runAsNonRoot: true, runAsUser: 65532, runAsGroup: 65532, fsGroup: 65532, seccompProfile: {type: RuntimeDefault}}")
	raw("      volumes:")
	raw("        - {name: tools, emptyDir: {}}")
	raw("        - {name: tmp, emptyDir: {}}")
	raw("      initContainers:")
	raw("        - name: fetch")
	add("          image: %s", fluxbotFetch)
	raw("          securityContext: {allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: {drop: [ALL]}}")
	raw("          volumeMounts: [{name: tools, mountPath: /tools}]")
	raw("          env:")
	raw(`            - {name: FROM, value: "$from"}`)
	raw("          command:")
	raw("            - sh")
	raw("            - -ec")
	raw("            - |")
	raw(`              case "\$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "no cub-flux build for \$(uname -m)"; exit 1 ;; esac`)
	raw(`              cd /tools`)
	raw(`              curl -fsSL -o cub-flux "\$FROM/cub-flux-linux-\$arch"`)
	raw(`              want=\$(curl -fsSL "\$FROM/cub-flux-linux-\$arch.sha256" | cut -d' ' -f1)`)
	raw(`              have=\$(sha256sum cub-flux | cut -d' ' -f1)`)
	raw(`              [ -n "\$want" ] && [ "\$want" = "\$have" ] || { echo "checksum of cub-flux-linux-\$arch is \$have, and the release says \$want"; exit 1; }`)
	raw(`              chmod 0555 cub-flux`)
	raw("      containers:")
	add("        - name: %s", fluxbotName)
	add("          image: %s", fluxbotRuntime)
	raw("          securityContext: {allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: {drop: [ALL]}}")
	raw("          volumeMounts: [{name: tools, mountPath: /tools, readOnly: true}, {name: tmp, mountPath: /tmp}]")
	add(`          command: ["/tools/cub-flux", "status", "--discover", "--watch", "--prefix", "%s", "--namespace", "$ns"]`, prefix)
	raw("          env:")
	raw(`            - {name: CONFIGHUB_URL, value: "$url"}`)
	raw("            - {name: HOME, value: /tmp}")
	raw("          envFrom:")
	add("            - secretRef: {name: %s}", fluxbotName)
	raw("          resources:")
	raw("            requests: {cpu: 10m, memory: 32Mi}")
	raw("            limits: {memory: 128Mi}")
	raw("YAML")
	add("# A credential or address that changed does not restart the pod by itself.")
	add(`k -n "$ns" rollout restart deployment/%s >/dev/null`, fluxbotName)
	add(`k -n "$ns" rollout status deployment/%s --timeout=5m`, fluxbotName)
	add("")
	add(`echo "fluxbot runs in $ns as %s/server-worker. What it reports, each pass:"`, targets)
	add(`echo "  kubectl --context $ctx -n $ns logs deploy/%s"`, fluxbotName)
	add(`echo "Each release a layer applies carries its live status:"`)
	add(`echo "  cub release list --space <variant Space>"`)
	add(`echo "To take it out again:"`)
	add(`echo "  kubectl --context $ctx -n $ns delete deployment,serviceaccount,role,rolebinding,secret %s"`, fluxbotName)
	add(`echo "and, if nothing else of yours records live status as that worker, its grant:"`)
	add(`echo "  cub target update $cluster --space %s --permission -EditChildren:$bot_user"`, targets)
	return strings.Join(L, "\n") + "\n"
}
