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
  echo "Runs offline checks only: no cluster, no Argo CD, no Flux, no ConfigHub calls." >&2
  echo "Writes the rendered manifests to var/, which is not tracked by Git." >&2
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
require_cmd bash

mkdir -p "$VAR_DIR"

echo "==> Checking that setup.sh --explain-json is valid JSON with the expected fields"
EXPLAIN_JSON_OUT="$("$SCRIPT_DIR/setup.sh" --explain-json)"
echo "$EXPLAIN_JSON_OUT" | jq . >/dev/null

for field in example_name mutates mutates_confighub mutates_live_infra spaces components unit_per_resource resources apps scenarios uploaded_states not_uploaded_states namespaces evaluation_modes; do
  if ! echo "$EXPLAIN_JSON_OUT" | jq -e "has(\"$field\")" >/dev/null; then
    echo "setup.sh --explain-json is missing field: $field" >&2
    exit 1
  fi
done

name="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.example_name')"
if [[ "$name" != "gitops-expert-broken-states" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

uploaded="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.uploaded_states | join(",")')"
if [[ "$uploaded" != "healthy" ]]; then
  echo "Expected uploaded_states to be exactly [\"healthy\"], got: $uploaded" >&2
  exit 1
fi

echo "==> Checking that this example uses one Space per Component (three distinct Spaces)"
space_count="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.spaces | length')"
if [[ "$space_count" -ne 3 ]]; then
  echo "Expected exactly 3 Spaces (one per Component), got $space_count" >&2
  exit 1
fi
distinct_space_count="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.spaces | unique | length')"
if [[ "$distinct_space_count" -ne 3 ]]; then
  echo "Expected the 3 Spaces to be distinct, got $distinct_space_count distinct values" >&2
  exit 1
fi

echo "==> Checking that the resources listed in --explain-json match what each component renders"
# cub variant upload makes one Unit per rendered resource, so the listed
# resources are the Units each Space will hold.
kustomize build "$SCRIPT_DIR/argo" > "$VAR_DIR/rendered-argo-control.yaml"
kustomize build "$SCRIPT_DIR/flux" > "$VAR_DIR/rendered-flux-control.yaml"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/healthy" > "$VAR_DIR/rendered-healthy.yaml"
for pair in "argo-control:rendered-argo-control.yaml" "flux-control:rendered-flux-control.yaml" "apptique:rendered-healthy.yaml"; do
  component="${pair%%:*}"
  rendered="$VAR_DIR/${pair#*:}"
  listed="$(echo "$EXPLAIN_JSON_OUT" | jq -r --arg c "$component" '.resources[$c] | sort | join(",")')"
  actual="$(awk '/^kind: /{k=$2} /^  name: /{if (k!="") {print k "/" $2; k=""}}' "$rendered" | sort | paste -sd, -)"
  if [[ "$listed" != "$actual" ]]; then
    echo "setup.sh --explain-json lists resources [$listed] for $component but its render has [$actual]" >&2
    exit 1
  fi
done

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

echo "==> Rendering the Argo and Flux control objects"
kustomize build "$SCRIPT_DIR/argo" > "$VAR_DIR/rendered-argo-control.yaml"
kustomize build "$SCRIPT_DIR/flux" > "$VAR_DIR/rendered-flux-control.yaml"

echo "==> Checking the Argo Application points at the healthy overlay and self-heals"
grep -q "path: gitops/expert-broken-states/apps/apptique/overlays/healthy" "$VAR_DIR/rendered-argo-control.yaml"
grep -q "selfHeal: true" "$VAR_DIR/rendered-argo-control.yaml"

echo "==> Checking the Flux Kustomization points at the healthy overlay and health-checks frontend"
grep -q "path: ./gitops/expert-broken-states/apps/apptique/overlays/healthy" "$VAR_DIR/rendered-flux-control.yaml"
grep -q "name: frontend" "$VAR_DIR/rendered-flux-control.yaml"
grep -q "kind: GitRepository" "$VAR_DIR/rendered-flux-control.yaml"

echo "==> Rendering the healthy, failed-sync and bad-commit overlays"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/healthy" > "$VAR_DIR/rendered-healthy.yaml"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/failed-sync" > "$VAR_DIR/rendered-failed-sync.yaml"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/bad-commit" > "$VAR_DIR/rendered-bad-commit.yaml"

echo "==> Checking the healthy overlay: 2 replicas, port 80 end to end"
grep -q "replicas: 2" "$VAR_DIR/rendered-healthy.yaml"
grep -q "namespace: apptique-broken-states$" "$VAR_DIR/rendered-healthy.yaml"
healthy_container_port="$(awk '/^kind: Deployment$/{f=1} f && /containerPort:/{print $NF; exit}' "$VAR_DIR/rendered-healthy.yaml")"
healthy_target_port="$(awk '/^kind: Service$/{f=1} f && /targetPort:/{print $NF; exit}' "$VAR_DIR/rendered-healthy.yaml")"
if [[ "$healthy_container_port" != "80" || "$healthy_target_port" != "80" ]]; then
  echo "Expected the healthy overlay's container and Service to both use port 80, got container=$healthy_container_port service=$healthy_target_port" >&2
  exit 1
fi

echo "==> Checking the failed-sync overlay: the healthy app plus one RedisCache with no CRD assumed, in the same namespace"
if ! grep -q "namespace: apptique-broken-states$" "$VAR_DIR/rendered-failed-sync.yaml" \
  || grep -q "namespace: apptique-broken-states-" "$VAR_DIR/rendered-failed-sync.yaml"; then
  echo "Expected the failed-sync overlay to render into the healthy app's namespace, apptique-broken-states," >&2
  echo "which is the namespace scenarios/02-failed-sync diagnoses" >&2
  exit 1
fi
grep -q "^kind: RedisCache$" "$VAR_DIR/rendered-failed-sync.yaml"
grep -q "apiVersion: cache.apptique.example/v1" "$VAR_DIR/rendered-failed-sync.yaml"
redis_count="$(grep -c "^kind: RedisCache$" "$VAR_DIR/rendered-failed-sync.yaml")"
if [[ "$redis_count" -ne 1 ]]; then
  echo "Expected exactly 1 RedisCache resource in the failed-sync overlay, found $redis_count" >&2
  exit 1
fi
# diff exits 1 when the files differ, which is expected here.
failed_sync_diff="$(diff "$VAR_DIR/rendered-healthy.yaml" "$VAR_DIR/rendered-failed-sync.yaml" || true)"
removed_lines="$(printf '%s\n' "$failed_sync_diff" | grep -c '^<' || true)"
added_kinds="$(printf '%s\n' "$failed_sync_diff" | grep '^> kind: ' || true)"
if [[ "$removed_lines" -ne 0 || "$added_kinds" != "> kind: RedisCache" ]]; then
  echo "Expected the failed-sync render to be the healthy render, unchanged, plus exactly one RedisCache. Difference:" >&2
  printf '%s\n' "$failed_sync_diff" >&2
  exit 1
fi

echo "==> Checking the bad-commit overlay: the healthy app with only the Service targetPort moved to 8080"
if ! grep -q "namespace: apptique-broken-states$" "$VAR_DIR/rendered-bad-commit.yaml" \
  || grep -q "namespace: apptique-broken-states-" "$VAR_DIR/rendered-bad-commit.yaml"; then
  echo "Expected the bad-commit overlay to render into the healthy app's namespace, apptique-broken-states" >&2
  exit 1
fi
bad_commit_container_port="$(awk '/^kind: Deployment$/{f=1} f && /containerPort:/{print $NF; exit}' "$VAR_DIR/rendered-bad-commit.yaml")"
bad_commit_target_port="$(awk '/^kind: Service$/{f=1} f && /targetPort:/{print $NF; exit}' "$VAR_DIR/rendered-bad-commit.yaml")"
bad_commit_probe_ports="$(awk '/^kind: Deployment$/{f=1} /^---$/{f=0} f && /Probe:$/{p=1} f && p && /port:/{print $NF; p=0}' "$VAR_DIR/rendered-bad-commit.yaml" | sort -u | tr '\n' ' ')"
if [[ "$bad_commit_container_port" != "80" ]]; then
  echo "Expected the bad-commit overlay's container to stay on port 80 (nginx listens there), got $bad_commit_container_port" >&2
  exit 1
fi
if [[ "$bad_commit_probe_ports" != "80 " ]]; then
  echo "Expected the bad-commit overlay's readiness and liveness probes to stay on port 80, got: $bad_commit_probe_ports" >&2
  echo "Moving the probes off the port nginx listens on makes the pods fail readiness," >&2
  echo "which turns this green-sync scenario into a failed rollout." >&2
  exit 1
fi
if [[ "$bad_commit_target_port" != "8080" ]]; then
  echo "Expected the bad-commit overlay's Service targetPort to be 8080, got $bad_commit_target_port" >&2
  exit 1
fi
changed_lines="$(diff "$VAR_DIR/rendered-healthy.yaml" "$VAR_DIR/rendered-bad-commit.yaml" | grep -c '^[<>]' || true)"
if [[ "$changed_lines" -ne 2 ]]; then
  echo "Expected the bad-commit render to differ from the healthy render in exactly one line, found:" >&2
  diff "$VAR_DIR/rendered-healthy.yaml" "$VAR_DIR/rendered-bad-commit.yaml" >&2 || true
  exit 1
fi

echo "==> Checking every script in this example is syntactically valid, including scenario break.sh files"
while IFS= read -r -d '' script; do
  echo "    bash -n ${script#"$SCRIPT_DIR/"}"
  bash -n "$script"
done < <(find "$SCRIPT_DIR" -name '*.sh' -print0)

echo "==> Checking setup.sh never mentions uploading the failed-sync or bad-commit overlays"
if grep -qE "cub variant upload.*(failed-sync|bad-commit)" "$SCRIPT_DIR/setup.sh"; then
  echo "setup.sh must never upload the failed-sync or bad-commit overlays" >&2
  exit 1
fi

echo "==> Checking setup.sh never uploads two Components into the same Space"
# A ConfigHub Space belongs to one Component: uploading a second Component
# into the same Space re-links it and overwrites its labels. Guard against
# that regression by requiring the Argo and Flux control uploads to name
# two different --space variables.
# shellcheck disable=SC2016  # the pattern is a literal, not an expansion
if ! grep -q -- '--space "\$ARGO_CONTROL_SPACE"' "$SCRIPT_DIR/setup.sh"; then
  echo "setup.sh must upload the argo-control Component into \$ARGO_CONTROL_SPACE" >&2
  exit 1
fi
# shellcheck disable=SC2016  # the pattern is a literal, not an expansion
if ! grep -q -- '--space "\$FLUX_CONTROL_SPACE"' "$SCRIPT_DIR/setup.sh"; then
  echo "setup.sh must upload the flux-control Component into \$FLUX_CONTROL_SPACE" >&2
  exit 1
fi
# The distinct-Spaces check above already confirms the two control Space
# defaults do not collide with each other or with the workload Space.

echo "All gitops-expert-broken-states checks passed."
