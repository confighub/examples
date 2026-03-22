#!/usr/bin/env bash
# ConfigHub setup for springboot-platform-app
#
# Creates ConfigHub spaces and units for inventory-api across dev, stage, prod.
#
# Usage:
#   ./confighub-setup.sh --explain          # Human-readable preview (read-only)
#   ./confighub-setup.sh --explain-json     # Machine-readable preview (read-only)
#   ./confighub-setup.sh                    # ConfigHub-only (spaces + units)
#   ./confighub-setup.sh --with-targets     # + infra space, Noop targets, apply
#
# Cleanup:
#   ./confighub-cleanup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"
INFRA_SPACE="inventory-api-infra"

ENVS=(dev stage prod)

space_name() {
  echo "inventory-api-${1}"
}

# --explain: human-readable preview
show_explain() {
  local with_targets="${1:-false}"
  local proof_label="confighub-only"
  if [[ "${with_targets}" == "true" ]]; then
    proof_label="confighub-only + Noop targets"
  fi
  cat <<EOF
confighub-setup: springboot-platform-app

Proof type: ${proof_label}

What it creates:
- 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
- 1 unit per space: inventory-api (ConfigMap + Deployment + Service)
- labels: ExampleName=springboot-platform-app, App=inventory-api, Environment=<env>
EOF
  if [[ "${with_targets}" == "true" ]]; then
    cat <<'EOF'
- 1 infra space: inventory-api-infra (server worker)
- 1 Noop target per env space (no cluster required)
- units are bound to targets and applied

This proves the full ConfigHub mutation-to-apply workflow using
Noop targets (no real cluster). The Noop worker accepts the apply
but does not deliver to Kubernetes.
EOF
  else
    cat <<'EOF'

This does NOT create targets, workers, or cluster bindings.
Use --with-targets to add Noop targets and prove the apply workflow.
EOF
  fi
  cat <<'EOF'

Mutating commands used:
- cub space create
- cub unit create
EOF
  if [[ "${with_targets}" == "true" ]]; then
    cat <<'EOF'
- cub worker create (server worker)
- cub target create (Noop)
- cub unit set-target
- cub unit apply
EOF
  fi
  cat <<'EOF'

Cleanup:
- ./confighub-cleanup.sh
EOF
}

# --explain-json: machine-readable preview
show_explain_json() {
  local with_targets="${1:-false}"
  if [[ "${with_targets}" == "true" ]]; then
    cat <<'ENDJSON'
{
  "example_name": "springboot-platform-app",
  "proof_type": "confighub-only+noop-targets",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "requires_cluster": false,
  "with_targets": true,
  "spaces_created": [
    "inventory-api-infra",
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": ["inventory-api"],
  "targets_per_space": ["dev", "stage", "prod"],
  "target_provider": "Noop",
  "cleanup": "./confighub-cleanup.sh"
}
ENDJSON
  else
    cat <<'ENDJSON'
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
  fi
}

WITH_TARGETS=false

case "${1:-}" in
  --explain)
    if [[ "${2:-}" == "--with-targets" ]]; then
      show_explain "true"
    else
      show_explain "false"
    fi
    exit 0
    ;;
  --explain-json)
    if [[ "${2:-}" == "--with-targets" ]]; then
      show_explain_json "true"
    else
      show_explain_json "false"
    fi
    exit 0
    ;;
  --with-targets)
    WITH_TARGETS=true
    ;;
  "")
    ;;
  *)
    echo "Usage: $0 [--explain [--with-targets]|--explain-json [--with-targets]|--with-targets]" >&2
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

if [[ "${WITH_TARGETS}" == "true" ]]; then
  echo "=== ConfigHub setup with Noop targets for springboot-platform-app ==="
else
  echo "=== ConfigHub-only setup for springboot-platform-app ==="
fi
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

# Phase 3: Noop targets (only with --with-targets)
if [[ "${WITH_TARGETS}" == "true" ]]; then
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
fi

echo "=== Setup complete ==="
echo ""
echo "Inspect with:"
echo "  ${CUB} space list --where \"Labels.ExampleName = '${EXAMPLE_LABEL}'\" --json"
echo "  ${CUB} unit get --space inventory-api-prod --json inventory-api"
echo ""
echo "Clean up with:"
echo "  ./confighub-cleanup.sh"
