#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GITOPS_DIR="$SCRIPT_DIR/gitops-repo"
VAR_DIR="$SCRIPT_DIR/var"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

usage() {
  echo "Usage: ./verify.sh" >&2
  echo "Runs offline checks only: no cluster, no build, no pull request, no ConfigHub calls." >&2
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
require_cmd kustomize

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
if [[ "$name" != "gitops-argo-intermediate-ci-to-gitops" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

for env in dev prod; do
  app="$GITOPS_DIR/applications/apptique-${env}.yaml"
  echo "==> Checking Application apptique-${env}"
  grep -q "^kind: Application$" "$app"
  grep -q "  name: apptique-${env}$" "$app"
  grep -q "path: gitops/argo/intermediate-ci-to-gitops/gitops-repo/environments/apptique/${env}$" "$app"
  grep -q "namespace: apptique-${env}$" "$app"
  grep -q "CreateNamespace=true" "$app"

  echo "==> Rendering environments/apptique/${env} with kustomize build"
  out="$VAR_DIR/rendered-${env}.yaml"
  kustomize build "$GITOPS_DIR/environments/apptique/${env}" > "$out"
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

image_tag_of() {
  grep -m1 "image: ghcr.io/confighub/apptique-frontend:" "$1" | sed 's/.*apptique-frontend://'
}

echo "==> Checking dev and prod image tags differ (prod is one promotion behind dev)"
dev_tag="$(image_tag_of "$VAR_DIR/rendered-dev.yaml")"
prod_tag="$(image_tag_of "$VAR_DIR/rendered-prod.yaml")"
if [[ -z "$dev_tag" || -z "$prod_tag" ]]; then
  echo "Could not read an apptique-frontend image tag from one of the renders" >&2
  exit 1
fi
if [[ "$dev_tag" == "$prod_tag" ]]; then
  echo "Expected the dev and prod apptique image tags to differ, both were $dev_tag" >&2
  exit 1
fi

echo "==> Checking the prod image tag matches the last recorded promotion"
promoted_record="$GITOPS_DIR/environments/apptique/prod/PROMOTED_FROM.md"
promoted_tag="$(grep -m1 '^- promoted_tag:' "$promoted_record" | awk '{print $3}')"
if [[ -z "$promoted_tag" ]]; then
  echo "Could not read promoted_tag from $promoted_record" >&2
  exit 1
fi
if [[ "$prod_tag" != "$promoted_tag" ]]; then
  echo "prod image tag $prod_tag does not match the promotion record $promoted_tag in" >&2
  echo "${promoted_record#$GITOPS_DIR/}. Was the wrong tag merged?" >&2
  exit 1
fi

echo "All gitops-argo-intermediate-ci-to-gitops checks passed."
