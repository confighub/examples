#!/usr/bin/env bash
# Setup for springboot-platform-app-centric (ADT view)
#
# Creates ConfigHub spaces, units, and targets for inventory-api across dev, stage, prod.
# This example presents the App → Deployments → Targets view of the platform model.
#
# Usage:
#   ./setup.sh --explain              # Show the ADT view (read-only)
#   ./setup.sh --explain-json         # Show the setup plan as JSON (read-only)
#   ./setup.sh --explain --confighub-only   # Explain confighub-only mode
#   ./setup.sh --explain --with-targets     # Explain real-target mode
#   ./setup.sh                         # Default: noop targets (no cluster needed)
#   ./setup.sh --confighub-only        # ConfigHub only (no targets)
#   ./setup.sh --with-targets          # Real Kubernetes (requires cluster + worker)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="${SCRIPT_DIR}/../shared"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app-centric"
INFRA_SPACE="inventory-api-infra"
DEPLOY_NAMESPACE="inventory-api"
LOCAL_IMAGE="inventory-api:local"

# External targets (set via env vars for --with-targets mode)
WORKER_SPACE="${WORKER_SPACE:-}"
K8S_TARGET="${K8S_TARGET:-}"

ENVS=(dev stage prod)

space_name() {
  echo "inventory-api-${1}"
}

resolve_k8s_target() {
  local target_json k8s_target_count

  target_json="$(${CUB} target list --space "${WORKER_SPACE}" --json 2>/dev/null || echo '[]')"

  if [[ -n "${K8S_TARGET}" ]]; then
    if ! echo "${target_json}" | jq -e --arg slug "${K8S_TARGET}" \
      '[.[] | select(.Target.Slug == $slug and (.Target.ProviderType | ascii_downcase) == "kubernetes")] | length > 0' >/dev/null 2>&1; then
      echo "error: Kubernetes target '${K8S_TARGET}' not found in space '${WORKER_SPACE}'." >&2
      exit 1
    fi
    return 0
  fi

  k8s_target_count="$(echo "${target_json}" | jq '[.[] | select((.Target.ProviderType | ascii_downcase) == "kubernetes")] | length')"
  case "${k8s_target_count}" in
    0)
      echo "error: No Kubernetes targets found in space '${WORKER_SPACE}'." >&2
      exit 1
      ;;
    1)
      K8S_TARGET="$(echo "${target_json}" | jq -r '[.[] | select((.Target.ProviderType | ascii_downcase) == "kubernetes")][0].Target.Slug')"
      echo "Auto-detected Kubernetes target: ${WORKER_SPACE}/${K8S_TARGET}"
      ;;
    *)
      echo "error: Multiple Kubernetes targets found in space '${WORKER_SPACE}'." >&2
      echo "Export K8S_TARGET with the exact target slug to use." >&2
      exit 1
      ;;
  esac
}

show_adt_view() {
  local mode="${1:-noop}"
  cat <<'EOF'
================================================================================
                        APP - DEPLOYMENT - TARGET VIEW
================================================================================

APP: inventory-api
  A Spring Boot inventory service with feature flags and runtime tuning.
  Source: inventory-api Spring Boot application

DEPLOYMENTS:
  +------------------+----------------------+--------------------------+
  | Deployment       | ConfigHub Space      | Purpose                  |
  +------------------+----------------------+--------------------------+
  | dev              | inventory-api-dev    | Development iteration    |
  | stage            | inventory-api-stage  | Validation before prod   |
  | prod             | inventory-api-prod   | Production workload      |
  +------------------+----------------------+--------------------------+

EOF

  case "${mode}" in
    "noop")
      cat <<'EOF'
TARGETS (noop mode - default):
  +------------------+----------------------+--------------------------+
  | Deployment       | Target               | Delivers to              |
  +------------------+----------------------+--------------------------+
  | dev              | dev (Noop)           | (accepts, no delivery)   |
  | stage            | stage (Noop)         | (accepts, no delivery)   |
  | prod             | prod (Noop)          | (accepts, no delivery)   |
  +------------------+----------------------+--------------------------+

  Noop targets let you exercise the full mutation-to-apply workflow
  without needing a Kubernetes cluster.

EOF
      ;;
    "confighub-only")
      cat <<'EOF'
TARGETS (confighub-only mode):
  No targets. Units exist in ConfigHub only.
  Use this to inspect spaces and units before binding targets.

EOF
      ;;
    "real")
      cat <<'EOF'
TARGETS (real mode):
  +------------------+----------------------+--------------------------+
  | Deployment       | Target               | Delivers to              |
  +------------------+----------------------+--------------------------+
  | dev              | (none)               | ConfigHub only           |
  | stage            | (none)               | ConfigHub only           |
  | prod             | Kubernetes target    | Real cluster namespace   |
  +------------------+----------------------+--------------------------+

  Only prod is bound to a real Kubernetes target.
  Requires: Kind cluster, Docker image, ConfigHub worker.

EOF
      ;;
  esac

  cat <<'EOF'
MUTATION OUTCOMES:
  +------------------+---------------------------+----------------------+
  | Outcome          | Example Field             | Owner                |
  +------------------+---------------------------+----------------------+
  | Apply here       | feature.inventory.*       | app-team             |
  | Lift upstream    | spring.cache.*            | app-team             |
  | Block/escalate   | spring.datasource.*       | platform-engineering |
  +------------------+---------------------------+----------------------+

  Field routing rules: ../shared/field-routes.yaml

================================================================================
EOF
}

show_explain() {
  local mode="${1:-noop}"
  show_adt_view "${mode}"

  cat <<EOF

What this setup does:
EOF

  case "${mode}" in
    "noop")
      cat <<'EOF'
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 infra space: inventory-api-infra (server worker)
  - Creates 1 unit per space: inventory-api
  - Creates 1 Noop target per space
  - Binds units to Noop targets
  - Applies all units

Cluster required: No
Mutates ConfigHub: Yes
Mutates live infrastructure: No

EOF
      ;;
    "confighub-only")
      cat <<'EOF'
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 unit per space: inventory-api
  - Does NOT create targets or apply

Cluster required: No
Mutates ConfigHub: Yes
Mutates live infrastructure: No

EOF
      ;;
    "real")
      cat <<'EOF'
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 unit per space: inventory-api
  - Creates Kubernetes namespace: inventory-api
  - Binds prod unit to real Kubernetes target
  - Applies prod unit (triggers kubectl apply via worker)

Cluster required: Yes
Mutates ConfigHub: Yes
Mutates live infrastructure: Yes (prod only)

Required environment variables:
  WORKER_SPACE=<space-where-worker-lives>
  K8S_TARGET=<target-slug>  (optional if exactly one Kubernetes target exists)

EOF
      ;;
  esac

  cat <<'EOF'
Safe next steps:
  ./setup.sh                # Run with noop targets (default)
  ./verify.sh               # Verify consistency
  ./cleanup.sh              # Remove all example objects
EOF
}

show_explain_json() {
  local mode="${1:-noop}"
  local selected_flag="null"
  local mutates_live="false"
  local cluster_required="false"
  local creates_infra_space="false"
  local creates_targets="false"
  local applies_units="false"

  case "${mode}" in
    "noop")
      creates_infra_space="true"
      creates_targets="true"
      applies_units="true"
      ;;
    "confighub-only")
      selected_flag="\"--confighub-only\""
      ;;
    "real")
      selected_flag="\"--with-targets\""
      mutates_live="true"
      cluster_required="true"
      creates_targets="true"
      applies_units="true"
      ;;
  esac

  cat <<EOF
{
  "example_name": "springboot-platform-app-centric",
  "proof_type": "adt-view",
  "selected_mode": "${mode}",
  "selected_setup_flag": ${selected_flag},
  "default_mode": "noop",
  "mutates_confighub": true,
  "mutates_live_infra": ${mutates_live},
  "cluster_required": ${cluster_required},
  "creates_infra_space": ${creates_infra_space},
  "creates_targets": ${creates_targets},
  "applies_units": ${applies_units},
  "deployment_map": "./deployment-map.json",
  "next_steps": [
    "./setup.sh",
    "./verify.sh",
    "./cleanup.sh"
  ]
}
EOF
}

# Parse arguments
MODE="noop"
EXPLAIN=false
EXPLAIN_JSON=false

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --explain)
      EXPLAIN=true
      shift
      ;;
    --explain-json)
      EXPLAIN_JSON=true
      shift
      ;;
    --confighub-only)
      MODE="confighub-only"
      shift
      ;;
    --with-targets)
      MODE="real"
      shift
      ;;
    *)
      echo "Usage: $0 [--explain|--explain-json] [--confighub-only|--with-targets]" >&2
      exit 2
      ;;
  esac
done

if [[ "${EXPLAIN}" == "true" ]]; then
  show_explain "${MODE}"
  exit 0
fi

if [[ "${EXPLAIN_JSON}" == "true" ]]; then
  show_explain_json "${MODE}"
  exit 0
fi

# Require cub
command -v "${CUB}" >/dev/null 2>&1 || {
  echo "error: cub CLI not found. Install cub and run cub auth login first." >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || {
  echo "error: jq not found." >&2
  exit 1
}

# Check shared YAML files exist
for env in "${ENVS[@]}"; do
  yaml_file="${SHARED_DIR}/confighub/inventory-api-${env}.yaml"
  if [[ ! -f "${yaml_file}" ]]; then
    echo "error: missing ${yaml_file}" >&2
    exit 1
  fi
done

# Additional checks for real-targets mode
if [[ "${MODE}" == "real" ]]; then
  if [[ -z "${WORKER_SPACE}" ]]; then
    echo "error: WORKER_SPACE environment variable not set." >&2
    exit 1
  fi

  command -v kubectl >/dev/null 2>&1 || {
    echo "error: kubectl not found (required for --with-targets)." >&2
    exit 1
  }

  if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "error: Kubernetes cluster not reachable." >&2
    exit 1
  fi

  resolve_k8s_target
fi

case "${MODE}" in
  "real")
    echo "=== ADT setup with REAL Kubernetes deployment ==="
    echo ""
    echo "Worker space: ${WORKER_SPACE}"
    echo "Target:       ${K8S_TARGET}"
    echo "Namespace:    ${DEPLOY_NAMESPACE}"
    echo "Deployed:     prod only (dev/stage are ConfigHub-only)"
    ;;
  "noop")
    echo "=== ADT setup with Noop targets (simulation) ==="
    ;;
  *)
    echo "=== ADT setup (ConfigHub-only) ==="
    ;;
esac
echo ""
echo "All entities are labeled ExampleName=${EXAMPLE_LABEL} for easy cleanup."
echo ""

# Phase 1: Create spaces
echo "Phase 1: Creating spaces..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  ${CUB} space create "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --allow-exists \
    --quiet
  echo "  Created space: ${space}"
done

echo "  Done."
echo ""

# Phase 2: Create units from shared YAML
echo "Phase 2: Creating units..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  yaml_file="${SHARED_DIR}/confighub/inventory-api-${env}.yaml"

  # Delete existing unit first to ensure fresh YAML content is used
  if ${CUB} unit get --space "${space}" inventory-api >/dev/null 2>&1; then
    echo "y" | ${CUB} unit delete --space "${space}" inventory-api >/dev/null 2>&1 || true
  fi

  ${CUB} unit create --space "${space}" inventory-api "${yaml_file}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --quiet
  echo "  Created unit: ${space}/inventory-api"
done

echo "  Done."
echo ""

# Phase 3+: Mode-specific setup
case "${MODE}" in
  "real")
    echo "Phase 3: Creating Kubernetes namespace..."
    kubectl create namespace "${DEPLOY_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    echo "  Namespace: ${DEPLOY_NAMESPACE}"
    echo "  Done."
    echo ""

    echo "Phase 4: Preparing prod unit for real cluster deployment..."
    ${CUB} function do --space inventory-api-prod \
      --unit inventory-api \
      --change-desc "setup: set deployment namespace for real cluster delivery" \
      --quiet \
      set-namespace "${DEPLOY_NAMESPACE}"
    echo "  Updated namespace: ${DEPLOY_NAMESPACE}"

    ${CUB} function do --space inventory-api-prod \
      --unit inventory-api \
      --change-desc "setup: use local image for real cluster delivery" \
      --quiet \
      set-image inventory-api "${LOCAL_IMAGE}"
    echo "  Updated image: ${LOCAL_IMAGE}"
    echo "  Done."
    echo ""

    echo "Phase 5: Binding prod unit to real Kubernetes target..."
    ${CUB} unit set-target "${WORKER_SPACE}/${K8S_TARGET}" \
      --space inventory-api-prod \
      --unit inventory-api \
      --quiet
    echo "  Bound: inventory-api-prod/inventory-api -> ${WORKER_SPACE}/${K8S_TARGET}"
    echo "  Done."
    echo ""

    echo "Phase 6: Applying prod unit to cluster..."
    ${CUB} unit apply --space inventory-api-prod inventory-api --quiet
    echo "  Applied: inventory-api-prod/inventory-api"
    echo ""
    echo "  Waiting for deployment to be ready..."
    kubectl rollout status deployment/inventory-api -n "${DEPLOY_NAMESPACE}" --timeout=120s || {
      echo "  warning: Deployment not ready within timeout." >&2
    }
    echo "  Done."
    echo ""
    ;;

  "noop")
    echo "Phase 3: Creating infra space and server worker..."

    ${CUB} space create "${INFRA_SPACE}" \
      --label "ExampleName=${EXAMPLE_LABEL}" \
      --label "Role=infra" \
      --allow-exists \
      --quiet
    echo "  Created infra space: ${INFRA_SPACE}"

    ${CUB} worker create worker --space "${INFRA_SPACE}" --quiet --is-server-worker \
      --allow-exists 2>/dev/null || true
    echo "  Created server worker: ${INFRA_SPACE}/worker"

    echo "  Done."
    echo ""

    echo "Phase 4: Creating Noop targets and binding units..."

    for env in "${ENVS[@]}"; do
      space="$(space_name "${env}")"

      ${CUB} target create "${env}" '{}' "${INFRA_SPACE}/worker" -p Noop \
        --space "${space}" \
        --label "ExampleName=${EXAMPLE_LABEL}" \
        --label "Environment=${env}" \
        --allow-exists \
        --quiet
      echo "  Created Noop target: ${space}/${env}"

      ${CUB} unit set-target "${env}" --space "${space}" --unit inventory-api --quiet
      echo "  Bound unit: ${space}/inventory-api -> ${env}"
    done

    echo "  Done."
    echo ""

    echo "Phase 5: Applying units..."

    for env in "${ENVS[@]}"; do
      space="$(space_name "${env}")"
      ${CUB} unit apply --space "${space}" inventory-api --quiet
      echo "  Applied: ${space}/inventory-api"
    done

    echo "  Done."
    echo ""
    ;;
esac

echo "=== Setup complete ==="
echo ""
echo "Inspect with:"
echo "  ${CUB} space list --where \"Labels.ExampleName = '${EXAMPLE_LABEL}'\" --json | jq '.[].Space.Slug'"
echo "  ${CUB} unit get --space inventory-api-prod --json inventory-api"
echo ""
echo "Clean up with:"
echo "  ./cleanup.sh"
