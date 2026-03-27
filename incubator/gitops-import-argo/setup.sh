#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME=${CLUSTER_NAME:-gitops-import-argo}
WITH_WORKER=false
WITH_CONTRAST=false
EXPLAIN=false
EXPLAIN_JSON=false
ARGOCD_HOST_PORT_VALUE=${ARGOCD_HOST_PORT:-}

for arg in "$@"; do
    case "$arg" in
        --with-worker) WITH_WORKER=true ;;
        --with-contrast) WITH_CONTRAST=true ;;
        --explain) EXPLAIN=true ;;
        --explain-json) EXPLAIN_JSON=true ;;
        *)
            echo "Unknown argument: $arg" >&2
            exit 1
            ;;
    esac
done

pick_argocd_host_port() {
    python3 - <<'PY'
import socket

def is_free(port: int) -> bool:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        sock.bind(("0.0.0.0", port))
    except OSError:
        return False
    finally:
        sock.close()
    return True

for port in range(9080, 9100):
    if is_free(port):
        print(port)
        break
else:
    raise SystemExit("No free ArgoCD host port found in 9080-9099")
PY
}

if [ -z "$ARGOCD_HOST_PORT_VALUE" ] && [ -f "$SCRIPT_DIR/var/argocd-host-port.txt" ]; then
    ARGOCD_HOST_PORT_VALUE="$(cat "$SCRIPT_DIR/var/argocd-host-port.txt")"
fi

if [ -z "$ARGOCD_HOST_PORT_VALUE" ]; then
    ARGOCD_HOST_PORT_VALUE="$(pick_argocd_host_port)"
fi

export ARGOCD_HOST_PORT="$ARGOCD_HOST_PORT_VALUE"

if $EXPLAIN_JSON; then
    cat <<JSON
{
  "example": "gitops-import-argo",
  "mutates": false,
  "clusterName": "$CLUSTER_NAME",
  "argocdHostPort": "$ARGOCD_HOST_PORT",
  "steps": [
    "bin/create-cluster",
    "bin/install-argocd",
    "bin/setup-apps"
  ],
  "optionalSteps": [
    "bin/install-worker (requires CUB_SPACE and cub auth)",
    "bin/setup-contrast-apps"
  ],
  "writes": [
    "kind cluster",
    "ArgoCD installation",
    "healthy guestbook ArgoCD applications",
    "optional ConfigHub worker",
    "optional contrast fixtures",
    "local var/*.kubeconfig, argocd-admin-password.txt, and argocd-host-port.txt"
  ]
}
JSON
    exit 0
fi

if $EXPLAIN; then
    cat <<TEXT
Example: gitops-import-argo
Mutates: yes
ArgoCD host port: $ARGOCD_HOST_PORT

Default steps:
- create a kind cluster named $CLUSTER_NAME
- install ArgoCD into that cluster
- apply the healthy guestbook Applications to ArgoCD

Optional steps:
- install a ConfigHub worker if --with-worker is set and CUB_SPACE is available
- apply brownfield contrast fixtures from cub-scout if --with-contrast is set

Writes:
- local kind cluster state
- local var/$CLUSTER_NAME.kubeconfig
- local var/argocd-admin-password.txt
- local var/argocd-host-port.txt
- optional ConfigHub worker objects and targets
TEXT
    exit 0
fi

mkdir -p "$SCRIPT_DIR/var"
printf '%s\n' "$ARGOCD_HOST_PORT" > "$SCRIPT_DIR/var/argocd-host-port.txt"

"$SCRIPT_DIR/bin/create-cluster" "$CLUSTER_NAME"
"$SCRIPT_DIR/bin/install-argocd" "$CLUSTER_NAME"
"$SCRIPT_DIR/bin/setup-apps" "$CLUSTER_NAME"

if $WITH_CONTRAST; then
    "$SCRIPT_DIR/bin/setup-contrast-apps" "$CLUSTER_NAME"
fi

if $WITH_WORKER; then
    if [ -z "${CUB_SPACE:-}" ]; then
        echo "Error: --with-worker requires CUB_SPACE to be set" >&2
        exit 1
    fi
    "$SCRIPT_DIR/bin/install-worker" "$CLUSTER_NAME"
fi

echo "gitops-import-argo setup complete."
