#!/bin/bash
# ConfigHub Demo Setup
#
# Populates an empty org with a realistic (but fake) multi-app, multi-environment dataset.
# Uses the App-Deployment-Target model with labels on top of ConfigHub's space/unit model.
#
# Prerequisites:
#   - Latest cub CLI authenticated (cub auth login --server <url>)
#   - An empty org (or one where you don't mind adding demo data)
#
# Usage:
#   ./setup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

echo "=== ConfigHub Demo Setup ==="
echo ""
echo "This will create 50 spaces and ~154 units in your org."
echo "All entities are labeled ExampleName=${EXAMPLE_NAME} for easy cleanup."
echo ""

##################################
# Phase 1: Infrastructure spaces
##################################
echo "Phase 1: Creating shared worker and infrastructure spaces..."

create_worker_space

for target in "${TARGETS[@]}"; do
  create_infra_space "$target"
done

echo "  Done. Created 1 worker space + ${#TARGETS[@]} infrastructure spaces."
echo ""

##################################
# Phase 2: App deployment spaces
##################################
echo "Phase 2: Creating app deployment spaces..."

count=0
for target in "${TARGETS[@]}"; do
  for app in "${APPS[@]}"; do
    create_app_space "$target" "$app"
    count=$((count + 1))
  done
done

echo "  Done. Created ${count} app spaces."
echo ""

##################################
# Phase 3: Create units in dev
##################################
echo "Phase 3: Creating units in dev spaces from YAML templates..."

for app in "${APPS[@]}"; do
  dev_space="us-dev-1-${app}"
  for yaml_file in "${SCRIPT_DIR}/config-data/${app}"/*.yaml; do
    unit_slug=$(basename "$yaml_file" .yaml)
    $CUB unit create --space "$dev_space" "$unit_slug" "$yaml_file" \
      --target "us-dev-1/us-dev-1" --quiet --wait=false
    echo "  Created unit: ${dev_space}/${unit_slug}"
  done
done

echo "  Done."
echo ""

##################################
# Phase 4: Clone to other envs
##################################
# Clone hierarchy: dev -> {dev-2, qa, staging} -> prod
echo "Phase 4a: Cloning dev units to dev-2, qa, and staging..."

for target in us-dev-2 us-qa-1 us-staging-1 eu-staging-1; do
  for app in "${APPS[@]}"; do
    dev_space="us-dev-1-${app}"
    dest_space="${target}-${app}"
    $CUB unit create --space "$dev_space" --dest-space "$dest_space" \
      --target "${target}/${target}" --quiet --wait=false
    echo "  Cloned ${app} -> ${dest_space} (target: ${target}/${target})"
  done
done

echo "  Done."
echo ""

echo "Phase 4b: Cloning staging units to prod..."

for app in "${APPS[@]}"; do
  $CUB unit create --space "us-staging-1-${app}" --dest-space "us-prod-1-${app}" \
    --target "us-prod-1/us-prod-1" --quiet --wait=false
  echo "  Cloned ${app} -> us-prod-1-${app} (from us-staging-1)"
  $CUB unit create --space "eu-staging-1-${app}" --dest-space "eu-prod-1-${app}" \
    --target "eu-prod-1/eu-prod-1" --quiet --wait=false
  echo "  Cloned ${app} -> eu-prod-1-${app} (from eu-staging-1)"
done

echo "  Done."
echo ""

##################################
# Phase 4c: Label units
##################################
echo "Phase 4c: Labeling units..."

for target in "${TARGETS[@]}"; do
  env=$(target_env "$target")
  region=$(target_region "$target")
  for app in "${APPS[@]}"; do
    space="${target}-${app}"
    $CUB unit update --space "$space" --where "Slug LIKE '%'" --patch --quiet \
      --label "ExampleName=${EXAMPLE_NAME}" \
      --label "App=${app}" \
      --label "AppOwner=$(app_dept "$app")" \
      --label "TargetRole=$(target_role "$env")" \
      --label "TargetRegion=$(region_label "$region")"
    echo "  Labeled units in: $space"
  done
done

echo "  Done."
echo ""

##################################
# Phase 5: Per-environment customization
##################################
echo "Phase 5: Customizing per-environment..."

for target in "${TARGETS[@]}"; do
  env=$(target_env "$target")
  region=$(target_region "$target")
  replicas=$(env_replicas "$env")
  log_level=$(env_log_level "$env")

  for app in "${APPS[@]}"; do
    space="${target}-${app}"

    # Set namespace on all resources
    $CUB function do set-namespace "$space" --space "$space" --quiet 2>/dev/null || true

    # Set hostname on ingress-bearing units
    $CUB function do set-hostname "${space}.demo.confighub.io" --space "$space" --quiet 2>/dev/null || true

    # Set replicas
    $CUB function do set-replicas "$replicas" --space "$space" --quiet 2>/dev/null || true

    # Set environment variables
    $CUB function do set-env-var '*' ENVIRONMENT "$env" --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-env-var '*' REGION "$region" --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-env-var '*' LOG_LEVEL "$log_level" --space "$space" --quiet 2>/dev/null || true

    echo "  Customized: ${space} (env=${env}, region=${region}, replicas=${replicas})"
  done
done

echo "  Done."
echo ""

##################################
# Phase 6: Prod adjustments
##################################
echo "Phase 6: Applying prod-specific resource scaling..."

for target in us-prod-1 eu-prod-1; do
  for app in "${APPS[@]}"; do
    space="${target}-${app}"

    # Scale up CPU/memory for prod Deployments
    $CUB function do set-string-path apps/v1/Deployment \
      "spec.template.spec.containers.0.resources.requests.cpu" 500m \
      --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-string-path apps/v1/Deployment \
      "spec.template.spec.containers.0.resources.requests.memory" 512Mi \
      --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-string-path apps/v1/Deployment \
      "spec.template.spec.containers.0.resources.limits.cpu" 2 \
      --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-string-path apps/v1/Deployment \
      "spec.template.spec.containers.0.resources.limits.memory" 1Gi \
      --space "$space" --quiet 2>/dev/null || true

    # Scale up StatefulSets too
    $CUB function do set-string-path apps/v1/StatefulSet \
      "spec.template.spec.containers.0.resources.requests.cpu" 500m \
      --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-string-path apps/v1/StatefulSet \
      "spec.template.spec.containers.0.resources.requests.memory" 512Mi \
      --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-string-path apps/v1/StatefulSet \
      "spec.template.spec.containers.0.resources.limits.cpu" 2 \
      --space "$space" --quiet 2>/dev/null || true
    $CUB function do set-string-path apps/v1/StatefulSet \
      "spec.template.spec.containers.0.resources.limits.memory" 1Gi \
      --space "$space" --quiet 2>/dev/null || true

    echo "  Scaled resources: ${space}"
  done
done

echo ""

##################################
# Phase 7: Upgrade and apply all units
##################################
echo "Phase 7: Upgrading all units to upstream head revision..."

$CUB unit update --space '*' \
  --where "Labels.ExampleName = '${EXAMPLE_NAME}' AND UpstreamUnitID IS NOT NULL" \
  --upgrade --patch --quiet

echo "  Done."
echo ""

echo "Phase 7b: Applying all units (noop worker — no real effect)..."

$CUB unit apply --space '*' \
  --where "Labels.ExampleName = '${EXAMPLE_NAME}' AND TargetID IS NOT NULL" \
  --wait=false

echo "  Done. Units are being applied in the background."
echo ""

##################################
# Phase 8: Create intentional version skew
##################################
# This runs AFTER upgrade+apply so the skew is the only unapplied change.
echo "Phase 8: Creating version skew for eshop..."

$CUB function do set-image-reference api ":4.2.0" \
  --space us-dev-1-eshop --unit api --quiet
$CUB function do set-image-reference worker ":4.2.0" \
  --space us-dev-1-eshop --unit worker --quiet
echo "  Set eshop api+worker to :4.2.0 in us-dev-1 (vs :4.2.1 elsewhere)"
echo ""

echo "=== Demo setup complete ==="
echo ""
echo "Summary:"
echo "  Infrastructure spaces: ${#TARGETS[@]}"
echo "  App deployment spaces: ${count}"
echo "  Total spaces: $(( ${#TARGETS[@]} + count ))"
echo ""
echo "Explore with:"
echo "  $CUB space list --where \"Labels.ExampleName = '${EXAMPLE_NAME}'\""
echo "  $CUB space list --where \"Labels.App = 'aichat'\""
echo "  $CUB space list --where \"Labels.TargetRole = 'Prod'\""
echo "  $CUB space list --where \"Labels.AppOwner = 'Product'\""
echo ""
echo "Clean up with:"
echo "  ./cleanup.sh"
