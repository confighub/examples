#!/usr/bin/env bash
set -euo pipefail

# This script documents the failed-sync scenario. It does not run
# anything against a cluster, and it does not upload anything to
# ConfigHub. The one thing it is safe to actually do is render the broken
# overlay locally, which is read-only.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLE_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

cat <<'EOF_STEPS'
Scenario 2: failed sync

The break is already written down as a file in this repo:
  apps/apptique/overlays/failed-sync/redis-cache.yaml

It is the healthy app plus one RedisCache custom resource whose CRD is not
installed, all in the same apptique-broken-states namespace. Rendering it
locally is read-only and safe:
EOF_STEPS

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1 (skipping the local render)" >&2
    exit 0
  }
}
require_cmd kustomize

echo
echo "==> kustomize build apps/apptique/overlays/failed-sync"
kustomize build "$EXAMPLE_DIR/apps/apptique/overlays/failed-sync"

cat <<'EOF_NEXT'

That render is what setup.sh never uploads. To see the real failure, in
YOUR OWN terminal, against a cluster you control, with this same repo
checked out and Argo CD or Flux pointed at it:

  # Whether the CRD exists, and what applied anyway in the namespace:
  kubectl get crd rediscaches.cache.apptique.example
  kubectl -n apptique-broken-states get deployment,service frontend

  # Argo CD, after pointing an Application at this overlay:
  argocd app sync apptique-broken-states
  argocd app get apptique-broken-states -o json

  # Flux, after pointing a Kustomization at this overlay:
  flux get kustomizations apptique-broken-states

None of these commands are run by this script. See ./README.md for what each one
reports.
EOF_NEXT
