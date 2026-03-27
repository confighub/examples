#!/usr/bin/env bash
# setup.sh — Seed demo campaigns in ConfigHub
#
# Creates 10 compliance campaigns backed by Kyverno CEL policies, along with
# ~47 sample Kubernetes units that the campaigns filter and evaluate.
#
# Each campaign is a View with Labels (campaign=true, campaign-priority,
# campaign-status) and Annotations (campaign-description, campaign-deadline,
# campaign-check-summary, etc.).  The View's underlying Filter selects units
# by team label.  An optional Trigger runs vet-kyverno against matched units.
#
# Prerequisites:
#   - cub CLI installed: https://docs.confighub.com/get-started/setup/#install-the-cli
#   - Authenticated: cub auth login
#
# Usage:
#   ./setup.sh                                    # defaults: app.confighub.com, space "campaigns-demo"
#   CONFIGHUB_URL=https://my-server.com ./setup.sh
#   SPACE=my-space ./setup.sh
#
# Environment variables:
#   CONFIGHUB_URL   ConfigHub server URL  (default: https://app.confighub.com)
#   SPACE           Target space slug     (default: campaigns-demo)
#   CUB             Path to cub binary    (default: cub on PATH)

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

export CONFIGHUB_URL="${CONFIGHUB_URL:-https://app.confighub.com}"
SPACE="${SPACE:-campaigns-demo}"
EXAMPLE_NAME="campaigns-demo"

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

API_TOKEN=$($cub auth get-token)

# ── Helpers ───────────────────────────────────────────────────────────────────

created_campaigns=0
created_units=0

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
    printf '{"Labels":{"ExampleName":"%s","Environment":"Demo"},"Annotations":{"Description":"Demo space for compliance campaigns"}}' "$EXAMPLE_NAME" \
      | $cub space create --json --from-stdin "$SPACE"
    echo "Created space '$SPACE'."
  fi
}

create_unit() {
  local slug="$1" campaign="$2" team="$3" yaml_file="$4"
  if $cub unit get --space "$SPACE" "$slug" --quiet 2>/dev/null; then
    echo "  ↳ $slug already exists, skipping."
    return
  fi
  $cub unit create --space "$SPACE" \
    --label "ExampleName=$EXAMPLE_NAME" \
    --label "campaign=$campaign" \
    --label "team=$team" \
    --quiet \
    "$slug" "$yaml_file"
  echo "  ↳ created $slug"
  created_units=$((created_units + 1))
}

# Look up the SpaceID for our space.
resolve_space_id() {
  $cub space get "$SPACE" --jq ".Space.SpaceID" --quiet 2>/dev/null
}

# Find a Ready bridge worker that supports vet-kyverno.
find_kyverno_worker() {
  local workers
  workers=$(api GET "/bridge_worker" 2>/dev/null || echo "[]")
  echo "$workers" | jq -r '
    [.[] | select(
      .BridgeWorker.Condition == "Ready" and
      (.BridgeWorker.ProvidedInfo.FunctionWorkerInfo.SupportedFunctions
       | to_entries[]?.value | has("vet-kyverno"))
    ) | .BridgeWorker.BridgeWorkerID][0] // empty'
}

# ── Temporary YAML templates ─────────────────────────────────────────────────

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

# Fully compliant deployment (probes, limits, nonroot, approved registry)
cat > "$WORKDIR/deployment-good.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "1.0.0"
    app.kubernetes.io/managed-by: confighub
    confighub.com/network-policy: "allow-internal"
spec:
  replicas: 2
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: ghcr.io/confighubai/app:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
        securityContext:
          runAsNonRoot: true
          allowPrivilegeEscalation: false
YAML

# Missing liveness and readiness probes
cat > "$WORKDIR/deployment-no-probes.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "1.0.0"
    app.kubernetes.io/managed-by: confighub
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: ghcr.io/confighubai/app:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        securityContext:
          runAsNonRoot: true
          allowPrivilegeEscalation: false
YAML

# Image from Docker Hub (non-approved registry)
cat > "$WORKDIR/deployment-dockerhub.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "2.1.0"
    app.kubernetes.io/managed-by: confighub
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: docker.io/library/python:3.11-slim
        ports:
        - containerPort: 5000
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "1"
        livenessProbe:
          httpGet:
            path: /health
            port: 5000
          initialDelaySeconds: 15
        readinessProbe:
          httpGet:
            path: /ready
            port: 5000
          initialDelaySeconds: 10
        securityContext:
          runAsNonRoot: true
          allowPrivilegeEscalation: false
YAML

# Missing runAsNonRoot (runs as root by default)
cat > "$WORKDIR/deployment-root.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: ghcr.io/confighubai/app:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
YAML

# Missing resource limits entirely
cat > "$WORKDIR/deployment-no-limits.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "0.5.0"
    app.kubernetes.io/managed-by: confighub
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: ghcr.io/confighubai/app:latest
        ports:
        - containerPort: 8080
YAML

# StatefulSet with large memory footprint
cat > "$WORKDIR/statefulset-highmem.yaml" << 'YAML'
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "1.0.0"
    app.kubernetes.io/managed-by: confighub
spec:
  serviceName: confighubplaceholder
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: db
        image: ghcr.io/confighubai/db:latest
        ports:
        - containerPort: 5432
        resources:
          requests:
            memory: "8Gi"
            cpu: "2"
          limits:
            memory: "16Gi"
            cpu: "4"
        livenessProbe:
          tcpSocket:
            port: 5432
          initialDelaySeconds: 30
        readinessProbe:
          tcpSocket:
            port: 5432
          initialDelaySeconds: 15
        securityContext:
          runAsNonRoot: true
          allowPrivilegeEscalation: false
YAML

# CronJob
cat > "$WORKDIR/cronjob.yaml" << 'YAML'
apiVersion: batch/v1
kind: CronJob
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "1.0.0"
    app.kubernetes.io/managed-by: confighub
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: job
            image: ghcr.io/confighubai/batch:latest
            resources:
              requests:
                memory: "256Mi"
                cpu: "100m"
              limits:
                memory: "512Mi"
                cpu: "500m"
            securityContext:
              runAsNonRoot: true
              allowPrivilegeEscalation: false
          restartPolicy: OnFailure
YAML

# TLS Secret
cat > "$WORKDIR/tls-secret.yaml" << 'YAML'
apiVersion: v1
kind: Secret
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
type: kubernetes.io/tls
data:
  tls.crt: LS0tLS1CRUdJTi...
  tls.key: LS0tLS1CRUdJTi...
YAML

# Node.js 18 (non-compliant for node22 upgrade campaign)
cat > "$WORKDIR/deployment-node18.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
  labels:
    app.kubernetes.io/name: confighubplaceholder
    app.kubernetes.io/version: "1.0.0"
    app.kubernetes.io/managed-by: confighub
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: node:18-alpine
        ports:
        - containerPort: 3000
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 3000
          initialDelaySeconds: 5
        securityContext:
          runAsNonRoot: true
          allowPrivilegeEscalation: false
YAML

# Missing standard Kubernetes labels
cat > "$WORKDIR/deployment-no-labels.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confighubplaceholder
  namespace: confighubplaceholder
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confighubplaceholder
  template:
    metadata:
      labels:
        app: confighubplaceholder
    spec:
      containers:
      - name: app
        image: ghcr.io/confighubai/app:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
        securityContext:
          runAsNonRoot: true
          allowPrivilegeEscalation: false
YAML

# ── Kyverno policy definitions ────────────────────────────────────────────────
# Two formats:
#   ValidatingPolicy (policies.kyverno.io/v1)               — Kyverno CEL
#   ValidatingAdmissionPolicy (admissionregistration.k8s.io) — native K8s CEL

policy_require_labels() {
cat << 'POLICY'
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments]
  validations:
    - expression: >
        has(object.metadata.labels) &&
        'app' in object.metadata.labels &&
        object.metadata.labels['app'] != ''
      message: "The label 'app' is required."
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

policy_require_min_replicas() {
cat << 'POLICY'
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-minimum-replicas
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets]
  validations:
    - expression: "has(object.spec.replicas) && object.spec.replicas >= 2"
      message: "Production workloads must have at least 2 replicas for availability."
POLICY
}

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

policy_disallow_latest_tag() {
cat << 'POLICY'
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-latest-tag
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments]
  validations:
    - expression: >
        object.spec.template.spec.containers.all(c,
          !c.image.endsWith(':latest')
        )
      message: "Using 'latest' tag is not allowed."
POLICY
}

policy_disallow_privileged() {
cat << 'POLICY'
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-privileged
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
          !has(c.securityContext) ||
          !has(c.securityContext.privileged) ||
          c.securityContext.privileged == false
        )
      message: "Privileged containers are not allowed."
    - expression: >
        !has(object.spec.template.spec.initContainers) ||
        object.spec.template.spec.initContainers.all(c,
          !has(c.securityContext) ||
          !has(c.securityContext.privileged) ||
          c.securityContext.privileged == false
        )
      message: "Privileged init containers are not allowed."
POLICY
}

policy_require_readonly_rootfs() {
cat << 'POLICY'
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: require-readonly-rootfs
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [deployments, statefulsets]
  validations:
    - expression: >
        object.spec.template.spec.containers.all(c,
          has(c.securityContext) &&
          has(c.securityContext.readOnlyRootFilesystem) &&
          c.securityContext.readOnlyRootFilesystem == true
        )
      message: "All containers must set readOnlyRootFilesystem to true."
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
          c.image.startsWith('ghcr.io/confighubai/') ||
          c.image.startsWith('gcr.io/confighub-prod/')
        )
      message: "Images must come from ghcr.io/confighubai or gcr.io/confighub-prod."
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

# ── Campaign creation ─────────────────────────────────────────────────────────
#
# Each campaign: Filter (selects units) + View (stores metadata) + Trigger (vet-kyverno).

create_campaign() {
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

  # 2. Create View with campaign labels and annotations
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
          campaign: "true",
          "campaign-priority": $priority,
          "campaign-status": $status
        },
        Annotations: {
          "campaign-description": $description,
          "campaign-deadline": $deadline,
          "campaign-completed-at": $completedAt,
          "campaign-trigger-id": "",
          "campaign-check-summary": $checkSummary
        }
      }'
    )" | jq -r '.ViewID // empty')

  if [[ -z "$view_id" ]]; then
    echo "  ✗ $name — failed to create view"
    return 1
  fi

  # 3. Create Trigger with inline Kyverno policy (if worker available)
  local trigger_id=""
  if [[ -n "$worker_id" ]]; then
    local policy_yaml
    policy_yaml=$($policy_func)
    trigger_id=$(api POST "/space/${SPACE_ID}/trigger?allow_exists=true" \
      -d "$(jq -n \
        --arg slug "${slug}-check" \
        --arg workerId "$worker_id" \
        --arg policy "$policy_yaml" \
        '{
          Slug: $slug,
          Event: "Mutation",
          ToolchainType: "Kubernetes/YAML",
          FunctionName: "vet-kyverno",
          BridgeWorkerID: $workerId,
          Arguments: [{ParameterName: "policy", Value: $policy}],
          Disabled: true
        }'
      )" 2>/dev/null | jq -r '.TriggerID // empty' 2>/dev/null) || true
  fi

  # 4. Patch View with trigger ID
  if [[ -n "$trigger_id" ]]; then
    api PATCH "/space/${SPACE_ID}/view/${view_id}" \
      -d "$(jq -n --arg tid "$trigger_id" '{Annotations: {"campaign-trigger-id": $tid}}')" \
      >/dev/null 2>&1 || true
  fi

  echo "  ✓ $name (priority=$priority, status=$status)"
  created_campaigns=$((created_campaigns + 1))
}

# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════

echo "=== ConfigHub Campaigns Demo ==="
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

# ── Step 2: Detect kyverno worker ────────────────────────────────────────────

WORKER_ID=$(find_kyverno_worker) || true
if [[ -n "$WORKER_ID" ]]; then
  echo "Kyverno worker: $WORKER_ID"
else
  echo "No Ready vet-kyverno worker found — campaigns will be created without triggers."
  echo "See README.md for how to set up a worker."
fi
echo ""

# ── Step 3: Units ─────────────────────────────────────────────────────────────

echo "--- Creating demo units ---"
echo ""

echo "Campaign: Require App Label"
create_unit "auth-proxy"             "std-labels"   "Platform" "$WORKDIR/deployment-good.yaml"
create_unit "config-reloader"        "std-labels"   "Platform" "$WORKDIR/deployment-good.yaml"
create_unit "cert-manager-webhook"   "std-labels"   "Platform" "$WORKDIR/deployment-no-labels.yaml"
create_unit "event-bus"              "std-labels"   "Platform" "$WORKDIR/statefulset-highmem.yaml"
create_unit "secret-rotator"         "std-labels"   "Platform" "$WORKDIR/deployment-good.yaml"
create_unit "cron-cleanup"           "std-labels"   "Platform" "$WORKDIR/cronjob.yaml"

echo ""
echo "Campaign: Resource Limits Enforcement"
create_unit "search-indexer"     "resource-limits" "Platform" "$WORKDIR/deployment-good.yaml"
create_unit "image-processor"    "resource-limits" "Platform" "$WORKDIR/deployment-no-limits.yaml"
create_unit "cache-warmer"       "resource-limits" "Platform" "$WORKDIR/deployment-no-limits.yaml"
create_unit "log-aggregator"     "resource-limits" "Platform" "$WORKDIR/deployment-good.yaml"
create_unit "metrics-collector"  "resource-limits" "Platform" "$WORKDIR/deployment-no-limits.yaml"
create_unit "batch-scheduler"    "resource-limits" "Platform" "$WORKDIR/deployment-good.yaml"
create_unit "audit-logger"       "resource-limits" "Platform" "$WORKDIR/deployment-good.yaml"

echo ""
echo "Campaign: High Availability — Minimum Replicas"
create_unit "api-gateway"        "ha-replicas" "Backend" "$WORKDIR/deployment-good.yaml"
create_unit "order-service"      "ha-replicas" "Backend" "$WORKDIR/deployment-good.yaml"
create_unit "inventory-api"      "ha-replicas" "Backend" "$WORKDIR/deployment-no-probes.yaml"
create_unit "shipping-tracker"   "ha-replicas" "Backend" "$WORKDIR/deployment-root.yaml"

echo ""
echo "Campaign: Liveness and Readiness Probes"
create_unit "checkout-flow"           "probe-compliance" "Backend" "$WORKDIR/deployment-good.yaml"
create_unit "user-profile-svc"        "probe-compliance" "Backend" "$WORKDIR/deployment-good.yaml"
create_unit "recommendation-engine"   "probe-compliance" "Backend" "$WORKDIR/deployment-no-probes.yaml"
create_unit "notification-dispatcher" "probe-compliance" "Backend" "$WORKDIR/statefulset-highmem.yaml"
create_unit "price-calculator"        "probe-compliance" "Backend" "$WORKDIR/deployment-good.yaml"

echo ""
echo "Campaign: Run-As-NonRoot Enforcement"
create_unit "debug-tools"        "pss-restricted" "Security" "$WORKDIR/deployment-root.yaml"
create_unit "legacy-cron-runner" "pss-restricted" "Security" "$WORKDIR/cronjob.yaml"
create_unit "data-pipeline"      "pss-restricted" "Security" "$WORKDIR/deployment-dockerhub.yaml"

echo ""
echo "Campaign: Disallow Latest Image Tag"
create_unit "web-dashboard"     "no-latest-tag" "Frontend" "$WORKDIR/deployment-good.yaml"
create_unit "admin-portal"      "no-latest-tag" "Frontend" "$WORKDIR/deployment-good.yaml"
create_unit "docs-renderer"     "no-latest-tag" "Frontend" "$WORKDIR/deployment-node18.yaml"
create_unit "webhook-processor" "no-latest-tag" "Frontend" "$WORKDIR/deployment-node18.yaml"
create_unit "email-templater"   "no-latest-tag" "Frontend" "$WORKDIR/deployment-node18.yaml"
create_unit "report-generator"  "no-latest-tag" "Frontend" "$WORKDIR/cronjob.yaml"

echo ""
echo "Campaign: Disallow Privileged Containers"
create_unit "graphql-gateway"  "no-privileged" "Security" "$WORKDIR/deployment-good.yaml"
create_unit "billing-service"  "no-privileged" "Security" "$WORKDIR/deployment-no-labels.yaml"
create_unit "session-manager"  "no-privileged" "Security" "$WORKDIR/deployment-good.yaml"
create_unit "feature-flag-svc" "no-privileged" "Security" "$WORKDIR/deployment-good.yaml"

echo ""
echo "Campaign: Read-Only Root Filesystem"
create_unit "ml-training-job"   "readonly-rootfs" "Data Science" "$WORKDIR/statefulset-highmem.yaml"
create_unit "etl-transform"     "readonly-rootfs" "Data Science" "$WORKDIR/deployment-good.yaml"
create_unit "vector-db"         "readonly-rootfs" "Data Science" "$WORKDIR/statefulset-highmem.yaml"
create_unit "embedding-service" "readonly-rootfs" "Data Science" "$WORKDIR/deployment-good.yaml"

echo ""
echo "Campaign: Image Registry Restriction"
create_unit "payment-service"       "registry-restrict" "Security" "$WORKDIR/deployment-good.yaml"
create_unit "fraud-detector"        "registry-restrict" "Security" "$WORKDIR/deployment-good.yaml"
create_unit "cdn-origin"            "registry-restrict" "Security" "$WORKDIR/deployment-dockerhub.yaml"
create_unit "analytics-ingest"      "registry-restrict" "Security" "$WORKDIR/statefulset-highmem.yaml"

echo ""
echo "Campaign: Disallow Host Ports"
create_unit "redis-cluster"    "no-host-ports" "Infrastructure" "$WORKDIR/statefulset-highmem.yaml"
create_unit "kafka-broker"     "no-host-ports" "Infrastructure" "$WORKDIR/statefulset-highmem.yaml"
create_unit "prometheus-stack" "no-host-ports" "Infrastructure" "$WORKDIR/deployment-good.yaml"

echo ""
echo "--- Creating campaigns ---"
echo ""

# ── Step 4: Campaigns ─────────────────────────────────────────────────────────

# 1. Require App Label
create_campaign \
  "Require App Label" \
  "All Deployments must have an 'app' label. Enables service discovery, monitoring dashboards, and CMDB synchronisation." \
  "HIGH" "in_progress" \
  "$(days_from_now 7)" "$(days_ago_iso 8)" "" \
  "Labels.team = 'Platform'" \
  policy_require_labels \
  "{\"passing\":3,\"failing\":3,\"total\":6,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 2. Resource Limits Enforcement
create_campaign \
  "Resource Limits Enforcement" \
  "All workloads must define CPU and memory limits. Required by platform policy POL-2024-11 to prevent resource starvation." \
  "HIGH" "in_progress" \
  "$(days_from_now 10)" "$(days_ago_iso 5)" "" \
  "Labels.team = 'Platform'" \
  policy_require_resource_limits \
  "{\"passing\":4,\"failing\":3,\"total\":7,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 3. High Availability — Minimum Replicas
create_campaign \
  "High Availability - Minimum Replicas" \
  "All production Deployments and StatefulSets must run at least 2 replicas to avoid single points of failure." \
  "MEDIUM" "draft" \
  "$(days_from_now 30)" "$(days_ago_iso 2)" "" \
  "Labels.team = 'Backend'" \
  policy_require_min_replicas \
  "{\"passing\":2,\"failing\":2,\"total\":4,\"checkedAt\":\"$(days_ago_iso 1)\"}" \
  "$WORKER_ID"

# 4. Liveness and Readiness Probe Compliance
create_campaign \
  "Liveness and Readiness Probe Compliance" \
  "All Deployments and StatefulSets must define livenessProbe and readinessProbe on every container." \
  "HIGH" "in_progress" \
  "$(days_from_now 7)" "$(days_ago_iso 6)" "" \
  "Labels.team = 'Backend'" \
  policy_require_probes \
  "{\"passing\":3,\"failing\":2,\"total\":5,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 5. Run-As-NonRoot Enforcement
create_campaign \
  "Run-As-NonRoot Enforcement" \
  "All pods must set runAsNonRoot at the pod or container level to ensure no container runs as UID 0." \
  "MEDIUM" "draft" \
  "$(days_from_now 21)" "$(days_ago_iso 1)" "" \
  "Labels.team = 'Security'" \
  policy_require_run_as_nonroot \
  "{\"passing\":2,\"failing\":1,\"total\":3,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 6. Disallow Latest Image Tag
create_campaign \
  "Disallow Latest Image Tag" \
  "No container may use ':latest' as its image tag. All images must pin an explicit version to ensure reproducible deployments." \
  "HIGH" "in_progress" \
  "$(days_from_now 7)" "$(days_ago_iso 8)" "" \
  "Labels.team = 'Frontend'" \
  policy_disallow_latest_tag \
  "{\"passing\":3,\"failing\":3,\"total\":6,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 7. Disallow Privileged Containers
create_campaign \
  "Disallow Privileged Containers" \
  "No container may run in privileged mode. Enforces Pod Security Standards restricted profile per CIS Benchmark 5.2.2." \
  "HIGH" "in_progress" \
  "$(days_from_now 5)" "$(days_ago_iso 12)" "" \
  "Labels.team = 'Security'" \
  policy_disallow_privileged \
  "{\"passing\":3,\"failing\":1,\"total\":4,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 8. Read-Only Root Filesystem
create_campaign \
  "Read-Only Root Filesystem" \
  "All containers must mount their root filesystem as read-only per CIS Benchmark 5.2.4 and the restricted PSS profile." \
  "MEDIUM" "draft" \
  "$(days_from_now 28)" "$(days_ago_iso 3)" "" \
  "Labels.team = 'Data Science'" \
  policy_require_readonly_rootfs \
  "{\"passing\":2,\"failing\":2,\"total\":4,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 9. Image Registry Restriction
create_campaign \
  "Image Registry Restriction" \
  "All container images must be pulled from approved registries. Docker Hub and other public registries are not permitted." \
  "HIGH" "in_progress" \
  "$(days_from_now 3)" "$(days_ago_iso 14)" "" \
  "Labels.team = 'Security'" \
  policy_restrict_image_registries \
  "{\"passing\":3,\"failing\":1,\"total\":4,\"checkedAt\":\"$(days_ago_iso 0)\"}" \
  "$WORKER_ID"

# 10. Disallow Host Ports (completed)
create_campaign \
  "Disallow Host Ports" \
  "Containers must not bind to host ports. Completed — all workloads now route through ClusterIP Services and Ingress." \
  "LOW" "completed" \
  "$(days_from_now -5)" "$(days_ago_iso 30)" "$(days_ago_iso 3)" \
  "Labels.team = 'Infrastructure'" \
  policy_disallow_host_ports \
  "{\"passing\":3,\"failing\":0,\"total\":3,\"checkedAt\":\"$(days_ago_iso 3)\"}" \
  "$WORKER_ID"

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "=== Done ==="
echo "  Space created:     $SPACE"
echo "  Units created:     $created_units"
echo "  Campaigns created: $created_campaigns"
echo ""
echo "Open the ConfigHub UI and navigate to Campaigns to see them."
if [[ -n "$WORKER_ID" ]]; then
  echo "Triggers were created (disabled). Enable them in campaign settings to run checks."
else
  echo "Re-run after connecting a vet-kyverno worker to add policy check triggers."
fi
echo ""
echo "To clean up, run: ./cleanup.sh"
