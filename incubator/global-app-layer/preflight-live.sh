#!/usr/bin/env bash
set -euo pipefail

require_cub() {
  if ! command -v cub >/dev/null 2>&1; then
    echo "Missing required command: cub" >&2
    exit 1
  fi
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "Missing required command: jq" >&2
    exit 1
  fi
}

usage() {
  cat <<'EOF_USAGE'
Check whether a target is actually ready for live delivery.

This is read-only.
It distinguishes:
- target exists in ConfigHub
- worker is attached to that target
- worker is currently Ready for apply

Usage:
  ./preflight-live.sh <space/target>
  ./preflight-live.sh <space/target> --json

Examples:
  ./preflight-live.sh gitops-import-test/worker-kubernetes-yaml-cluster
  ./preflight-live.sh gitops-import-test/worker-argocdrenderer-kubernetes-yaml-cluster --json
EOF_USAGE
}

is_zero_time() {
  local ts="${1:-}"
  [[ -z "${ts}" || "${ts}" == "0001-01-01T00:00:00Z" ]]
}

json_output=0
target_ref=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      json_output=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      if [[ -n "${target_ref}" ]]; then
        echo "Unexpected extra argument: $1" >&2
        usage >&2
        exit 1
      fi
      target_ref="$1"
      shift
      ;;
  esac
done

if [[ -z "${target_ref}" || "${target_ref}" != */* ]]; then
  echo "Expected target ref in the form <space/target>." >&2
  usage >&2
  exit 1
fi

require_cub
require_jq

target_space="${target_ref%%/*}"
target_slug="${target_ref##*/}"

target_doc="$(
  cub target list --space "*" --json \
    | jq --arg space "${target_space}" --arg target "${target_slug}" '
        [ .[] | select(.Space.Slug == $space and .Target.Slug == $target) ][0]
      '
)"

if [[ "${target_doc}" == "null" || -z "${target_doc}" ]]; then
  if [[ "${json_output}" -eq 1 ]]; then
    jq -n \
      --arg targetRef "${target_ref}" \
      '{
        mutates: false,
        targetRef: $targetRef,
        targetExists: false,
        applyReady: false,
        reasons: ["target not found in current ConfigHub context"]
      }'
  else
    cat <<EOF_NOT_FOUND
Live delivery preflight for ${target_ref}

Read-only: yes
Target exists: no
Apply ready: false

Why:
- target not found in current ConfigHub context

What to do next:
1. stay in ConfigHub-only mode
2. run: cub target list --space "*" --json | jq
3. choose a real <space/target> and run this preflight again
EOF_NOT_FOUND
  fi
  exit 1
fi

provider_type="$(printf '%s\n' "${target_doc}" | jq -r '.Target.ProviderType // ""')"
bridge_worker_slug="$(printf '%s\n' "${target_doc}" | jq -r '.BridgeWorker.Slug // ""')"

delivery_mode="unknown"
case "${provider_type}" in
  Kubernetes) delivery_mode="direct" ;;
  ArgoCDRenderer) delivery_mode="gitops" ;;
esac

worker_condition=""
worker_last_seen=""

if [[ -n "${bridge_worker_slug}" ]]; then
  worker_condition="$(
    cub worker get "${bridge_worker_slug}" --space "${target_space}" --json 2>/dev/null \
      | jq -r '.BridgeWorker.Condition // ""' 2>/dev/null || true
  )"
  worker_last_seen="$(
    cub worker get "${bridge_worker_slug}" --space "${target_space}" --json 2>/dev/null \
      | jq -r '.BridgeWorker.LastSeenAt // ""' 2>/dev/null || true
  )"
fi

reasons=()

if [[ -z "${bridge_worker_slug}" ]]; then
  reasons+=("target has no bridge worker attached")
fi

if [[ -n "${bridge_worker_slug}" && -z "${worker_condition}" ]]; then
  reasons+=("bridge worker could not be read from cub worker get")
fi

if [[ -n "${worker_condition}" && "${worker_condition}" != "Ready" ]]; then
  reasons+=("worker condition is ${worker_condition}")
fi

if [[ -n "${bridge_worker_slug}" && -n "${worker_last_seen}" ]] && is_zero_time "${worker_last_seen}"; then
  reasons+=("worker has no meaningful LastSeenAt timestamp")
fi

if [[ "${delivery_mode}" == "unknown" ]]; then
  reasons+=("provider type ${provider_type:-<empty>} is not one of the documented live paths")
fi

apply_ready=true
if [[ "${#reasons[@]}" -gt 0 ]]; then
  apply_ready=false
fi

reasons_json="$(printf '%s\n' "${reasons[@]:-}" | jq -R . | jq -s 'map(select(length > 0))')"

next_steps_json="$(
  jq -n \
    --arg deliveryMode "${delivery_mode}" \
    --arg applyReady "${apply_ready}" '
      if $applyReady == "true" then
        if $deliveryMode == "direct" then
          [
            "run ./set-target.sh <space/target> if needed",
            "run cub unit approve ...",
            "run cub unit apply ..."
          ]
        elif $deliveryMode == "gitops" then
          [
            "use the GitOps/Argo-oriented path for this target",
            "expect delegated delivery rather than direct kubectl-style apply",
            "verify the resulting controller-side objects"
          ]
        else
          [
            "the worker looks ready, but the provider type is not part of the documented example flow"
          ]
        end
      else
        [
          "do not claim the live path is ready",
          "stay in ConfigHub-only mode or choose another target",
          "for a known-good local proof, use incubator/global-app-layer/e2e/02-greenfield.sh after gitops-import/bin/install-worker"
        ]
      end
    '
)"

if [[ "${json_output}" -eq 1 ]]; then
  jq -n \
    --arg targetRef "${target_ref}" \
    --arg targetSpace "${target_space}" \
    --arg targetSlug "${target_slug}" \
    --arg providerType "${provider_type}" \
    --arg deliveryMode "${delivery_mode}" \
    --arg workerSlug "${bridge_worker_slug}" \
    --arg workerCondition "${worker_condition}" \
    --arg workerLastSeen "${worker_last_seen}" \
    --argjson targetExists true \
    --argjson applyReady "${apply_ready}" \
    --argjson reasons "${reasons_json}" \
    --argjson nextSteps "${next_steps_json}" \
    '{
      mutates: false,
      targetRef: $targetRef,
      targetExists: $targetExists,
      space: $targetSpace,
      target: $targetSlug,
      providerType: $providerType,
      deliveryMode: $deliveryMode,
      bridgeWorker: {
        slug: (if $workerSlug == "" then null else $workerSlug end),
        condition: (if $workerCondition == "" then null else $workerCondition end),
        lastSeenAt: (if $workerLastSeen == "" then null else $workerLastSeen end)
      },
      applyReady: $applyReady,
      reasons: $reasons,
      nextSteps: $nextSteps
    }'
else
  echo "Live delivery preflight for ${target_ref}"
  echo ""
  echo "Read-only: yes"
  echo "Target exists: yes"
  echo "Provider: ${provider_type:-<unknown>}"
  echo "Delivery mode: ${delivery_mode}"
  echo "Worker: ${bridge_worker_slug:-<none>}"
  echo "Worker condition: ${worker_condition:-<unknown>}"
  echo "Worker last seen: ${worker_last_seen:-<unknown>}"
  echo "Apply ready: ${apply_ready}"
  echo ""
  if [[ "${apply_ready}" == "true" ]]; then
    echo "What this means:"
    if [[ "${delivery_mode}" == "direct" ]]; then
      echo "- direct apply is expected to work if you approve and apply the units"
    elif [[ "${delivery_mode}" == "gitops" ]]; then
      echo "- delegated/GitOps delivery is expected to work for this target"
    else
      echo "- the worker looks ready, but the provider type is not part of the documented example flow"
    fi
  else
    echo "Why apply would be blocked:"
    printf '%s\n' "${reasons[@]}" | sed 's/^/- /'
  fi
  echo ""
  echo "Next steps:"
  printf '%s\n' "${next_steps_json}" | jq -r '.[]' | nl -w1 -s'. '
fi

if [[ "${apply_ready}" != "true" ]]; then
  exit 1
fi
