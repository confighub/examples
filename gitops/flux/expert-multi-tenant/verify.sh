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
  echo "Runs offline checks only: no cluster, no Flux, no ConfigHub calls." >&2
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

mkdir -p "$VAR_DIR"

echo "==> Checking that setup.sh --explain-json is valid JSON with the expected fields"
EXPLAIN_JSON_OUT="$("$SCRIPT_DIR/setup.sh" --explain-json)"
echo "$EXPLAIN_JSON_OUT" | jq . >/dev/null

for field in example_name mutates mutates_confighub mutates_live_infra spaces units cluster teams namespaces apps evaluation_modes; do
  if ! echo "$EXPLAIN_JSON_OUT" | jq -e "has(\"$field\")" >/dev/null; then
    echo "setup.sh --explain-json is missing field: $field" >&2
    exit 1
  fi
done

name="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.example_name')"
if [[ "$name" != "gitops-flux-multi-tenant" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

echo "==> Rendering the cluster-level layer and every team's bootstrap and workloads"
kustomize build "$SCRIPT_DIR/clusters/shared" > "$VAR_DIR/rendered-cluster-control.yaml"
for team in storefront payments loyalty; do
  kustomize build "$SCRIPT_DIR/tenants/base/team-$team" > "$VAR_DIR/rendered-bootstrap-$team.yaml"
  kustomize build "$SCRIPT_DIR/tenants/base/team-$team/workloads" > "$VAR_DIR/rendered-workloads-$team.yaml"
done

echo "==> Checking the platform bootstrap layer runs with the platform's own identity"
f="$VAR_DIR/rendered-cluster-control.yaml"
grep -q "^kind: Kustomization$" "$f"
grep -q "  name: tenants$" "$f"
grep -q "path: ./gitops/flux/expert-multi-tenant/tenants/base" "$f"
if grep -q "serviceAccountName:" "$f"; then
  echo "The platform bootstrap Kustomization should not impersonate any tenant" >&2
  exit 1
fi

echo "==> Checking each team has its own namespace, ServiceAccount and RoleBinding"
for team in storefront payments loyalty; do
  f="$VAR_DIR/rendered-bootstrap-$team.yaml"
  grep -q "^kind: Namespace$" "$f"
  grep -q "  name: team-$team$" "$f"
  grep -q "^kind: ServiceAccount$" "$f"
  grep -q "^kind: RoleBinding$" "$f"
  if ! grep -q "  name: admin$" "$f"; then
    echo "team-$team's RoleBinding should bind to the built-in admin ClusterRole" >&2
    exit 1
  fi
done

echo "==> Checking each team has its own guardrails: a ResourceQuota and a same-namespace-only NetworkPolicy"
for team in storefront payments loyalty; do
  f="$VAR_DIR/rendered-bootstrap-$team.yaml"
  grep -q "^kind: ResourceQuota$" "$f"
  grep -q "^kind: NetworkPolicy$" "$f"
  if grep -q "namespaceSelector:" "$f"; then
    echo "team-$team's NetworkPolicy should not admit traffic from other namespaces" >&2
    exit 1
  fi
done

echo "==> Checking every team's own Kustomization impersonates its own ServiceAccount and targets its own namespace"
for team in storefront payments loyalty; do
  f="$VAR_DIR/rendered-bootstrap-$team.yaml"
  grep -q "^kind: GitRepository$" "$f"
  if ! grep -q "  serviceAccountName: team-$team$" "$f"; then
    echo "team-$team's Kustomization does not impersonate its own ServiceAccount" >&2
    echo "This is the tenant-escape break: a Kustomization with no serviceAccountName," >&2
    echo "or one naming another team, would reconcile with the wrong identity." >&2
    exit 1
  fi
  if ! grep -q "  targetNamespace: team-$team$" "$f"; then
    echo "team-$team's Kustomization does not target its own namespace" >&2
    echo "This is the tenant-escape break this example ships: on a live cluster," >&2
    echo "team-$team's RoleBinding only grants rights inside team-$team, so a" >&2
    echo "Kustomization that targets another team's namespace is refused by" >&2
    echo "Kubernetes RBAC (a Forbidden error), and Flux reports that" >&2
    echo "Kustomization as not Ready. This check is the offline stand-in for" >&2
    echo "that refusal: it catches the same misconfiguration before anything" >&2
    echo "is applied anywhere." >&2
    exit 1
  fi
done

echo "==> Checking each team's workloads render with the expected apps"
grep -q "  name: storefront$" "$VAR_DIR/rendered-workloads-storefront.yaml"
grep -q "^kind: Service$" "$VAR_DIR/rendered-workloads-storefront.yaml"
grep -q "  name: payments-api$" "$VAR_DIR/rendered-workloads-payments.yaml"
grep -q "^kind: Service$" "$VAR_DIR/rendered-workloads-payments.yaml"
grep -q "  name: loyalty-api$" "$VAR_DIR/rendered-workloads-loyalty.yaml"
if grep -q "^kind: Service$" "$VAR_DIR/rendered-workloads-loyalty.yaml"; then
  echo "team-loyalty's workloads should carry a Deployment only, no Service" >&2
  exit 1
fi

echo "==> Checking every path a Kustomization points at exists in this repo"
while IFS= read -r p; do
  rel="${p#./gitops/flux/expert-multi-tenant/}"
  if [[ ! -f "$SCRIPT_DIR/$rel/kustomization.yaml" ]]; then
    echo "Kustomization path $p has no kustomization.yaml" >&2
    exit 1
  fi
done < <(grep -rho "path: \./gitops/flux/expert-multi-tenant/[a-z/-]*" "$SCRIPT_DIR/clusters" "$SCRIPT_DIR/tenants/base" | awk '{print $2}' | sort -u)

echo "All gitops-flux-multi-tenant checks passed."
