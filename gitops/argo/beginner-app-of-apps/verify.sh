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
if [[ "$name" != "gitops-argo-beginner-app-of-apps" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

echo "==> Checking the root Application points at this example's apps/ directory"
grep -q "path: gitops/argo/beginner-app-of-apps/apps$" "$SCRIPT_DIR/root/root-app.yaml"

for env in dev prod; do
  child="$SCRIPT_DIR/apps/apptique-${env}.yaml"
  echo "==> Checking child Application apptique-${env}"
  grep -q "^kind: Application$" "$child"
  grep -q "  name: apptique-${env}$" "$child"
  grep -q "path: gitops/argo/beginner-app-of-apps/manifests/apptique/${env}$" "$child"
  grep -q "namespace: apptique-${env}$" "$child"
  grep -q "CreateNamespace=true" "$child"

  echo "==> Collecting the ${env} manifests the child Application points at"
  out="$VAR_DIR/rendered-${env}.yaml"
  : > "$out"
  for f in "$SCRIPT_DIR/manifests/apptique/${env}"/*.yaml; do
    echo "---" >> "$out"
    cat "$f" >> "$out"
  done
  for kind in Deployment Service ServiceAccount; do
    if ! grep -q "^kind: ${kind}$" "$out"; then
      echo "Missing kind ${kind} in the ${env} render" >&2
      exit 1
    fi
  done
  if ! grep -q "environment: ${env}$" "$out"; then
    echo "The ${env} render is not labelled environment: ${env}" >&2
    exit 1
  fi
done

echo "==> Checking prod replica count differs from dev"
dev_replicas="$(awk '/^kind: Deployment$/{f=1} f && /replicas:/{print $2; exit}' "$VAR_DIR/rendered-dev.yaml")"
prod_replicas="$(awk '/^kind: Deployment$/{f=1} f && /replicas:/{print $2; exit}' "$VAR_DIR/rendered-prod.yaml")"
if [[ "$dev_replicas" == "$prod_replicas" ]]; then
  echo "Expected dev and prod replica counts to differ, both were $dev_replicas" >&2
  exit 1
fi

echo "All gitops-argo-beginner-app-of-apps checks passed."
