#!/usr/bin/env bash
# setup.sh — Seed demo initiatives in ConfigHub
#
# Creates 5 compliance initiatives backed by Kyverno CEL policies, using the
# same app units as the promotion demo (aichat, website, docs, eshop, portal).
#
# Each initiative is a View with Labels (initiative=true, initiative-priority,
# initiative-status) and Annotations (initiative-description, initiative-deadline,
# initiative-check-summary, etc.).  The View's underlying Filter selects units
# by App or AppOwner label.  An optional Trigger runs vet-kyverno against
# matched units.
#
# Prerequisites:
#   - cub CLI installed: https://docs.confighub.com/get-started/setup/#install-the-cli
#   - Authenticated: cub auth login
#   - kind, kubectl, docker, jq on PATH (used to stand up the local kyverno worker)
#
# Usage:
#   ./setup.sh
#   SPACE=my-space ./setup.sh
#
# Environment variables:
#   CONFIGHUB_URL          ConfigHub server URL  (default: derived from current cub context)
#   SPACE                  Target space slug     (default: initiatives-demo)
#   CLUSTER_NAME           kind cluster name     (default: $SPACE)
#   TRIGGER_ENABLE_DELAY   Seconds between enabling each trigger (default: 15)
#   CUB                    Path to cub binary    (default: cub on PATH)

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

SPACE="${SPACE:-initiatives-demo}"
EXAMPLE_NAME="initiatives-demo"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMO_DIR="${SCRIPT_DIR}/../promotion-demo-data/config-data"

cub="${CUB:-cub}"

# Verify cub is available
if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found. Install it from https://docs.confighub.com/get-started/setup/#install-the-cli" >&2
  exit 1
fi

# Verify authentication
if ! $cub auth get-token &>/dev/null; then
  echo "ERROR: Not authenticated. Run: $cub auth login" >&2
  exit 1
fi

# Verify the tools we need to stand up the local kind cluster + kyverno worker.
for tool in kind kubectl docker jq; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH" >&2
    exit 1
  fi
done

# Verify promotion data exists
if [[ ! -d "$PROMO_DIR" ]]; then
  echo "ERROR: Promotion demo data not found at $PROMO_DIR" >&2
  echo "This script uses YAML files from the promotion-demo-data example." >&2
  exit 1
fi

# Verify the kyverno worker source ships alongside this demo.
KYVERNO_DIR="${SCRIPT_DIR}/../custom-workers/kyverno"
if [[ ! -d "$KYVERNO_DIR" ]]; then
  echo "ERROR: Kyverno worker source not found at $KYVERNO_DIR" >&2
  exit 1
fi

# Derive the API endpoint from the current cub context so raw curl calls hit
# the same server cub itself is talking to. Honour an explicit CONFIGHUB_URL
# override if the caller set one.
if [[ -z "${CONFIGHUB_URL:-}" ]]; then
  CONFIGHUB_URL=$($cub context get --jq '.coordinate.serverURL' 2>/dev/null || true)
fi
if [[ -z "${CONFIGHUB_URL:-}" ]]; then
  echo "ERROR: Could not determine ConfigHub server URL from cub context." >&2
  echo "       Set CONFIGHUB_URL explicitly or run 'cub context use <name>'." >&2
  exit 1
fi
export CONFIGHUB_URL

API_TOKEN=$($cub auth get-token)

# ── Helpers ───────────────────────────────────────────────────────────────────

created_initiatives=0
created_units=0
TRIGGER_IDS=()
# Seconds to wait between enabling each trigger. Enabling a trigger fires it
# against every unit matched by its filter, so back-to-back enables compound
# the per-trigger fan-outs. A small delay keeps the work paced.
TRIGGER_ENABLE_DELAY="${TRIGGER_ENABLE_DELAY:-15}"

# Cross-platform date arithmetic (macOS + Linux)
days_from_now() {
  local n="$1"
  if date --version &>/dev/null 2>&1; then
    date -d "+${n} days" +%Y-%m-%d
  else
    if (( n < 0 )); then
      date -v"${n}"d +%Y-%m-%d
    else
      date -v+"${n}"d +%Y-%m-%d
    fi
  fi
}

days_ago_iso() {
  local n="$1"
  if date --version &>/dev/null 2>&1; then
    date -d "-${n} days" -u +%Y-%m-%dT%H:%M:%SZ
  else
    date -v-"${n}"d -u +%Y-%m-%dT%H:%M:%SZ
  fi
}

make_slug() {
  echo "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/^-//;s/-$//' | cut -c1-50
}

# API helper — wraps curl with auth and JSON content type.
api() {
  local method="$1" path="$2"
  shift 2
  local content_type="application/json"
  if [[ "$method" == "PATCH" ]]; then
    content_type="application/merge-patch+json"
  fi
  curl -sf \
    -X "$method" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: ${content_type}" \
    "$@" \
    "${CONFIGHUB_URL}/api${path}"
}

create_space_if_needed() {
  if $cub space get "$SPACE" --quiet 2>/dev/null; then
    echo "Space '$SPACE' already exists, reusing."
  else
    printf '{"Labels":{"ExampleName":"%s"},"Annotations":{"Description":"Demo space for compliance initiatives"}}' "$EXAMPLE_NAME" \
      | $cub space create --json --from-stdin "$SPACE"
    echo "Created space '$SPACE'."
  fi
}

create_unit() {
  local slug="$1" app="$2" owner="$3" team="$4" yaml_file="$5"
  if $cub unit get --space "$SPACE" "$slug" --quiet 2>/dev/null; then
    echo "  ↳ $slug already exists, skipping."
    return
  fi
  $cub unit create --space "$SPACE" \
    --label "ExampleName=$EXAMPLE_NAME" \
    --label "App=$app" \
    --label "AppOwner=$owner" \
    --label "Team=$team" \
    --quiet \
    "$slug" "$yaml_file"
  echo "  ↳ created $slug"
  created_units=$((created_units + 1))
}

resolve_space_id() {
  $cub space get "$SPACE" --jq ".Space.SpaceID" --quiet 2>/dev/null
}

# Find a Ready bridge worker that supports vet-kyverno *within this demo's
# own space*. We deliberately do not search globally — sharing a worker
# across demos couples their lifecycles, so each demo brings its own.
# Some workers expose FunctionWorkerInfo.SupportedFunctions as null, so we
# normalise to {} before iterating to avoid jq errors on those entries.
find_kyverno_worker() {
  local workers
  workers=$(api GET "/space/${SPACE_ID}/bridge_worker" 2>/dev/null || echo "[]")
  # Defensive: if the per-space endpoint ever surfaces visible-but-not-resident
  # workers, the SpaceID check still keeps us pinned to our own space.
  echo "$workers" | jq -r --arg sid "$SPACE_ID" '
    [ .[]
      | select(.BridgeWorker.SpaceID == $sid)
      | select(.BridgeWorker.Condition == "Ready")
      | select(
          ((.BridgeWorker.ProvidedInfo.FunctionWorkerInfo.SupportedFunctions // {})
             | to_entries
             | map(.value | has("vet-kyverno"))
             | any)
        )
      | .BridgeWorker.BridgeWorkerID
    ][0] // empty'
}

# ── Non-compliant variant ────────────────────────────────────────────────────
# Create a copy of website/cms.yaml with resource limits stripped, so the
# "Resource Limits Enforcement" initiative has a failing unit.

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

awk '
  /^        resources:$/ { skip=1; next }
  skip && /^          / { next }
  skip { skip=0 }
  { print }
' "$PROMO_DIR/website/cms.yaml" > "$WORKDIR/cms-no-limits.yaml"

# ── Kyverno policy definitions ───────────────────────────────────────────────

policy_require_probes() {
cat << 'POLICY'
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-probes
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets]
  validations:
    - expression: >
        object.spec.template.spec.containers.all(c, has(c.livenessProbe))
      message: "All containers must define a livenessProbe."
    - expression: >
        object.spec.template.spec.containers.all(c, has(c.readinessProbe))
      message: "All containers must define a readinessProbe."
POLICY
}

policy_restrict_image_registries() {
cat << 'POLICY'
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: restrict-image-registries
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets, daemonsets]
  validations:
    - expression: >
        object.spec.template.spec.containers.all(c,
          c.image.startsWith('ghcr.io/acme/')
        )
      message: "Images must come from ghcr.io/acme/."
POLICY
}

policy_require_run_as_nonroot() {
cat << 'POLICY'
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-run-as-nonroot
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets, daemonsets]
  validations:
    - expression: >
        (
          has(object.spec.template.spec.securityContext) &&
          has(object.spec.template.spec.securityContext.runAsNonRoot) &&
          object.spec.template.spec.securityContext.runAsNonRoot == true
        ) ||
        object.spec.template.spec.containers.all(c,
          has(c.securityContext) &&
          has(c.securityContext.runAsNonRoot) &&
          c.securityContext.runAsNonRoot == true
        )
      message: "Pods or all containers must set runAsNonRoot to true."
POLICY
}

policy_require_resource_limits() {
cat << 'POLICY'
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-resource-limits
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets, daemonsets]
  validations:
    - expression: >
        object.spec.template.spec.containers.all(c,
          has(c.resources) &&
          has(c.resources.limits) &&
          has(c.resources.limits.cpu) &&
          has(c.resources.limits.memory)
        )
      message: "All containers must specify CPU and memory limits."
POLICY
}

policy_disallow_host_ports() {
cat << 'POLICY'
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-host-ports
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets, daemonsets]
  validations:
    - expression: >
        object.spec.template.spec.containers.all(c,
          !has(c.ports) ||
          c.ports.all(p, !has(p.hostPort) || p.hostPort == 0)
        )
      message: "Containers must not bind to host ports."
POLICY
}

# ── Initiative creation ─────────────────────────────────────────────────────────
#
# Each initiative: Filter (selects units) + View (stores metadata) + Trigger (vet-kyverno).

create_initiative() {
  local name="$1"
  local description="$2"
  local priority="$3"    # HIGH, MEDIUM, LOW
  local status="$4"      # draft, in_progress, completed
  local deadline="$5"    # YYYY-MM-DD
  local created_at="$6"  # ISO 8601
  local completed_at="$7"
  local where_clause="$8"
  local policy_func="$9"
  local check_summary="${10}"
  local worker_id="${11:-}"
  local enforce="${12:-false}"  # true → Warn: false (hard gate); false → Warn: true (advisory)

  local slug
  slug=$(make_slug "$name")

  # 1. Create Filter
  local filter_id
  filter_id=$(api POST "/space/${SPACE_ID}/filter?allow_exists=true" \
    -d "$(jq -n \
      --arg from "Unit" \
      --arg slug "$slug" \
      --arg name "$name" \
      --arg where "$where_clause" \
      '{From: $from, Slug: $slug, DisplayName: $name, Where: $where}'
    )" | jq -r '.FilterID // empty')

  if [[ -z "$filter_id" ]]; then
    echo "  ✗ $name — failed to create filter"
    return 1
  fi

  # 2. Create View with initiative labels and annotations
  local view_id
  view_id=$(api POST "/space/${SPACE_ID}/view?allow_exists=true" \
    -d "$(jq -n \
      --arg filterId "$filter_id" \
      --arg slug "$slug" \
      --arg name "$name" \
      --arg priority "$priority" \
      --arg status "$status" \
      --arg description "$description" \
      --arg deadline "$deadline" \
      --arg completedAt "$completed_at" \
      --arg checkSummary "$check_summary" \
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
          "initiative-completed-at": $completedAt,
          "initiative-trigger-id": "",
          "initiative-check-summary": $checkSummary
        }
      }'
    )" | jq -r '.ViewID // empty')

  if [[ -z "$view_id" ]]; then
    echo "  ✗ $name — failed to create view"
    return 1
  fi

  # 3. Create Trigger with inline Kyverno policy (if worker available).
  #    Disabled: true   → trigger is created in a paused state. Step 6 below
  #                       enables them one at a time so the per-trigger
  #                       fan-outs don't all land at once. Enable manually
  #                       in the UI later if you want to fire on demand.
  #    Warn: true       → failures populate ApplyWarnings (non-blocking/advisory).
  #    Warn: false      → failures populate ApplyGates (enforced, blocks Applies).
  local warn_value="true"
  [[ "$enforce" == "true" ]] && warn_value="false"
  local trigger_id=""
  if [[ -n "$worker_id" ]]; then
    local policy_yaml
    policy_yaml=$($policy_func)
    trigger_id=$(api POST "/space/${SPACE_ID}/trigger?allow_exists=true" \
      -d "$(jq -n \
        --arg slug "${slug}-check" \
        --arg workerId "$worker_id" \
        --arg policy "$policy_yaml" \
        --argjson warn "$warn_value" \
        '{
          Slug: $slug,
          Event: "Mutation",
          ToolchainType: "Kubernetes/YAML",
          FunctionName: "vet-kyverno",
          BridgeWorkerID: $workerId,
          Arguments: [{ParameterName: "policy", Value: $policy}],
          Disabled: true,
          Warn: $warn
        }'
      )" 2>/dev/null | jq -r '.TriggerID // empty' 2>/dev/null) || true
  fi

  # 4. Patch View with trigger ID
  if [[ -n "$trigger_id" ]]; then
    api PATCH "/space/${SPACE_ID}/view/${view_id}" \
      -d "$(jq -n --arg tid "$trigger_id" '{Annotations: {"initiative-trigger-id": $tid}}')" \
      >/dev/null 2>&1 || true
    TRIGGER_IDS+=("$trigger_id")
  fi

  echo "  ✓ $name (priority=$priority, status=$status)"
  created_initiatives=$((created_initiatives + 1))
}

# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════

echo "=== ConfigHub Initiatives Demo ==="
echo "Server:  $CONFIGHUB_URL"
echo "Space:   $SPACE"
echo ""

# ── Step 1: Space ─────────────────────────────────────────────────────────────

echo "--- Setting up space ---"
create_space_if_needed
SPACE_ID=$(resolve_space_id)
if [[ -z "$SPACE_ID" ]]; then
  echo "ERROR: Could not resolve SpaceID for '$SPACE'." >&2
  exit 1
fi
echo "SpaceID: $SPACE_ID"
echo ""

# ── Step 2: Local kind cluster + kyverno worker ──────────────────────────────
#
# The demo brings its own kind cluster, k8s-worker, and vet-kyverno worker so
# initiative triggers have somewhere to run. Everything is namespaced under
# CLUSTER_NAME (defaults to the space slug) so cleanup.sh can find it.

CLUSTER_NAME="${CLUSTER_NAME:-$SPACE}"
KCTX="kind-$CLUSTER_NAME"
K8S_WORKER="k8s-worker"
K8S_TARGET="k8s-worker-kubernetes-yaml-cluster"
KYVERNO_WORKER="kyverno-cli-worker"
KYVERNO_WORKER_NAMESPACE="kyverno-cli-worker"
KYVERNO_IMAGE="kyverno-cli-worker:initiatives-demo"

echo "--- Setting up kind cluster '$CLUSTER_NAME' ---"
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "Cluster '$CLUSTER_NAME' already exists, reusing."
else
  kind create cluster --name "$CLUSTER_NAME"
fi
kubectl cluster-info --context "$KCTX" >/dev/null
echo ""

echo "--- Building and loading kyverno worker image ---"
docker build -t "$KYVERNO_IMAGE" "$KYVERNO_DIR"
kind load docker-image "$KYVERNO_IMAGE" --name "$CLUSTER_NAME"
echo ""

echo "--- Installing standard Kubernetes worker ---"
if $cub worker get --space "$SPACE" "$K8S_WORKER" --quiet 2>/dev/null; then
  echo "Worker '$K8S_WORKER' already exists, reusing."
else
  $cub worker install --space "$SPACE" \
    --export --include-secret \
    -t Kubernetes \
    "$K8S_WORKER" 2>/dev/null | kubectl --context "$KCTX" apply -f -
fi
kubectl --context "$KCTX" -n confighub rollout status deployment/"$K8S_WORKER" --timeout=120s
$cub target get --space "$SPACE" --wait --timeout 120s "$K8S_TARGET" >/dev/null
echo "Target $K8S_TARGET is ready."
echo ""

echo "--- Installing kyverno CLI worker ---"
if $cub worker get --space "$SPACE" "$KYVERNO_WORKER" --quiet 2>/dev/null; then
  echo "Worker '$KYVERNO_WORKER' already exists, reusing."
else
  $cub worker install --space "$SPACE" \
    --unit kyverno-cli-worker-unit \
    --target "$K8S_TARGET" \
    -n "$KYVERNO_WORKER_NAMESPACE" \
    --image "$KYVERNO_IMAGE" \
    --image-pull-policy Never \
    "$KYVERNO_WORKER"
  # Don't wait — the deployment won't be Ready until the secret below is applied.
  $cub unit apply --space "$SPACE" kyverno-cli-worker-unit
  kubectl --context "$KCTX" -n "$KYVERNO_WORKER_NAMESPACE" \
    wait --for=create deployment/"$KYVERNO_WORKER" --timeout=120s
  $cub worker install --space "$SPACE" \
    --export-secret-only \
    -n "$KYVERNO_WORKER_NAMESPACE" \
    "$KYVERNO_WORKER" 2>/dev/null | kubectl --context "$KCTX" apply -f -
fi
kubectl --context "$KCTX" -n "$KYVERNO_WORKER_NAMESPACE" \
  rollout status deployment/"$KYVERNO_WORKER" --timeout=120s
echo "Kyverno CLI worker is ready."
echo ""

# ── Step 3: Detect kyverno worker ────────────────────────────────────────────

WORKER_ID=$(find_kyverno_worker) || true
if [[ -n "$WORKER_ID" ]]; then
  echo "Kyverno worker: $WORKER_ID"
else
  echo "WARNING: vet-kyverno worker did not register with ConfigHub — initiatives will be created without triggers."
fi
echo ""

# ── Step 4: Create units from promotion demo YAML files ─────────────────────
#
# Units are labeled with App and AppOwner to match the promotion demo model.
# Initiative filters use these labels to select units for each initiative.
#
#   Initiative 1 (Probes)            → AppOwner = 'Support'  (aichat + portal)
#   Initiative 2 (Registry)          → AppOwner = 'Product'  (eshop + docs)
#   Initiative 3 (Run-As-NonRoot)    → App = 'aichat'
#   Initiative 4 (Resource Limits)   → App = 'website'
#   Initiative 5 (Host Ports)        → App = 'docs'

echo "--- Creating units from promotion demo data ---"
echo ""

echo "App: aichat (Owner: Support, Team: AI)"
create_unit "aichat-api"      "aichat" "Support" "AI" "$PROMO_DIR/aichat/api.yaml"
create_unit "aichat-frontend" "aichat" "Support" "AI" "$PROMO_DIR/aichat/frontend.yaml"
create_unit "aichat-postgres" "aichat" "Support" "AI" "$PROMO_DIR/aichat/postgres.yaml"
create_unit "aichat-redis"    "aichat" "Support" "AI" "$PROMO_DIR/aichat/redis.yaml"
create_unit "aichat-worker"   "aichat" "Support" "AI" "$PROMO_DIR/aichat/worker.yaml"

echo ""
echo "App: portal (Owner: Support, Team: Portal)"
create_unit "portal-api"      "portal" "Support" "Portal" "$PROMO_DIR/portal/api.yaml"
create_unit "portal-frontend" "portal" "Support" "Portal" "$PROMO_DIR/portal/frontend.yaml"
create_unit "portal-postgres" "portal" "Support" "Portal" "$PROMO_DIR/portal/postgres.yaml"

echo ""
echo "App: eshop (Owner: Product, Team: Commerce)"
create_unit "eshop-api"      "eshop" "Product" "Commerce" "$PROMO_DIR/eshop/api.yaml"
create_unit "eshop-frontend" "eshop" "Product" "Commerce" "$PROMO_DIR/eshop/frontend.yaml"
create_unit "eshop-postgres" "eshop" "Product" "Commerce" "$PROMO_DIR/eshop/postgres.yaml"
create_unit "eshop-redis"    "eshop" "Product" "Commerce" "$PROMO_DIR/eshop/redis.yaml"
create_unit "eshop-worker"   "eshop" "Product" "Commerce" "$PROMO_DIR/eshop/worker.yaml"

echo ""
echo "App: docs (Owner: Product, Team: DevEx)"
create_unit "docs-server" "docs" "Product" "DevEx" "$PROMO_DIR/docs/server.yaml"
create_unit "docs-search" "docs" "Product" "DevEx" "$PROMO_DIR/docs/search.yaml"

echo ""
echo "App: website (Owner: Marketing, Team: Web)"
create_unit "website-web"      "website" "Marketing" "Web" "$PROMO_DIR/website/web.yaml"
create_unit "website-cms"      "website" "Marketing" "Web" "$WORKDIR/cms-no-limits.yaml"
create_unit "website-postgres" "website" "Marketing" "Web" "$PROMO_DIR/website/postgres.yaml"

echo ""

# ── Step 5: Create initiatives ─────────────────────────────────────────────────

echo "--- Creating initiatives ---"
echo ""

# 1. Liveness and Readiness Probes (4 passing / 4 failing)
#    Pass: aichat-api, aichat-frontend, portal-api, portal-frontend
#    Fail: aichat-postgres, aichat-redis, aichat-worker, portal-postgres
create_initiative \
  "Liveness and Readiness Probes" \
  "All Deployments and StatefulSets must define livenessProbe and readinessProbe on every container. Required for graceful rollouts and service mesh integration." \
  "HIGH" "in_progress" \
  "$(days_from_now 7)" "$(days_ago_iso 6)" "" \
  "Labels.AppOwner = 'Support'" \
  policy_require_probes \
  "{\"passing\":4,\"failing\":4,\"total\":8,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 2. Image Registry Restriction (4 passing / 3 failing)
#    Pass: eshop-api, eshop-frontend, eshop-worker, docs-server
#    Fail: eshop-postgres (postgres:16-alpine), eshop-redis (redis:7-alpine), docs-search (meilisearch)
create_initiative \
  "Image Registry Restriction" \
  "All container images must be pulled from the approved registry (ghcr.io/acme). Public registries like Docker Hub are not permitted in production." \
  "HIGH" "in_progress" \
  "$(days_from_now 10)" "$(days_ago_iso 14)" "" \
  "Labels.AppOwner = 'Product'" \
  policy_restrict_image_registries \
  "{\"passing\":4,\"failing\":3,\"total\":7,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 3. Run-As-NonRoot Enforcement (2 passing / 3 failing)
#    Pass: aichat-api, aichat-frontend (have securityContext.runAsNonRoot)
#    Fail: aichat-postgres, aichat-redis, aichat-worker
create_initiative \
  "Run-As-NonRoot Enforcement" \
  "All pods must set runAsNonRoot at the pod or container level to ensure no container runs as UID 0." \
  "MEDIUM" "in_progress" \
  "$(days_from_now 21)" "$(days_ago_iso 3)" "" \
  "Labels.App = 'aichat'" \
  policy_require_run_as_nonroot \
  "{\"passing\":2,\"failing\":3,\"total\":5,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 4. Resource Limits Enforcement (2 passing / 1 failing)
#    Pass: website-web, website-postgres
#    Fail: website-cms (runtime variant with limits stripped)
create_initiative \
  "Resource Limits Enforcement" \
  "All workloads must define CPU and memory limits to prevent resource starvation." \
  "MEDIUM" "draft" \
  "$(days_from_now 14)" "$(days_ago_iso 2)" "" \
  "Labels.App = 'website'" \
  policy_require_resource_limits \
  "{\"passing\":2,\"failing\":1,\"total\":3,\"checkedAt\":\"$(days_ago_iso 1)\"}" \
  "$WORKER_ID"

# 5. Disallow Host Ports — completed + enforced (2 passing / 0 failing)
#    Pass: docs-server, docs-search
#    Enforce: true → trigger runs as a hard gate (Warn: false / ApplyGates)
create_initiative \
  "Disallow Host Ports" \
  "Containers must not bind to host ports. Completed — all workloads now route through ClusterIP Services and Ingress." \
  "LOW" "completed" \
  "$(days_from_now -5)" "$(days_ago_iso 30)" "$(days_ago_iso 3)" \
  "Labels.App = 'docs'" \
  policy_disallow_host_ports \
  "{\"passing\":2,\"failing\":0,\"total\":2,\"checkedAt\":\"$(days_ago_iso 3)\"}" \
  "$WORKER_ID" \
  "true"

# ── Step 6: Enable triggers (staggered) ──────────────────────────────────────
#
# Enabling a trigger fires it against every unit currently matched by its
# filter. Across all 5 initiatives that's ~25 fan-out events; doing the
# enables back-to-back stacks those bursts on top of each other. Stagger
# by TRIGGER_ENABLE_DELAY seconds (default 15) to keep the work paced.

if (( ${#TRIGGER_IDS[@]} > 0 )); then
  echo ""
  echo "--- Enabling triggers (one every ${TRIGGER_ENABLE_DELAY}s) ---"
  first=1
  for tid in "${TRIGGER_IDS[@]}"; do
    if (( first )); then
      first=0
    else
      sleep "$TRIGGER_ENABLE_DELAY"
    fi
    api PATCH "/space/${SPACE_ID}/trigger/${tid}" \
      -d '{"Disabled": false}' >/dev/null \
      && echo "  ✓ enabled $tid" \
      || echo "  ✗ failed to enable $tid"
  done
fi

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "=== Done ==="
echo "  Space:       $SPACE"
echo "  Cluster:     $CLUSTER_NAME"
echo "  Units:       $created_units"
echo "  Initiatives: $created_initiatives"
echo ""
echo "Open the ConfigHub UI and navigate to Initiatives to see them."
if [[ -n "$WORKER_ID" ]]; then
  echo "Triggers were enabled one at a time (${TRIGGER_ENABLE_DELAY}s apart) so the"
  echo "per-initiative fan-outs land in sequence rather than all at once."
  echo "Override the pacing with TRIGGER_ENABLE_DELAY=N (seconds)."
else
  echo "Worker did not register — re-run after fixing to attach policy check triggers."
fi
echo ""
echo "To clean up (space + kind cluster), run: ./cleanup.sh"
