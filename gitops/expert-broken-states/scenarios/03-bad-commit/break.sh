#!/usr/bin/env bash
set -euo pipefail

# This script documents the bad-commit scenario. It does not run anything
# against a cluster, and it does not upload anything to ConfigHub. The one
# thing it is safe to actually do is render the broken overlay locally,
# which is read-only.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLE_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

cat <<'EOF_STEPS'
Scenario 3: bad commit

The break is already written down as a file in this repo:
  apps/apptique/overlays/bad-commit/kustomization.yaml

It is the healthy app with one field changed: the frontend Service's
targetPort moves to 8080, while the container and its probes stay on port
80. Rendering it locally is read-only and safe:
EOF_STEPS

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1 (skipping the local render)" >&2
    exit 0
  }
}
require_cmd kustomize

echo
echo "==> kustomize build apps/apptique/overlays/bad-commit"
kustomize build "$EXAMPLE_DIR/apps/apptique/overlays/bad-commit"

cat <<'EOF_NEXT'

That render is what setup.sh never uploads. To see the real failure, in
YOUR OWN terminal, against a cluster you control, with this same repo
checked out and Argo CD or Flux pointed at it:

  argocd app get apptique-broken-states -o json
  flux get kustomizations apptique-broken-states
  kubectl -n apptique-broken-states get endpointslices \
    -l kubernetes.io/service-name=frontend
  kubectl -n apptique-broken-states run probe --rm -i --restart=Never \
    --image=curlimages/curl -- curl -sS -m 3 \
    http://frontend.apptique-broken-states.svc.cluster.local

None of these commands are run by this script. See ./README.md for what
each one shows, and why sync state alone does not catch this class of bug.
EOF_NEXT
