#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the opa-gatekeeper example worker.
#
# Prerequisites:
#   - kind, kubectl, cub, docker installed
#   - Authenticated with: cub auth login
#
# Usage:
#   cd custom-workers/opa-gatekeeper
#   bash demo.sh

set -euo pipefail

# --- Prerequisites -----------------------------------------------------------

for tool in kind kubectl cub docker; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH"
    exit 1
  fi
done

# --- Configuration -----------------------------------------------------------

CLUSTER_NAME="gatekeeper-demo-$(( RANDOM % 9000 + 1000 ))"
SPACE="gatekeeper-demo-$(( RANDOM % 9000 + 1000 ))"
GATEKEEPER_WORKER="gatekeeper-worker"
GATEKEEPER_WORKER_NAMESPACE="gatekeeper-worker"
IMAGE_NAME="opa-gatekeeper-worker:demo"

echo "=== OPA Gatekeeper Demo ==="
echo "Cluster:  $CLUSTER_NAME"
echo "Space:    $SPACE"
echo ""

# --- Cleanup trap ------------------------------------------------------------

cleanup() {
  if [ "${NOCLEANUP:-}" = "1" ]; then
    echo ""
    echo "=== Skipping cleanup (NOCLEANUP=1) ==="
    echo "Cluster: $CLUSTER_NAME"
    echo "Space:   $SPACE"
    return
  fi
  echo ""
  echo "=== Cleaning up ==="
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT

# --- Create kind cluster -----------------------------------------------------

echo "--- Creating kind cluster ---"
kind create cluster --name "$CLUSTER_NAME"
kubectl cluster-info --context "kind-$CLUSTER_NAME"
echo ""

# --- Build and load Docker image (while cluster is fresh) --------------------

echo "--- Building Docker image ---"
docker build -t "$IMAGE_NAME" .

echo "--- Loading image into kind ---"
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space ---"
cub space create "$SPACE"
echo ""

# --- Install Gatekeeper -------------------------------------------------------

echo "--- Installing OPA Gatekeeper ---"
kubectl apply -f \
  https://raw.githubusercontent.com/open-policy-agent/gatekeeper/v3.17.1/deploy/gatekeeper.yaml

echo "Waiting for Gatekeeper to be ready..."
kubectl -n gatekeeper-system rollout status deployment/gatekeeper-audit --timeout=120s
kubectl -n gatekeeper-system rollout status deployment/gatekeeper-controller-manager --timeout=120s
echo "Gatekeeper is ready."
echo ""

# --- Create Constraint Template and Constraint --------------------------------

echo "--- Creating ConstraintTemplate (K8sRequiredLabels) ---"
kubectl apply -f - <<'TEMPLATE'
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels
spec:
  crd:
    spec:
      names:
        kind: K8sRequiredLabels
      validation:
        openAPIV3Schema:
          type: object
          properties:
            labels:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package k8srequiredlabels
        violation[{"msg": msg}] {
          provided := {label | input.review.object.metadata.labels[label]}
          required := {label | label := input.parameters.labels[_]}
          missing := required - provided
          count(missing) > 0
          msg := sprintf("Missing required labels: %v", [missing])
        }
TEMPLATE
# Gatekeeper creates the constraint's CRD from the template; wait for it.
for _ in $(seq 1 60); do
  kubectl get crd k8srequiredlabels.constraints.gatekeeper.sh &>/dev/null && break
  sleep 2
done
kubectl wait --for=condition=established crd/k8srequiredlabels.constraints.gatekeeper.sh --timeout=60s
echo ""

echo "--- Creating Constraint (require-team-label) ---"
kubectl apply -f - <<'CONSTRAINT'
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sRequiredLabels
metadata:
  name: require-team-label
spec:
  match:
    kinds:
      - apiGroups: ["apps"]
        kinds: ["Deployment"]
    excludedNamespaces:
      - gatekeeper-system
      - gatekeeper-worker
      - confighub
      - kube-system
  parameters:
    labels:
      - team
CONSTRAINT
echo ""

# --- Deploy gatekeeper function executor -------------------------------------

echo "--- Deploying gatekeeper function executor ---"
# The function executor lists ValidatingWebhookConfigurations to find Gatekeeper's webhooks.
kubectl create clusterrole gatekeeper-webhook-reader \
  --verb=list,watch --resource=validatingwebhookconfigurations.admissionregistration.k8s.io
kubectl create clusterrolebinding gatekeeper-worker-webhook-reader \
  --clusterrole=gatekeeper-webhook-reader \
  --group="system:serviceaccounts:$GATEKEEPER_WORKER_NAMESPACE"

# A Secret with the Worker's credentials and a Deployment that reads it.
"$(dirname "$0")/../deploy-worker.sh" "$SPACE" "$GATEKEEPER_WORKER" "$GATEKEEPER_WORKER_NAMESPACE" "$IMAGE_NAME" \
  GATEKEEPER_URL=https://gatekeeper-webhook-service.gatekeeper-system.svc:443 \
  GATEKEEPER_SKIP_TLS_VERIFY=true
echo "Gatekeeper function executor is ready."
echo ""

# --- Create test units -------------------------------------------------------

echo "--- Creating test unit: deploy-with-labels (should pass) ---"
cub unit create --space "$SPACE" deploy-with-labels - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: good-deploy
  namespace: default
  labels:
    team: platform
    app: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: nginx
          image: nginx:1.21
UNIT
echo ""

echo "--- Creating test unit: deploy-without-labels (should fail require-team-label) ---"
cub unit create --space "$SPACE" deploy-without-labels - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bad-deploy
  namespace: default
  labels:
    app: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: nginx
          image: nginx:1.21
UNIT
echo ""

# --- Wait for Gatekeeper webhook to be fully ready ---------------------------

echo "--- Waiting for Gatekeeper webhook endpoints ---"
kubectl -n gatekeeper-system get endpoints gatekeeper-webhook-service
sleep 10

# --- Run validation ----------------------------------------------------------

echo "--- Running vet-opa-gatekeeper on deploy-with-labels (should pass) ---"
cub function do vet-opa-gatekeeper --space "$SPACE" --unit deploy-with-labels --worker "$SPACE/$GATEKEEPER_WORKER"
echo ""

echo "--- Running vet-opa-gatekeeper on deploy-without-labels (should fail) ---"
cub function do vet-opa-gatekeeper --space "$SPACE" --unit deploy-without-labels --worker "$SPACE/$GATEKEEPER_WORKER"
echo ""

echo "=== Demo complete ==="
