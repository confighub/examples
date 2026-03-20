#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME=${CLUSTER_NAME:-gitops-import-argo}

"$SCRIPT_DIR/bin/teardown" "$CLUSTER_NAME"
