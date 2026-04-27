#!/usr/bin/env bash
set -euo pipefail

# seed-initiatives.sh — Create five compliance initiatives over the materialized
# enterprise-rag-blueprint chain. Each initiative is a View (in the recipe space)
# with a Filter that selects relevant units by their ExampleChain label plus a
# component/layer/GPUUser predicate.
#
# Prerequisites:
#   - ./setup.sh has been run (the chain is materialized)
#   - cub auth get-token works
#
# Initiatives are created without vet-kyverno triggers in this example. The
# initiatives-demo (../../../initiatives-demo/) shows how to attach triggers when
# a vet-kyverno-capable worker is connected.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state
begin_log_capture seed-initiatives

INITIATIVE_SPACE="$(recipe_space)"
echo "==> Initiatives will be created in: ${INITIATIVE_SPACE}"

CONFIGHUB_URL="${CONFIGHUB_URL:-$(cub context get --jq '.coordinate.serverURL' 2>/dev/null || true)}"
if [[ -z "${CONFIGHUB_URL}" ]]; then
  echo "ERROR: Could not determine ConfigHub server URL." >&2
  exit 1
fi
API_TOKEN="$(cub auth get-token)"

api() {
  local method="$1" path="$2"
  shift 2
  local content_type="application/json"
  if [[ "$method" == "PATCH" ]]; then
    content_type="application/merge-patch+json"
  fi
  curl -sf -X "$method" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: ${content_type}" \
    "$@" \
    "${CONFIGHUB_URL}/api${path}"
}

SPACE_ID="$(get_space_field "${INITIATIVE_SPACE}" SpaceID)"
if [[ -z "${SPACE_ID}" || "${SPACE_ID}" == "null" ]]; then
  echo "ERROR: Could not resolve SpaceID for ${INITIATIVE_SPACE}." >&2
  exit 1
fi

days_from_now() {
  local n="$1"
  if date --version &>/dev/null 2>&1; then
    date -d "+${n} days" +%Y-%m-%d
  else
    if (( n < 0 )); then date -v"${n}"d +%Y-%m-%d
    else date -v+"${n}"d +%Y-%m-%d
    fi
  fi
}

iso_now() {
  if date --version &>/dev/null 2>&1; then
    date -u +%Y-%m-%dT%H:%M:%SZ
  else
    date -u +%Y-%m-%dT%H:%M:%SZ
  fi
}

create_initiative() {
  local slug="$1"
  local name="$2"
  local description="$3"
  local priority="$4"
  local status="$5"
  local where_clause="$6"
  local check_summary="$7"
  local deadline_days="${8:-21}"

  local filter_id
  filter_id="$(api POST "/space/${SPACE_ID}/filter?allow_exists=true" \
    -d "$(jq -n \
      --arg from "Unit" \
      --arg slug "${slug}" \
      --arg name "${name}" \
      --arg where "${where_clause}" \
      '{From: $from, Slug: $slug, DisplayName: $name, Where: $where}'
    )" | jq -r '.FilterID // empty')"

  if [[ -z "${filter_id}" ]]; then
    echo "  - ${name}: failed to create filter"
    return 1
  fi

  local view_id
  view_id="$(api POST "/space/${SPACE_ID}/view?allow_exists=true" \
    -d "$(jq -n \
      --arg filterId "${filter_id}" \
      --arg slug "${slug}" \
      --arg name "${name}" \
      --arg priority "${priority}" \
      --arg status "${status}" \
      --arg description "${description}" \
      --arg deadline "$(days_from_now "${deadline_days}")" \
      --arg checkSummary "${check_summary}" \
      --arg createdAt "$(iso_now)" \
      '{
        FilterID: $filterId,
        Slug: $slug,
        DisplayName: $name,
        Labels: {
          initiative: "true",
          "initiative-priority": $priority,
          "initiative-status": $status
        },
        Annotations: {
          "initiative-description": $description,
          "initiative-deadline": $deadline,
          "initiative-completed-at": "",
          "initiative-trigger-id": "",
          "initiative-check-summary": $checkSummary
        }
      }'
    )" | jq -r '.ViewID // empty')"

  if [[ -z "${view_id}" ]]; then
    echo "  - ${name}: failed to create view"
    return 1
  fi
  echo "  + ${name}  (priority=${priority}, status=${status})"
}

prefix="$(state_prefix)"
echo "==> Seeding 5 initiatives for ExampleChain=${prefix}"
echo

create_initiative \
  "pin-model-versions" \
  "Pin Model Versions" \
  "Every model-bearing component must use a pinned image tag (no :latest). Required for reproducibility, rollback, and supply-chain attestation." \
  "HIGH" "in_progress" \
  "Labels.ExampleChain = '${prefix}' AND Labels.GPUUser = 'true'" \
  '{"passing":0,"failing":0,"total":0,"checkedAt":"pending"}' \
  14

create_initiative \
  "embed-index-dim-match" \
  "Embedding and Index Dimensions Match" \
  "The EMBED_DIM env on nim-embedding must equal EMBED_DIM on vector-db. A mismatch silently breaks retrieval. Caught at the profile layer; must remain consistent through deployment." \
  "HIGH" "in_progress" \
  "Labels.ExampleChain = '${prefix}' AND Labels.Layer = 'profile'" \
  '{"passing":0,"failing":0,"total":0,"checkedAt":"pending"}' \
  7

create_initiative \
  "gpu-resource-limits" \
  "GPU Resource Limits" \
  "Every GPU-using pod (nim-llm, nim-embedding) must declare an nvidia.com/gpu resource request and an explicit GPU memory limit." \
  "HIGH" "in_progress" \
  "Labels.ExampleChain = '${prefix}' AND Labels.GPUUser = 'true' AND Labels.Layer = 'deployment'" \
  '{"passing":0,"failing":0,"total":0,"checkedAt":"pending"}' \
  21

create_initiative \
  "guardrail-policy-required" \
  "Guardrail Policy Required" \
  "Every rag-server deployment must have GUARDRAIL_POLICY set to a non-off value. Required to prevent prompt injection and PII leakage in production." \
  "MEDIUM" "in_progress" \
  "Labels.ExampleChain = '${prefix}' AND Labels.Component = 'rag-server' AND Labels.Layer = 'deployment'" \
  '{"passing":0,"failing":0,"total":0,"checkedAt":"pending"}' \
  30

create_initiative \
  "resource-limits-enforcement" \
  "Resource Limits Enforcement" \
  "Every deployment-layer pod declares CPU and memory requests and limits. Prevents resource starvation across tenants." \
  "MEDIUM" "draft" \
  "Labels.ExampleChain = '${prefix}' AND Labels.Layer = 'deployment'" \
  '{"passing":0,"failing":0,"total":0,"checkedAt":"pending"}' \
  21

echo
echo "Done. Five initiatives created in ${INITIATIVE_SPACE}."
echo
echo "View Explorer URLs (open one to see the units that match each initiative):"
for slug in pin-model-versions embed-index-dim-match gpu-resource-limits guardrail-policy-required resource-limits-enforcement; do
  vid="$(cub view get --space "${INITIATIVE_SPACE}" -o json "${slug}" 2>/dev/null | jq -r '.View.ViewID')"
  if [[ -n "${vid}" && "${vid}" != "null" ]]; then
    echo "  ${slug}:"
    echo "    ${CONFIGHUB_URL}/x/view-explorer?view=${vid}"
  fi
done
echo
echo "To attach vet-kyverno triggers (live policy enforcement), connect a"
echo "vet-kyverno-capable bridge worker and adapt initiatives-demo/setup.sh."
