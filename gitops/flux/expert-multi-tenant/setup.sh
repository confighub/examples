#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
PLATFORM_SPACE="${GITOPS_FLUX_MULTI_TENANT_PLATFORM_SPACE:-gitops-flux-multi-tenant-platform}"
STOREFRONT_SPACE="${GITOPS_FLUX_MULTI_TENANT_STOREFRONT_SPACE:-gitops-flux-multi-tenant-team-storefront}"
PAYMENTS_SPACE="${GITOPS_FLUX_MULTI_TENANT_PAYMENTS_SPACE:-gitops-flux-multi-tenant-team-payments}"
LOYALTY_SPACE="${GITOPS_FLUX_MULTI_TENANT_LOYALTY_SPACE:-gitops-flux-multi-tenant-team-loyalty}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders a single-cluster Flux multi-tenancy repo with
kustomize, once for the platform's cluster-level bootstrap and once per
team, then uploads each render into its own ConfigHub Space with
"cub variant upload". It does not bootstrap Flux and does not touch a
Kubernetes cluster.
EOF_USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain)
      EXPLAIN=1
      shift
      ;;
    --explain-json)
      EXPLAIN_JSON=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$EXPLAIN" -eq 1 ]]; then
  cat <<EOF_PLAN
This is a read-only setup plan for gitops/flux/expert-multi-tenant.
Nothing will be mutated.

Conceptual model (one cluster, one platform layer, three tenants):

  clusters/shared/tenants.yaml   -> tenants/base   (platform bootstrap, no serviceAccountName)
    tenants/base/team-storefront/
      rbac.yaml, guardrails.yaml   -> Namespace, ServiceAccount, RoleBinding, ResourceQuota, NetworkPolicy
      sync.yaml                    -> the tenant's own GitRepository + Kustomization,
                                       serviceAccountName + targetNamespace: team-storefront
      workloads/                   -> the tenant's own app, reconciled by sync.yaml, not by this Kustomization
    tenants/base/team-payments/    -> same shape
    tenants/base/team-loyalty/     -> same shape

  Least-privilege model: the platform's own Kustomization creates every
  team's namespace, ServiceAccount and RoleBinding. Each team's own
  Kustomization then reconciles only inside its own namespace, impersonating
  its own ServiceAccount. No team's Kustomization can reach another team's
  namespace, because no RoleBinding anywhere grants it.

This example will:
- render clusters/shared with "kustomize build"
- render tenants/base/team-storefront, team-payments and team-loyalty
  (their rbac.yaml, guardrails.yaml and sync.yaml)
- render each team's workloads/ folder separately
- upload the cluster-level render into ConfigHub Space "$PLATFORM_SPACE"
- upload each team's bootstrap and workloads renders into that team's own
  Space ("$STOREFRONT_SPACE", "$PAYMENTS_SPACE", "$LOYALTY_SPACE") using
  "cub variant upload --component <tenant-bootstrap|tenant-workloads> --variant <team> --owner <platform|team>"

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$PLATFORM_SPACE" with the cluster-level render
- creates (or updates) Space "$STOREFRONT_SPACE" with team-storefront's
  bootstrap and workloads
- creates (or updates) Space "$PAYMENTS_SPACE" with team-payments' bootstrap
  and workloads
- creates (or updates) Space "$LOYALTY_SPACE" with team-loyalty's bootstrap
  and workloads

This example never runs "flux bootstrap", never reconciles anything, and
never creates or mutates a live Kubernetes cluster.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg platformSpace "$PLATFORM_SPACE" \
    --arg storefrontSpace "$STOREFRONT_SPACE" \
    --arg paymentsSpace "$PAYMENTS_SPACE" \
    --arg loyaltySpace "$LOYALTY_SPACE" \
    '{
      example_name: "gitops-flux-multi-tenant",
      mutates: false,
      mutates_confighub: true,
      mutates_live_infra: false,
      spaces: [$platformSpace, $storefrontSpace, $paymentsSpace, $loyaltySpace],
      units: [
        "cluster-control",
        "tenant-bootstrap-storefront", "tenant-workloads-storefront",
        "tenant-bootstrap-payments", "tenant-workloads-payments",
        "tenant-bootstrap-loyalty", "tenant-workloads-loyalty"
      ],
      cluster: "shared",
      teams: ["team-storefront", "team-payments", "team-loyalty"],
      namespaces: ["flux-system", "team-storefront", "team-payments", "team-loyalty"],
      apps: ["storefront", "payments-api", "loyalty-api"],
      evaluation_modes: {
        fast_preview: {
          mutates: false,
          commands: ["./setup.sh --explain", "./setup.sh --explain-json | jq"]
        },
        fast_operational_evaluation: {
          mutates_confighub: true,
          mutates_live_infra: false,
          commands: ["./setup.sh", "./verify.sh"],
          stop_before_cleanup: true
        }
      }
    }'
  exit 0
fi

require_cmd kustomize
require_cmd cub

mkdir -p "$VAR_DIR"

kustomize build "$SCRIPT_DIR/clusters/shared" > "$VAR_DIR/rendered-cluster-control.yaml"

for team in storefront payments loyalty; do
  kustomize build "$SCRIPT_DIR/tenants/base/team-$team" > "$VAR_DIR/rendered-bootstrap-$team.yaml"
  kustomize build "$SCRIPT_DIR/tenants/base/team-$team/workloads" > "$VAR_DIR/rendered-workloads-$team.yaml"
done

cub variant upload \
  --component cluster-control --variant shared --environment Control --owner platform \
  --space "$PLATFORM_SPACE" \
  "$VAR_DIR/rendered-cluster-control.yaml"

upload_team() {
  local team="$1"
  local space="$2"
  cub variant upload \
    --component tenant-bootstrap --variant "$team" --environment Platform --owner platform \
    --space "$space" \
    "$VAR_DIR/rendered-bootstrap-$team.yaml"
  cub variant upload \
    --component tenant-workloads --variant "$team" --environment Team --owner "team-$team" \
    --namespace "team-$team" \
    --space "$space" \
    "$VAR_DIR/rendered-workloads-$team.yaml"
  echo "Uploaded team-$team's bootstrap and workloads to Space $space."
}

upload_team storefront "$STOREFRONT_SPACE"
upload_team payments "$PAYMENTS_SPACE"
upload_team loyalty "$LOYALTY_SPACE"

echo "Uploaded the cluster-level render to Space $PLATFORM_SPACE."
echo "Next: ./verify.sh"
