#!/bin/bash
# Shared variables and helpers for ConfigHub demo setup
# Compatible with bash 3.2+ (macOS default)

set -euo pipefail

EXAMPLE_NAME="demo-data"
CUB="${CUB:-cub}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Components and their metadata
APPS=(aichat website docs eshop portal platform)

# Targets (deployment destinations)
TARGETS=(us-dev-1 us-dev-2 us-qa-1 us-staging-1 eu-staging-1 us-prod-1 eu-prod-1)

# Lookup functions (bash 3.2 doesn't support associative arrays)

target_env() {
  case "$1" in
    us-dev-*)     echo "dev" ;;
    eu-dev-*)     echo "dev" ;;
    us-qa-*)      echo "qa" ;;
    eu-qa-*)      echo "qa" ;;
    us-staging-*) echo "staging" ;;
    eu-staging-*) echo "staging" ;;
    us-prod-*)    echo "prod" ;;
    eu-prod-*)    echo "prod" ;;
  esac
}

target_region() {
  case "$1" in
    us-*)  echo "us" ;;
    eu-*)  echo "eu" ;;
  esac
}

env_replicas() {
  case "$1" in
    dev)     echo "1" ;;
    qa)      echo "1" ;;
    staging) echo "2" ;;
    prod)    echo "3" ;;
  esac
}

env_log_level() {
  case "$1" in
    dev)     echo "debug" ;;
    qa)      echo "debug" ;;
    staging) echo "info" ;;
    prod)    echo "warn" ;;
  esac
}

target_role() {
  case "$1" in
    dev)     echo "Dev" ;;
    qa)      echo "QA" ;;
    staging) echo "Staging" ;;
    prod)    echo "Prod" ;;
  esac
}

region_label() {
  case "$1" in
    us) echo "US" ;;
    eu) echo "EU" ;;
  esac
}

app_dept() {
  case "$1" in
    website)  echo "Marketing" ;;
    docs)     echo "Product" ;;
    eshop)    echo "Product" ;;
    portal)   echo "Support" ;;
    aichat)   echo "Support" ;;
    platform) echo "Platform" ;;
  esac
}

app_team() {
  case "$1" in
    aichat)   echo "AI" ;;
    website)  echo "Web" ;;
    docs)     echo "DevEx" ;;
    eshop)    echo "Commerce" ;;
    portal)   echo "Portal" ;;
    platform) echo "Platform" ;;
  esac
}

WORKER_SPACE="demo-infra"

# Create the shared worker space
create_worker_space() {
  $CUB space create "$WORKER_SPACE" \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "Owner=Platform" \
    --quiet

  $CUB worker create worker --space "$WORKER_SPACE" \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --quiet --is-server-worker

  echo "  Created worker space: $WORKER_SPACE"
}

# Create an infrastructure space with target (worker lives in $WORKER_SPACE)
create_infra_space() {
  local target="$1"
  local env
  env=$(target_env "$target")
  local region
  region=$(target_region "$target")

  $CUB space create "$target" \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "Owner=Platform" \
    --label "Variant=${target}" \
    --label "TargetRole=$(target_role "$env")" \
    --label "TargetRegion=$(region_label "$region")" \
    --quiet

  $CUB target create "$target" '{}' "${WORKER_SPACE}/worker" -p Noop --space "$target" \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "DisplayName=$(region_label "$region") - $(target_role "$env")" \
    --label "TargetRole=$(target_role "$env")" \
    --label "TargetRegion=$(region_label "$region")" \
    --quiet

  echo "  Created infra space: $target (target)"
}

# Create a component deployment space with labels
create_app_space() {
  local target="$1"
  local app="$2"
  local space="${target}-${app}"
  local env
  env=$(target_env "$target")
  local region
  region=$(target_region "$target")
  $CUB space create "$space" \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "Component=${app}" \
    --label "Owner=$(app_dept "$app")" \
    --label "Team=$(app_team "$app")" \
    --label "Variant=${target}" \
    --label "TargetRole=$(target_role "$env")" \
    --label "TargetRegion=$(region_label "$region")" \
    --quiet

  echo "  Created component space: $space"
}
