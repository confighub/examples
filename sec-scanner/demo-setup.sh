#!/usr/bin/env bash
# demo-setup.sh — Seed the sec-scanner demo fleet in ConfigHub, scan its images
# against a real CVE database, and let the guardrails gate what's vulnerable.
#
# Layout (mirrors the rbac-manager example):
#
#   sec-demo-policy    Triggers (the image guardrail pack) + Filters. No Units.
#   sec-demo-base      Workload Units (Deployments) on current, pinned images.
#   sec-demo-dev       "Cluster" Space (env=dev)     — clones + planted violations
#   sec-demo-staging   "Cluster" Space (env=staging) — clones
#   sec-demo-prod      "Cluster" Space (env=prod)    — clones, approval required
#
# Cluster Spaces select the guardrail Triggers via a Filter in the policy Space
# (TriggerFilterID pattern), so policy is defined once and enforced everywhere.
#
# Three guardrails:
#   valid-schemas     vet-schemas   — Kubernetes schema validation
#   no-latest-tag     vet-celexpr   — block :latest / untagged images (static)
#   no-critical-cves  vet-celexpr   — block images the scanner flagged CRITICAL
#   require-approval   vet-approvedby 1 (prod only)
#
# The no-critical-cves gate is data-driven: the custom scanner (scanner/) digs
# into each Unit's image, matches packages against the cvedb (a SQLite file of
# unified GitHub-Advisory / CVE-List / OSV data), and writes the result back
# onto the Unit as an annotation. The Trigger then gates whatever it marked
# CRITICAL. Config (the image ref + the scan verdict) and policy live as data.
#
# These are "paper clusters": ConfigHub Spaces only, no Targets/Workers — nothing
# deploys to a live cluster. The scanner does pull the real images locally to
# scan them; that is the only thing that touches the outside world.
#
# Prerequisites:
#   - cub CLI installed + authenticated (cub auth login)
#   - go (to build secscan — the scanner and the CVE importer)
#   - network access: secscan pulls image layers straight from their registries
#     (no Docker daemon) and reads the cvedb via a pure-Go SQLite driver (no
#     Python, no sqlite3 binary)
#
# Usage:
#   ./demo-setup.sh                  # create everything (idempotent; safe to re-run)
#   ./demo-setup.sh --explain        # print the plan, mutate nothing
#   ./demo-setup.sh --explain-json   # print the plan as JSON, mutate nothing
#   ./demo-setup.sh --no-scan        # seed ConfigHub only; skip cvedb + scan
#
# Environment variables:
#   PREFIX               Space slug prefix (default: sec-demo)
#   CUB                  Path to cub binary (default: cub on PATH)
#   SEC_SCANNER_DB       cvedb SQLite path (default: cvedb/cve.db)
#   SEC_SCANNER_OFFLINE  =1 to load curated fixtures instead of OSV downloads

set -euo pipefail

PREFIX="${PREFIX:-sec-demo}"
EXAMPLE_NAME="sec-scanner"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS="${SCRIPT_DIR}/manifests"

cub="${CUB:-cub}"
export SEC_SCANNER_DB="${SEC_SCANNER_DB:-${SCRIPT_DIR}/cvedb/cve.db}"

POLICY_SPACE="${PREFIX}-policy"
BASE_SPACE="${PREFIX}-base"
CLUSTER_SPACES=("${PREFIX}-dev" "${PREFIX}-staging" "${PREFIX}-prod")
DEV_SPACE="${PREFIX}-dev"
WORKLOADS=(frontend api cache)
VIOLATIONS=(legacy-frontend legacy-api unpinned-web)
# OSV ecosystem exports covering the demo images (their old Alpine releases).
OSV_ECOSYSTEMS=(Alpine:v3.9 Alpine:v3.10 Alpine:v3.12)

# Guardrail CEL expressions. Validated offline against the manifests with:
#   cub function local <manifest> vet-celexpr '<expr>' --toolchain Kubernetes/YAML
NO_LATEST="r.kind != 'Deployment' || r.spec.template.spec.containers.all(c, c.image.contains(':') && !c.image.endsWith(':latest'))"
NO_CRITICAL="r.kind != 'Deployment' || !has(r.metadata.annotations) || !('sec-scanner.confighub.com/max-severity' in r.metadata.annotations) || r.metadata.annotations['sec-scanner.confighub.com/max-severity'] != 'CRITICAL'"

# ── Explain modes (no mutation) ───────────────────────────────────────────────

explain() {
  cat <<EOF
sec-scanner setup plan
======================

Model: container image security managed as data. Image refs live in ConfigHub
Units; a custom scanner digs into each image, matches packages against a unified
CVE database, and writes findings back as data; guardrails gate the vulnerable.

    ${POLICY_SPACE}             ${BASE_SPACE}
    (guardrail Triggers          (workload Units on current images:
     + Filters, no Units)         frontend, api, cache)
            |                           |  clone (upstream/downstream)
            | TriggerFilterID           v
            +----------->  ${PREFIX}-dev     (env=dev;     + planted violations)
            +----------->  ${PREFIX}-staging (env=staging)
            +----------->  ${PREFIX}-prod    (env=prod;    + approval required)

       cvedb (SQLite)  ◀── import GitHub Advisory DB / CVE List V5 / OSV.dev
            ▲                (unified to one schema)
            │ match
       scanner ── pull image layers (crane) ─▶ parse apk/dpkg ─▶ findings ─▶ write back to Units

Will create (idempotently):
  - 5 Spaces: ${POLICY_SPACE}, ${BASE_SPACE}, ${CLUSTER_SPACES[*]}
  - 4 Triggers in ${POLICY_SPACE} (Pack=sec-guardrails):
      valid-schemas       vet-schemas
      no-latest-tag       vet-celexpr (block :latest / untagged images)
      no-critical-cves    vet-celexpr (block images scanned CRITICAL)
      require-approval    vet-approvedby 1 (prod only)
  - 2 Trigger Filters: sec-guardrails (Scope=all), sec-guardrails-prod (incl. approval)
  - 3 workload Units in ${BASE_SPACE}, cloned into each of the 3 cluster Spaces (9 clones)
  - 3 planted violations in ${PREFIX}-dev:
      legacy-frontend  nginx:1.16-alpine     → CRITICAL CVEs → gated (no-critical-cves)
      legacy-api       python:3.7-alpine3.10 → CRITICAL CVEs → gated (no-critical-cves)
      unpinned-web     nginx:latest          → gated (no-latest-tag, static)

Then: create the cvedb SQLite file, import CVE data, build + run the scanner over
the fleet with --write-back so the no-critical-cves gate fires on the vulnerable
Units.

Mutates: ConfigHub (Spaces/Units) + a local SQLite file (cvedb/cve.db). Pulls
the demo images locally to scan them. No Kubernetes Targets, Workers, or live deploys.
EOF
}

explain_json() {
  local spaces_json units_json eco_json
  spaces_json=$(printf '"%s",' "$POLICY_SPACE" "$BASE_SPACE" "${CLUSTER_SPACES[@]}")
  units_json=$(printf '"%s",' "${WORKLOADS[@]}" "${VIOLATIONS[@]}")
  eco_json=$(printf '"%s",' "${OSV_ECOSYSTEMS[@]}")
  cat <<EOF
{
  "example_name": "${EXAMPLE_NAME}",
  "mutates": true,
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "mutates_local_cvedb": true,
  "pulls_images_locally": true,
  "spaces": [${spaces_json%,}],
  "units": [${units_json%,}],
  "cve_sources": ["github-advisory-database", "cve-list-v5", "osv.dev"],
  "osv_ecosystems_imported": [${eco_json%,}],
  "notes": {
    "workloads_cloned_into": ["${PREFIX}-dev", "${PREFIX}-staging", "${PREFIX}-prod"],
    "violations_space": "${PREFIX}-dev",
    "expected_apply_gates": {
      "legacy-frontend": "no-critical-cves",
      "legacy-api": "no-critical-cves",
      "unpinned-web": "no-latest-tag"
    }
  },
  "evaluation_modes": {
    "fast_preview": {
      "mutates": false,
      "commands": ["./demo-setup.sh --explain", "./demo-setup.sh --explain-json | jq"]
    },
    "fast_operational_evaluation": {
      "mutates_confighub": true,
      "mutates_live_infra": false,
      "commands": ["./demo-setup.sh", "./demo-verify.sh"],
      "stop_before_cleanup": true
    }
  }
}
EOF
}

DO_SCAN=1
case "${1:-}" in
  --explain) explain; exit 0 ;;
  --explain-json) explain_json; exit 0 ;;
  --no-scan) DO_SCAN=0 ;;
  "") ;;
  *) echo "Unknown argument: $1 (supported: --explain, --explain-json, --no-scan)" >&2; exit 2 ;;
esac

# ── Preflight ─────────────────────────────────────────────────────────────────

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found. Install it from https://docs.confighub.com/get-started/setup/#install-the-cli" >&2
  exit 1
fi
if ! $cub space list --quiet &>/dev/null; then
  echo "ERROR: Cannot reach ConfigHub (not authenticated?). Run: $cub auth login" >&2
  exit 1
fi
for f in "${WORKLOADS[@]/#/${MANIFESTS}/workloads/}" "${VIOLATIONS[@]/#/${MANIFESTS}/violations/}"; do
  [[ -f "${f}.yaml" ]] || { echo "ERROR: manifest not found: ${f}.yaml" >&2; exit 1; }
done

created=0
skipped=0
note() { printf '%s\n' "$*"; }

space_exists()   { $cub space get "$1" --quiet &>/dev/null; }
unit_exists()    { $cub unit get "$2" --space "$1" --quiet &>/dev/null; }
trigger_exists() { $cub trigger get "$2" --space "$1" --quiet &>/dev/null; }
filter_exists()  { $cub filter get "$2" --space "$1" --quiet &>/dev/null; }

ensure_space() { # slug, extra flags...
  local slug="$1"; shift
  if space_exists "$slug"; then
    note "  space ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub space create "$slug" "$@" >/dev/null
    note "  created space ${slug}"; ((created+=1))
  fi
}

# ── 1. Policy Space: guardrail Triggers + Filters ─────────────────────────────

note "Policy Space: ${POLICY_SPACE}"
ensure_space "$POLICY_SPACE" --label app=sec-scanner --label role=policy

create_trigger() { # slug scope description function [args...]
  local slug="$1" scope="$2" desc="$3"; shift 3
  if trigger_exists "$POLICY_SPACE" "$slug"; then
    note "  trigger ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub trigger create --space "$POLICY_SPACE" \
      --label Pack=sec-guardrails --label "Scope=${scope}" \
      --description "$desc" \
      "$slug" Mutation Kubernetes/YAML "$@" >/dev/null
    note "  created trigger ${slug}"; ((created+=1))
  fi
}

create_trigger valid-schemas all \
  "Validates Kubernetes resource schemas with kubeconform. Fix: correct the field names/types reported." \
  vet-schemas

create_trigger no-latest-tag all \
  "Blocks Deployments using a :latest or untagged image. Fix: pin the image to an immutable tag or digest." \
  vet-celexpr "$NO_LATEST"

create_trigger no-critical-cves all \
  "Blocks images the scanner flagged with a CRITICAL CVE (sec-scanner.confighub.com/max-severity annotation). Fix: upgrade to a patched image, then re-scan." \
  vet-celexpr "$NO_CRITICAL"

create_trigger require-approval prod \
  "Requires one approval before prod image changes can be applied. Fix: have a reviewer approve the Unit." \
  vet-approvedby 1

ensure_filter() { # slug where
  local slug="$1" where="$2"
  if filter_exists "$POLICY_SPACE" "$slug"; then
    note "  filter ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub filter create --space "$POLICY_SPACE" "$slug" Trigger --where-field "$where" >/dev/null
    note "  created filter ${slug}"; ((created+=1))
  fi
}

ensure_filter sec-guardrails      "Labels.Pack = 'sec-guardrails' AND Labels.Scope = 'all'"
ensure_filter sec-guardrails-prod "Labels.Pack = 'sec-guardrails'"

# ── 2. Base Space: workload Units on current images ───────────────────────────

note "Base Space: ${BASE_SPACE}"
ensure_space "$BASE_SPACE" --label app=sec-scanner --label role=base \
  --trigger-filter "${POLICY_SPACE}/sec-guardrails"

for w in "${WORKLOADS[@]}"; do
  if unit_exists "$BASE_SPACE" "$w"; then
    note "  unit ${w} exists, skipping"; ((skipped+=1))
  else
    # Feed config via stdin ("-") so no local source path is recorded.
    $cub unit create --space "$BASE_SPACE" "$w" - \
      --label app=sec-scanner --label "workload=${w}" \
      --change-desc "Seed ${w} workload on a current image" \
      < "${MANIFESTS}/workloads/${w}.yaml" >/dev/null
    note "  created unit ${w}"; ((created+=1))
  fi
done

# ── 3. Cluster Spaces: clones of the base workloads ───────────────────────────

cluster_env()    { case "$1" in *-dev) echo dev ;; *-staging) echo staging ;; *-prod) echo prod ;; esac; }
cluster_filter() { case "$1" in *-prod) echo sec-guardrails-prod ;; *) echo sec-guardrails ;; esac; }

for space in "${CLUSTER_SPACES[@]}"; do
  env="$(cluster_env "$space")"
  note "Cluster Space: ${space} (env=${env})"
  ensure_space "$space" \
    --label app=sec-scanner --label "env=${env}" \
    --trigger-filter "${POLICY_SPACE}/$(cluster_filter "$space")"

  for w in "${WORKLOADS[@]}"; do
    if unit_exists "$space" "$w"; then
      note "  unit ${w} exists, skipping"; ((skipped+=1))
    else
      $cub unit create --space "$space" "$w" \
        --upstream-unit "$w" --upstream-space "$BASE_SPACE" \
        --label app=sec-scanner --label "workload=${w}" --label "env=${env}" \
        --change-desc "Clone ${w} workload from ${BASE_SPACE}" >/dev/null
      note "  cloned unit ${w}"; ((created+=1))
    fi
  done
done

# ── 4. Planted violations in dev ──────────────────────────────────────────────

note "Planted violations in ${DEV_SPACE}"
for v in "${VIOLATIONS[@]}"; do
  if unit_exists "$DEV_SPACE" "$v"; then
    note "  unit ${v} exists, skipping"; ((skipped+=1))
  else
    # Feed config via stdin ("-") so no local source path is recorded.
    $cub unit create --space "$DEV_SPACE" "$v" - \
      --label app=sec-scanner --label imported=true \
      --change-desc "Planted demo violation: ${v}" \
      < "${MANIFESTS}/violations/${v}.yaml" >/dev/null
    note "  created unit ${v}"; ((created+=1))
  fi
done

# ── 5. cvedb + scan the fleet ─────────────────────────────────────────────────

if (( DO_SCAN )); then
  note ""
  note "Scanning the fleet (cvedb + custom scanner)"
  "${SCRIPT_DIR}/cvedb/build.sh"        # create cvedb SQLite file + import CVE data (idempotent)
  scanner_bin="${SCRIPT_DIR}/scanner/secscan"
  if [[ ! -x "$scanner_bin" ]]; then
    note "  building scanner..."
    ( cd "${SCRIPT_DIR}/scanner" && go build -o secscan . )
  fi
  # The scanner talks to the ConfigHub REST API directly (like the web app does),
  # so bridge this cub session's server URL + token to it via the environment.
  export CONFIGHUB_URL="${CONFIGHUB_URL:-$($cub context get 2>/dev/null | awk '/Server URL/{print $NF}')}"
  export CONFIGHUB_TOKEN="${CONFIGHUB_TOKEN:-$($cub auth get-token 2>/dev/null)}"
  note "  scanning images referenced across ${PREFIX}-* and writing findings back..."
  "$scanner_bin" scan-fleet --space "${PREFIX}-*" --write-back --status-space "$POLICY_SPACE"
else
  note ""
  note "Skipped scan (--no-scan). The no-critical-cves gate will not fire until you run:"
  note "  ./cvedb/build.sh && (cd scanner && go build -o secscan .)"
  note "  ./scanner/secscan scan-fleet --space '${PREFIX}-*' --write-back"
fi

# ── Summary ───────────────────────────────────────────────────────────────────

cat <<EOF

Done. Created ${created} entities, skipped ${skipped} existing.

Inspect the result:
  $cub unit list --space "${PREFIX}-*" --where "Labels.app = 'sec-scanner'"
  $cub unit get legacy-frontend --space ${DEV_SPACE} -o jq=".Unit.ApplyGates"
  ./scanner/secscan inventory --space "${PREFIX}-*"

Next: ./demo-verify.sh confirms the layout, the gate matrix, and the cvedb.
EOF
