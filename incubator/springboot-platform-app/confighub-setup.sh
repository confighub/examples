#!/usr/bin/env bash
# ConfigHub setup for springboot-platform-app
#
# Creates ConfigHub spaces and units for inventory-api across dev, stage, prod.
#
# Usage:
#   ./confighub-setup.sh --explain              # Human-readable preview (read-only)
#   ./confighub-setup.sh --explain-json         # Machine-readable preview (read-only)
#   ./confighub-setup.sh                        # ConfigHub-only (spaces + units)
#   ./confighub-setup.sh --with-targets         # Real Kubernetes deployment (requires cluster + worker)
#   ./confighub-setup.sh --with-noop-targets    # Noop targets for simulation (no cluster needed)
#
# For --with-targets, you need:
#   export WORKER_SPACE=springboot-infra        # Space where the Kubernetes worker lives
#   export K8S_TARGET=<printed target slug>     # Optional if WORKER_SPACE has exactly one Kubernetes target
#
# Cleanup:
#   ./confighub-cleanup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"
INFRA_SPACE="inventory-api-infra"
DEPLOY_NAMESPACE="inventory-api"
LOCAL_IMAGE="inventory-api:local"
DEFAULT_KUBECONFIG_PATH="${SCRIPT_DIR}/var/springboot-platform.kubeconfig"

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
      echo "Run ./bin/install-worker first or export the exact target slug it printed." >&2
      exit 1
    fi
    return 0
  fi

  k8s_target_count="$(echo "${target_json}" | jq '[.[] | select((.Target.ProviderType | ascii_downcase) == "kubernetes")] | length')"
  case "${k8s_target_count}" in
    0)
      echo "error: No Kubernetes targets found in space '${WORKER_SPACE}'." >&2
      echo "Run ./bin/install-worker first." >&2
      exit 1
      ;;
    1)
      K8S_TARGET="$(echo "${target_json}" | jq -r '[.[] | select((.Target.ProviderType | ascii_downcase) == "kubernetes")][0].Target.Slug')"
      echo "Auto-detected Kubernetes target: ${WORKER_SPACE}/${K8S_TARGET}"
      ;;
    *)
      echo "error: Multiple Kubernetes targets found in space '${WORKER_SPACE}'." >&2
      echo "Export K8S_TARGET with the exact target slug to use." >&2
      echo "${target_json}" | jq -r '.[] | select((.Target.ProviderType | ascii_downcase) == "kubernetes") | "  - \(.Target.Slug)"' >&2
      exit 1
      ;;
  esac
}

# --explain: human-readable preview
show_explain() {
  local mode="${1:-confighub-only}"
  local target_hint="${K8S_TARGET:-<auto-detected-kubernetes-target>}"
  cat <<EOF
confighub-setup: springboot-platform-app

Mode: ${mode}

What it creates:
- 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
- 1 unit per space: inventory-api (ConfigMap + Deployment + Service)
- labels: ExampleName=springboot-platform-app, App=inventory-api, Environment=<env>
EOF
  case "${mode}" in
    "real-targets")
      cat <<EOF

REAL KUBERNETES DEPLOYMENT:
- Requires: Kind cluster + ConfigHub worker (./bin/create-cluster, ./bin/install-worker)
- Creates namespace '${DEPLOY_NAMESPACE}' in the cluster
- Deploys ONLY prod environment to real Kubernetes
- Dev and stage remain ConfigHub-only (for comparison demos)
- Uses image: ${LOCAL_IMAGE} (build with ./bin/build-image)
- Binds prod unit to real target: \${WORKER_SPACE}/${target_hint}
- Apply triggers real kubectl apply via the worker

Required env vars:
  WORKER_SPACE=<space-where-worker-lives>
  K8S_TARGET=<target-slug>  (optional; auto-detected if exactly one Kubernetes target exists)

Mutating commands:
- cub space create
- cub unit create
- cub function do (update image)
- cub unit set-target
- cub unit apply
- kubectl create namespace
EOF
      ;;
    "noop-targets")
      cat <<EOF

NOOP TARGETS (simulation):
- 1 infra space: inventory-api-infra (server worker)
- 1 Noop target per env space (no cluster required)
- Units are bound to targets and applied
- Noop worker accepts apply but does NOT deliver to Kubernetes

This proves the ConfigHub mutation-to-apply workflow without a real cluster.

Mutating commands:
- cub space create
- cub unit create
- cub worker create (server worker)
- cub target create (Noop)
- cub unit set-target
- cub unit apply
EOF
      ;;
    *)
      cat <<EOF

This does NOT create targets, workers, or cluster bindings.
Use --with-targets for real Kubernetes deployment.
Use --with-noop-targets for simulation without a cluster.
EOF
      ;;
  esac
  cat <<'EOF'

Cleanup:
- ./confighub-cleanup.sh
EOF
}

# --explain-json: machine-readable preview
show_explain_json() {
  local mode="${1:-confighub-only}"
  case "${mode}" in
    "real-targets")
      cat <<ENDJSON
{
  "example_name": "springboot-platform-app",
  "proof_type": "real-kubernetes-deployment",
  "mutates_confighub": true,
  "mutates_live_infra": true,
  "requires_cluster": true,
  "with_targets": true,
  "target_provider": "Kubernetes",
  "deploy_namespace": "${DEPLOY_NAMESPACE}",
  "deployed_environments": ["prod"],
  "confighub_only_environments": ["dev", "stage"],
  "spaces_created": [
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": ["inventory-api"],
  "required_env_vars": ["WORKER_SPACE"],
  "optional_env_vars": ["K8S_TARGET"],
  "cleanup": "./confighub-cleanup.sh"
}
ENDJSON
      ;;
    "noop-targets")
      cat <<ENDJSON
{
  "example_name": "springboot-platform-app",
  "proof_type": "confighub-only+noop-targets",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "requires_cluster": false,
  "with_targets": true,
  "target_provider": "Noop",
  "spaces_created": [
    "inventory-api-infra",
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": ["inventory-api"],
  "targets_per_space": ["dev", "stage", "prod"],
  "cleanup": "./confighub-cleanup.sh"
}
ENDJSON
      ;;
    *)
      cat <<ENDJSON
{
  "example_name": "springboot-platform-app",
  "proof_type": "confighub-only",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "requires_cluster": false,
  "with_targets": false,
  "spaces_created": [
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": ["inventory-api"],
  "cleanup": "./confighub-cleanup.sh"
}
ENDJSON
      ;;
  esac
}

MODE="confighub-only"

case "${1:-}" in
  --explain)
    case "${2:-}" in
      --with-targets) show_explain "real-targets" ;;
      --with-noop-targets) show_explain "noop-targets" ;;
      *) show_explain "confighub-only" ;;
    esac
    exit 0
    ;;
  --explain-json)
    case "${2:-}" in
      --with-targets) show_explain_json "real-targets" ;;
      --with-noop-targets) show_explain_json "noop-targets" ;;
      *) show_explain_json "confighub-only" ;;
    esac
    exit 0
    ;;
  --with-targets)
    MODE="real-targets"
    ;;
  --with-noop-targets)
    MODE="noop-targets"
    ;;
  "")
    ;;
  *)
    echo "Usage: $0 [--explain|--explain-json] [--with-targets|--with-noop-targets]" >&2
    echo "       $0 [--with-targets|--with-noop-targets]" >&2
    exit 2
    ;;
esac

# Mutating path: require cub
command -v "${CUB}" >/dev/null 2>&1 || {
  echo "error: cub CLI not found. Install cub and run cub auth login first." >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || {
  echo "error: jq not found." >&2
  exit 1
}

# Additional checks for real-targets mode
if [[ "${MODE}" == "real-targets" ]]; then
  if [[ -z "${WORKER_SPACE}" ]]; then
    echo "error: WORKER_SPACE environment variable not set." >&2
    echo "" >&2
    echo "For real Kubernetes deployment, you need:" >&2
    echo "  1. ./bin/create-cluster" >&2
    echo "  2. ./bin/build-image" >&2
    echo "  3. CUB_SPACE=springboot-infra ./bin/install-worker" >&2
    echo "  4. export WORKER_SPACE=springboot-infra" >&2
    echo "  5. ./confighub-setup.sh --with-targets" >&2
    exit 1
  fi

  command -v kubectl >/dev/null 2>&1 || {
    echo "error: kubectl not found (required for --with-targets)." >&2
    exit 1
  }

  if [[ -z "${KUBECONFIG:-}" && -f "${DEFAULT_KUBECONFIG_PATH}" ]]; then
    export KUBECONFIG="${DEFAULT_KUBECONFIG_PATH}"
    echo "Using kubeconfig: ${KUBECONFIG}"
  fi

  # Check cluster is reachable
  if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "error: Kubernetes cluster not reachable." >&2
    echo "Run ./bin/create-cluster first, then export KUBECONFIG." >&2
    exit 1
  fi

  resolve_k8s_target
fi

case "${MODE}" in
  "real-targets")
    echo "=== ConfigHub setup with REAL Kubernetes deployment ==="
    echo ""
    echo "Worker space: ${WORKER_SPACE}"
    echo "Target:       ${K8S_TARGET}"
    echo "Namespace:    ${DEPLOY_NAMESPACE}"
    echo "Deployed:     prod only (dev/stage are ConfigHub-only)"
    ;;
  "noop-targets")
    echo "=== ConfigHub setup with Noop targets (simulation) ==="
    ;;
  *)
    echo "=== ConfigHub-only setup for springboot-platform-app ==="
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

# Phase 2: Create units from YAML
echo "Phase 2: Creating units from operational YAML..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  yaml_file="${SCRIPT_DIR}/confighub/inventory-api-${env}.yaml"

  if [[ ! -f "${yaml_file}" ]]; then
    echo "  error: missing ${yaml_file}" >&2
    exit 1
  fi

  ${CUB} unit create --space "${space}" inventory-api "${yaml_file}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --label "Component=backend" \
    --allow-exists \
    --quiet
  echo "  Created unit: ${space}/inventory-api"
done

echo "  Done."
echo ""

# Phase 3+: Mode-specific setup
case "${MODE}" in
  "real-targets")
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
      echo "  warning: Deployment not ready within timeout. Check:" >&2
      echo "    kubectl get pods -n ${DEPLOY_NAMESPACE}" >&2
      echo "    kubectl describe deployment inventory-api -n ${DEPLOY_NAMESPACE}" >&2
    }
    echo "  Done."
    echo ""
    ;;

  "noop-targets")
    echo "Phase 3: Creating infra space and server worker..."

    ${CUB} space create "${INFRA_SPACE}" \
      --label "ExampleName=${EXAMPLE_LABEL}" \
      --label "AppOwner=Platform" \
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
echo "  ${CUB} space list --where \"Labels.ExampleName = '${EXAMPLE_LABEL}'\" --json"
echo "  ${CUB} unit get --space inventory-api-prod --json inventory-api"

if [[ "${MODE}" == "real-targets" ]]; then
  echo ""
  echo "Verify deployment:"
  echo "  kubectl get pods -n ${DEPLOY_NAMESPACE}"
  echo "  ./verify-e2e.sh"
fi

echo ""
echo "Clean up with:"
echo "  ./confighub-cleanup.sh"
if [[ "${MODE}" == "real-targets" ]]; then
  echo "  ./bin/teardown"
fi
