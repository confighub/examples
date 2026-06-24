#!/usr/bin/env bash
# setup.sh — Seed the ConfigHub initiatives demo (component-per-Space model)
#
# Layout (the "component model"):
#   - One Space PER application component: aichat, portal, eshop, docs, website.
#     Each carries the well-known component labels on the SPACE (not the units):
#       Component, Variant=base, Owner, Team, Layer=App, Purpose=initiatives-demo
#     and references the shared compliance Triggers via --trigger-filter.
#   - One "platform" Space — initiatives-demo — that holds NO application units,
#     only the cross-cutting entities: the kyverno/k8s Workers, the initiative
#     Views + their Filters, and the vet-kyverno Triggers.
#
# Each initiative is a View with Labels (initiative=true, initiative-priority,
# initiative-status) and Annotations (initiative-description, initiative-deadline,
# initiative-check-summary, etc.). The View's underlying Filter selects units
# ACROSS the component Spaces by their Space labels (Space.Labels.Component /
# Space.Labels.Owner, scoped by Space.Labels.Purpose). A Trigger runs
# vet-kyverno against the matched units.
#
# Prerequisites:
#   - cub CLI installed: https://docs.confighub.com/get-started/setup/#install-the-cli
#   - Authenticated: cub auth login
#   - kind, kubectl, docker, jq on PATH (used to stand up the local kyverno worker)
#
# Usage:
#   ./setup.sh
#   PLATFORM_SPACE=my-platform ./setup.sh
#
# Environment variables:
#   PLATFORM_SPACE         Platform space slug   (default: initiatives-demo)
#   PURPOSE                Purpose label value tying the component spaces together
#                          (default: initiatives-demo)
#   CLUSTER_NAME           kind cluster name     (default: $PLATFORM_SPACE)
#   TRIGGER_ENABLE_DELAY   Seconds between enabling each trigger (default: 15)
#   CUB                    Path to cub binary    (default: cub on PATH)

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

# The platform space holds the demo's shared entities (workers, filters, views,
# triggers) — but no application units. SPACE is accepted as a legacy alias.
PLATFORM_SPACE="${PLATFORM_SPACE:-${SPACE:-initiatives-demo}}"
# Purpose label stamped on every component space so the initiative filters can
# scope to "the spaces in this demo" regardless of how they are named.
PURPOSE="${PURPOSE:-initiatives-demo}"

# A WhereTrigger predicate that matches no Trigger. An *empty* WhereTrigger is
# not "no triggers" — the server defaults it to `SpaceID = <this space>`, which
# would make the platform space validate its own units (the worker unit) with
# the demo's triggers. Setting an impossible predicate (no Trigger lives in the
# nil-UUID space) opts the platform space out without a TriggerFilterID.
NO_TRIGGERS_WHERE="SpaceID = '00000000-0000-0000-0000-000000000000'"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMO_DIR="${SCRIPT_DIR}/../promotion-demo-data/config-data"

cub="${CUB:-cub}"

# Verify cub is available
if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found. Install it from https://docs.confighub.com/get-started/setup/#install-the-cli" >&2
  exit 1
fi

# Verify authentication
if ! $cub auth status &>/dev/null; then
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

# ── Component model ───────────────────────────────────────────────────────────
#
# One Space per component. The Owner/Team values become Space labels and the
# initiative filters select across spaces by them.
#
#   Component  Owner      Team
#   aichat     Support    AI
#   portal     Support    Portal
#   eshop      Product    Commerce
#   docs       Product    DevEx
#   website    Marketing  Web
#
# Slug == component (Variant=base). Edit COMPONENTS / component_meta to taste.
# (Plain arrays + a case lookup keep this compatible with the bash 3.2 that
# ships on macOS — no associative arrays.)
COMPONENTS=(aichat portal eshop docs website)

# Echo "<Owner> <Team>" for a component.
component_meta() {
  case "$1" in
    aichat)  echo "Support AI" ;;
    portal)  echo "Support Portal" ;;
    eshop)   echo "Product Commerce" ;;
    docs)    echo "Product DevEx" ;;
    website) echo "Marketing Web" ;;
    *)       echo "" ;;
  esac
}

# ── Helpers ───────────────────────────────────────────────────────────────────

created_spaces=0
created_units=0
# Seconds to wait between refreshing each component space in Step 8. Each
# refresh fans every initiative check out across that space's units, so a small
# delay keeps the work paced.
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

# Create the platform space. It holds the workers, filters, views, and triggers
# — but no application units. We point its WhereTrigger at an impossible
# predicate so the demo's own vet-kyverno triggers (which live here) do NOT fire
# against the platform space's worker unit; the triggers only reach the
# component spaces, which opt in via --trigger-filter.
create_platform_space() {
  if $cub space get "$PLATFORM_SPACE" --quiet 2>/dev/null; then
    echo "Platform space '$PLATFORM_SPACE' already exists, reusing."
  else
    $cub space create "$PLATFORM_SPACE" \
      --label "Purpose=$PURPOSE" \
      --label "Layer=Platform" \
      --quiet
    echo "Created platform space '$PLATFORM_SPACE'."
  fi
  # Opt the platform space out of trigger validation (see NO_TRIGGERS_WHERE).
  $cub space update "$PLATFORM_SPACE" --where-trigger "$NO_TRIGGERS_WHERE" --quiet >/dev/null
}

# Create (or update) a component space with the well-known component labels and
# wire it to the shared compliance triggers via --trigger-filter. With a
# TriggerFilterID set, clearing --where-trigger leaves the filter as the sole
# predicate so only the platform's vet-kyverno triggers apply here. Trigger
# wiring is always done via `cub space update`.
create_component_space() {
  local comp="$1" owner="$2" team="$3" trigger_filter_ref="$4"
  local -a labels=(
    --label "Component=$comp"
    --label "Variant=base"
    --label "Owner=$owner"
    --label "Team=$team"
    --label "Layer=App"
    --label "Purpose=$PURPOSE"
  )
  if $cub space get "$comp" --quiet 2>/dev/null; then
    echo "  ↳ space '$comp' already exists, updating labels + triggers."
    $cub space update "$comp" "${labels[@]}" --quiet >/dev/null
  else
    $cub space create "$comp" "${labels[@]}" --quiet
    echo "  ↳ created space '$comp' (Owner=$owner, Team=$team)"
    created_spaces=$((created_spaces + 1))
  fi
  $cub space update "$comp" \
    --trigger-filter "$trigger_filter_ref" --where-trigger "-" --quiet >/dev/null
}

# Create a unit in a component space. Labels live on the Space, not the unit, so
# units stay clean — the initiative filters select them via Space.Labels.
create_unit() {
  local space="$1" slug="$2" yaml_file="$3"
  if $cub unit get --space "$space" "$slug" --quiet 2>/dev/null; then
    echo "  ↳ $space/$slug already exists, skipping."
    return
  fi
  $cub unit create --space "$space" --quiet "$slug" "$yaml_file"
  echo "  ↳ created $space/$slug"
  created_units=$((created_units + 1))
}

resolve_space_id() {
  $cub space get "$1" -o jq=".Space.SpaceID" 2>/dev/null
}

# Find a Ready bridge worker that supports vet-kyverno *within the platform
# space*. We deliberately do not search globally — sharing a worker across
# demos couples their lifecycles, so each demo brings its own. `--select "*"`
# is required: ProvidedInfo.FunctionWorkerInfo.SupportedFunctions is not among
# the default columns `cub worker list` returns. Some workers expose
# SupportedFunctions as null, so we normalise to {} before iterating.
find_kyverno_worker() {
  $cub worker list --space "$PLATFORM_SPACE" --select "*" -o json 2>/dev/null | jq -r --arg sid "$PLATFORM_SPACE_ID" '
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
# Each initiative lives entirely in the platform space:
#   Filter (selects units cross-space via Space.Labels) + View (metadata) +
#   Trigger (vet-kyverno, labelled initiative-check=true so the component
#   spaces' --trigger-filter picks it up).

create_initiative() {
  local name="$1"
  local description="$2"
  local priority="$3"    # HIGH, MEDIUM, LOW
  local status="$4"      # draft, in_progress, completed
  local deadline="$5"    # YYYY-MM-DD
  local created_at="$6"  # ISO 8601 (kept for call-site readability)
  local completed_at="$7"
  local where_clause="$8"
  local policy_func="$9"
  local check_summary="${10}"
  local worker_id="${11:-}"
  local enforce="${12:-false}"  # true → gate (ApplyGates); false → warning (ApplyWarnings)

  local slug
  slug=$(make_slug "$name")

  # 1. Filter — selects units cross-space via the Space labels in $where_clause.
  #    No --from-space, so FromSpaceID stays empty and the filter resolves across
  #    spaces (matching units by their Space's labels).
  $cub filter create --space "$PLATFORM_SPACE" --allow-exists \
    "$slug" Unit --where-field "$where_clause" --quiet >/dev/null

  # 2. View — initiative metadata in Labels + Annotations, referencing the
  #    filter (same slug). `cub view create` has no --display-name flag, and the
  #    annotation map carries JSON / possibly-empty values, so the body goes in
  #    via --from-stdin, merged with the positional <slug> <filter>.
  jq -n \
    --arg name "$name" --arg priority "$priority" --arg status "$status" \
    --arg description "$description" --arg deadline "$deadline" \
    --arg completedAt "$completed_at" --arg checkSummary "$check_summary" \
    '{
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
    }' \
    | $cub view create --space "$PLATFORM_SPACE" --allow-exists \
        "$slug" "$slug" --from-stdin --quiet >/dev/null

  # 3. Trigger — vet-kyverno with the inline policy as its positional argument.
  #    --label initiative-check=true lets every component space pick it up via
  #    --trigger-filter. --disable creates it paused (Step 8 enables). --warn
  #    makes failures advisory (ApplyWarnings); omitting it makes the trigger an
  #    enforced gate (ApplyGates).
  if [[ -n "$worker_id" ]]; then
    local -a trig_flags=(--worker "$worker_id" --label "initiative-check=true" --disable)
    [[ "$enforce" != "true" ]] && trig_flags+=(--warn)

    # 4. Create the trigger and capture its id straight from the create response
    #    (-o jq), then record it on the view.
    local trigger_id
    trigger_id=$($cub trigger create --space "$PLATFORM_SPACE" --allow-exists "${trig_flags[@]}" \
      "${slug}-check" Mutation Kubernetes/YAML vet-kyverno "$($policy_func)" -o jq='.TriggerID')
    [[ -n "$trigger_id" ]] && $cub view update --space "$PLATFORM_SPACE" "$slug" \
      --annotation "initiative-trigger-id=$trigger_id" --quiet >/dev/null
  fi

  echo "  ✓ $name (priority=$priority, status=$status)"
}

# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════

echo "=== ConfigHub Initiatives Demo ==="
echo "Platform:  $PLATFORM_SPACE"
echo "Purpose:   $PURPOSE"
echo "Components: ${COMPONENTS[*]}"
echo ""

# ── Step 1: Platform space ────────────────────────────────────────────────────

echo "--- Setting up platform space ---"
create_platform_space
PLATFORM_SPACE_ID=$(resolve_space_id "$PLATFORM_SPACE")
if [[ -z "$PLATFORM_SPACE_ID" ]]; then
  echo "ERROR: Could not resolve SpaceID for '$PLATFORM_SPACE'." >&2
  exit 1
fi
echo "SpaceID: $PLATFORM_SPACE_ID"
echo ""

# ── Step 2: Local kind cluster + kyverno worker ──────────────────────────────
#
# The demo brings its own kind cluster, k8s-worker, and vet-kyverno worker so
# initiative triggers have somewhere to run. Workers live in the platform
# space. Everything is namespaced under CLUSTER_NAME (defaults to the platform
# space slug) so cleanup.sh can find it.

CLUSTER_NAME="${CLUSTER_NAME:-$PLATFORM_SPACE}"
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
if $cub worker get --space "$PLATFORM_SPACE" "$K8S_WORKER" --quiet 2>/dev/null; then
  echo "Worker '$K8S_WORKER' already exists, reusing."
else
  $cub worker install --space "$PLATFORM_SPACE" \
    --export --include-secret \
    -t Kubernetes \
    "$K8S_WORKER" 2>/dev/null | kubectl --context "$KCTX" apply -f -
fi
kubectl --context "$KCTX" -n confighub rollout status deployment/"$K8S_WORKER" --timeout=120s
$cub target get --space "$PLATFORM_SPACE" --wait --timeout 120s "$K8S_TARGET" >/dev/null
echo "Target $K8S_TARGET is ready."
echo ""

echo "--- Installing kyverno CLI worker ---"
if $cub worker get --space "$PLATFORM_SPACE" "$KYVERNO_WORKER" --quiet 2>/dev/null; then
  echo "Worker '$KYVERNO_WORKER' already exists, reusing."
else
  $cub worker install --space "$PLATFORM_SPACE" \
    --unit kyverno-cli-worker-unit \
    --target "$K8S_TARGET" \
    -n "$KYVERNO_WORKER_NAMESPACE" \
    --image "$KYVERNO_IMAGE" \
    --image-pull-policy Never \
    "$KYVERNO_WORKER"
  # Don't wait — the deployment won't be Ready until the secret below is applied.
  $cub unit apply --space "$PLATFORM_SPACE" kyverno-cli-worker-unit
  kubectl --context "$KCTX" -n "$KYVERNO_WORKER_NAMESPACE" \
    wait --for=create deployment/"$KYVERNO_WORKER" --timeout=120s
  $cub worker install --space "$PLATFORM_SPACE" \
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

# ── Step 4: Trigger-selecting filter ─────────────────────────────────────────
#
# A From=Trigger filter that selects every initiative check (Labels.initiative-
# check=true). The component spaces reference it via --trigger-filter so all of
# the platform's vet-kyverno triggers fire on their units.

echo "--- Creating shared trigger filter ---"
CHECKS_FILTER_SLUG="initiative-checks"
CHECKS_FILTER_ID=$($cub filter create --space "$PLATFORM_SPACE" --allow-exists \
  "$CHECKS_FILTER_SLUG" Trigger --where-field "Labels.initiative-check = 'true'" -o jq='.FilterID')
if [[ -z "$CHECKS_FILTER_ID" ]]; then
  echo "ERROR: Could not resolve trigger filter '$CHECKS_FILTER_SLUG'." >&2
  exit 1
fi
echo "Trigger filter: $CHECKS_FILTER_SLUG ($CHECKS_FILTER_ID)"
# Reference the trigger filter cross-space as <platform-space>/<slug>. A bare
# UUID makes `cub space create` try to resolve the filter inside the target
# space (and panic on an empty ID); the space-qualified slug resolves it in the
# platform space instead.
CHECKS_FILTER_REF="$PLATFORM_SPACE/$CHECKS_FILTER_SLUG"
echo ""

# ── Step 5: Create initiatives (filters + views + triggers, paused) ──────────
#
# Created before the component spaces so the triggers exist when each space
# resolves its --trigger-filter.

echo "--- Creating initiatives ---"
echo ""

# 1. Liveness and Readiness Probes — Owner = Support (aichat + portal)
#    Pass: aichat-api, aichat-frontend, portal-api, portal-frontend
#    Fail: aichat-postgres, aichat-redis, aichat-worker, portal-postgres
create_initiative \
  "Liveness and Readiness Probes" \
  "All Deployments and StatefulSets must define livenessProbe and readinessProbe on every container. Required for graceful rollouts and service mesh integration." \
  "HIGH" "in_progress" \
  "$(days_from_now 7)" "$(days_ago_iso 6)" "" \
  "Space.Labels.Owner = 'Support' AND Space.Labels.Purpose = '$PURPOSE'" \
  policy_require_probes \
  "{\"passing\":4,\"failing\":4,\"total\":8,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 2. Image Registry Restriction — Owner = Product (eshop + docs)
#    Pass: eshop-api, eshop-frontend, eshop-worker, docs-server
#    Fail: eshop-postgres (postgres:16-alpine), eshop-redis (redis:7-alpine), docs-search (meilisearch)
create_initiative \
  "Image Registry Restriction" \
  "All container images must be pulled from the approved registry (ghcr.io/acme). Public registries like Docker Hub are not permitted in production." \
  "HIGH" "in_progress" \
  "$(days_from_now 10)" "$(days_ago_iso 14)" "" \
  "Space.Labels.Owner = 'Product' AND Space.Labels.Purpose = '$PURPOSE'" \
  policy_restrict_image_registries \
  "{\"passing\":4,\"failing\":3,\"total\":7,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 3. Run-As-NonRoot Enforcement — Component = aichat (2 passing / 3 failing)
#    Pass: aichat-api, aichat-frontend (have securityContext.runAsNonRoot)
#    Fail: aichat-postgres, aichat-redis, aichat-worker
create_initiative \
  "Run-As-NonRoot Enforcement" \
  "All pods must set runAsNonRoot at the pod or container level to ensure no container runs as UID 0." \
  "MEDIUM" "in_progress" \
  "$(days_from_now 21)" "$(days_ago_iso 3)" "" \
  "Space.Labels.Component = 'aichat' AND Space.Labels.Purpose = '$PURPOSE'" \
  policy_require_run_as_nonroot \
  "{\"passing\":2,\"failing\":3,\"total\":5,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 4. Resource Limits Enforcement — Component = website (2 passing / 1 failing)
#    Pass: website-web, website-postgres
#    Fail: website-cms (runtime variant with limits stripped)
create_initiative \
  "Resource Limits Enforcement" \
  "All workloads must define CPU and memory limits to prevent resource starvation." \
  "MEDIUM" "draft" \
  "$(days_from_now 14)" "$(days_ago_iso 2)" "" \
  "Space.Labels.Component = 'website' AND Space.Labels.Purpose = '$PURPOSE'" \
  policy_require_resource_limits \
  "{\"passing\":2,\"failing\":1,\"total\":3,\"checkedAt\":\"$(days_ago_iso 1)\"}" \
  "$WORKER_ID"

# 5. Disallow Host Ports — Component = docs, completed + enforced (2 passing / 0 failing)
#    Pass: docs-server, docs-search
#    Enforce: true → trigger runs as a hard gate (Warn: false / ApplyGates)
create_initiative \
  "Disallow Host Ports" \
  "Containers must not bind to host ports. Completed — all workloads now route through ClusterIP Services and Ingress." \
  "LOW" "completed" \
  "$(days_from_now -5)" "$(days_ago_iso 30)" "$(days_ago_iso 3)" \
  "Space.Labels.Component = 'docs' AND Space.Labels.Purpose = '$PURPOSE'" \
  policy_disallow_host_ports \
  "{\"passing\":2,\"failing\":0,\"total\":2,\"checkedAt\":\"$(days_ago_iso 3)\"}" \
  "$WORKER_ID" \
  "true"
echo ""

# ── Step 6: Create component spaces ──────────────────────────────────────────
#
# Each space carries the component labels and references the shared trigger
# filter. The triggers are still paused here; Step 8 enables them and refreshes
# each space so they resolve and fire against its units.

echo "--- Creating component spaces ---"
for comp in "${COMPONENTS[@]}"; do
  read -r comp_owner comp_team <<< "$(component_meta "$comp")"
  create_component_space "$comp" "$comp_owner" "$comp_team" "$CHECKS_FILTER_REF"
done
echo ""

# ── Step 7: Create units in their component spaces ───────────────────────────

echo "--- Creating units ---"
echo ""
echo "Component: aichat"
create_unit aichat "aichat-api"      "$PROMO_DIR/aichat/api.yaml"
create_unit aichat "aichat-frontend" "$PROMO_DIR/aichat/frontend.yaml"
create_unit aichat "aichat-postgres" "$PROMO_DIR/aichat/postgres.yaml"
create_unit aichat "aichat-redis"    "$PROMO_DIR/aichat/redis.yaml"
create_unit aichat "aichat-worker"   "$PROMO_DIR/aichat/worker.yaml"

echo ""
echo "Component: portal"
create_unit portal "portal-api"      "$PROMO_DIR/portal/api.yaml"
create_unit portal "portal-frontend" "$PROMO_DIR/portal/frontend.yaml"
create_unit portal "portal-postgres" "$PROMO_DIR/portal/postgres.yaml"

echo ""
echo "Component: eshop"
create_unit eshop "eshop-api"      "$PROMO_DIR/eshop/api.yaml"
create_unit eshop "eshop-frontend" "$PROMO_DIR/eshop/frontend.yaml"
create_unit eshop "eshop-postgres" "$PROMO_DIR/eshop/postgres.yaml"
create_unit eshop "eshop-redis"    "$PROMO_DIR/eshop/redis.yaml"
create_unit eshop "eshop-worker"   "$PROMO_DIR/eshop/worker.yaml"

echo ""
echo "Component: docs"
create_unit docs "docs-server" "$PROMO_DIR/docs/server.yaml"
create_unit docs "docs-search" "$PROMO_DIR/docs/search.yaml"

echo ""
echo "Component: website"
create_unit website "website-web"      "$PROMO_DIR/website/web.yaml"
create_unit website "website-cms"      "$WORKDIR/cms-no-limits.yaml"
create_unit website "website-postgres" "$PROMO_DIR/website/postgres.yaml"
echo ""

# ── Step 8: Enable triggers, then refresh the component spaces ───────────────
#
# Two-part dance:
#   a. Enable the (paused) triggers. They live in the platform space; enabling
#      alone does NOT fire them against units in the component spaces.
#   b. Each component space resolved its trigger list at create time, when the
#      triggers were still paused — so it captured zero. Re-resolve with
#      `cub space update --refresh-triggers`, which both picks up the now-enabled
#      triggers AND fires them against the space's existing units.
#
# We pace the work by refreshing one component space at a time
# (TRIGGER_ENABLE_DELAY seconds apart); each refresh fans every initiative check
# out across that space's units.

if [[ -n "$WORKER_ID" ]]; then
  echo "--- Enabling triggers ---"
  $cub trigger update --space "$PLATFORM_SPACE" --patch \
    --where "Labels.initiative-check = 'true'" --enable --quiet >/dev/null \
    && echo "  ✓ enabled initiative triggers" \
    || echo "  ✗ failed to enable triggers"

  echo "--- Refreshing component-space triggers (one every ${TRIGGER_ENABLE_DELAY}s) ---"
  first=1
  for comp in "${COMPONENTS[@]}"; do
    if (( first )); then
      first=0
    else
      sleep "$TRIGGER_ENABLE_DELAY"
    fi
    $cub space update "$comp" \
      --trigger-filter "$CHECKS_FILTER_REF" --where-trigger "-" --refresh-triggers --quiet >/dev/null \
      && echo "  ✓ refreshed $comp" \
      || echo "  ✗ failed to refresh $comp"
  done
fi

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "=== Done ==="
echo "  Platform space:  $PLATFORM_SPACE"
echo "  Component spaces: ${COMPONENTS[*]}"
echo "  Cluster:         $CLUSTER_NAME"
echo "  Units:           $created_units"
echo "  Spaces created:  $created_spaces"
echo ""
echo "Open the ConfigHub UI and navigate to Initiatives to see them."
if [[ -n "$WORKER_ID" ]]; then
  echo "Triggers were enabled and each component space refreshed one at a time"
  echo "(${TRIGGER_ENABLE_DELAY}s apart) so the per-space fan-outs land in sequence."
  echo "Override the pacing with TRIGGER_ENABLE_DELAY=N (seconds)."
else
  echo "Worker did not register — re-run after fixing to attach policy check triggers."
fi
echo ""
echo "To clean up (spaces + kind cluster), run: ./cleanup.sh"
