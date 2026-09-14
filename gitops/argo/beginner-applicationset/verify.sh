#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

usage() {
  echo "Usage: ./verify.sh" >&2
  echo "Runs offline checks only: no cluster and no ConfigHub calls." >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

require_cmd kustomize
require_cmd jq

mkdir -p "$VAR_DIR"

echo "==> Checking that setup.sh --explain-json is valid JSON with the expected fields"
EXPLAIN_JSON_OUT="$("$SCRIPT_DIR/setup.sh" --explain-json)"
echo "$EXPLAIN_JSON_OUT" | jq . >/dev/null

for field in example_name mutates mutates_confighub mutates_live_infra spaces units evaluation_modes; do
  if ! echo "$EXPLAIN_JSON_OUT" | jq -e "has(\"$field\")" >/dev/null; then
    echo "setup.sh --explain-json is missing field: $field" >&2
    exit 1
  fi
done

name="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.example_name')"
if [[ "$name" != "gitops-argo-beginner-applicationset" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

echo "==> Rendering the dev overlay with kustomize build"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/dev" > "$VAR_DIR/rendered-dev.yaml"

echo "==> Rendering the prod overlay with kustomize build"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/prod" > "$VAR_DIR/rendered-prod.yaml"

check_resource() {
  local file="$1"
  local kind="$2"
  local name="$3"
  local namespace="$4"
  if ! grep -q "^kind: ${kind}\$" "$file"; then
    echo "Missing kind ${kind} in ${file}" >&2
    exit 1
  fi
  if ! grep -q "  name: ${name}\$" "$file"; then
    echo "Missing resource named ${name} in ${file}" >&2
    exit 1
  fi
  if ! grep -q "  namespace: ${namespace}\$" "$file"; then
    echo "Missing namespace ${namespace} reference in ${file}" >&2
    exit 1
  fi
}

echo "==> Checking expected resources in the dev render"
grep -q "^kind: Namespace\$" "$VAR_DIR/rendered-dev.yaml"
grep -q "  name: apptique-dev\$" "$VAR_DIR/rendered-dev.yaml"
check_resource "$VAR_DIR/rendered-dev.yaml" Deployment frontend apptique-dev
check_resource "$VAR_DIR/rendered-dev.yaml" Service frontend apptique-dev

echo "==> Checking expected resources in the prod render"
grep -q "^kind: Namespace\$" "$VAR_DIR/rendered-prod.yaml"
grep -q "  name: apptique-prod\$" "$VAR_DIR/rendered-prod.yaml"
check_resource "$VAR_DIR/rendered-prod.yaml" Deployment frontend apptique-prod
check_resource "$VAR_DIR/rendered-prod.yaml" Service frontend apptique-prod

echo "==> Checking prod replica count differs from dev"
dev_replicas="$(awk '/^kind: Deployment$/{f=1} f && /replicas:/{print $2; exit}' "$VAR_DIR/rendered-dev.yaml")"
prod_replicas="$(awk '/^kind: Deployment$/{f=1} f && /replicas:/{print $2; exit}' "$VAR_DIR/rendered-prod.yaml")"
if [[ "$dev_replicas" == "$prod_replicas" ]]; then
  echo "Expected dev and prod replica counts to differ, both were $dev_replicas" >&2
  exit 1
fi

echo "==> Checking bootstrap ApplicationSet references this example's overlays"
grep -q "gitops/argo/beginner-applicationset/apps/apptique/overlays" "$SCRIPT_DIR/bootstrap/applicationset.yaml"

echo "All gitops-argo-beginner-applicationset checks passed."
