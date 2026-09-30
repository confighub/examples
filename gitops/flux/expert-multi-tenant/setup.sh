#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
PLATFORM_SPACE="${GITOPS_FLUX_MULTI_TENANT_PLATFORM_SPACE:-gitops-flux-multi-tenant-platform}"
STOREFRONT_BOOTSTRAP_SPACE="${GITOPS_FLUX_MULTI_TENANT_STOREFRONT_BOOTSTRAP_SPACE:-gitops-flux-multi-tenant-team-storefront-bootstrap}"
STOREFRONT_WORKLOADS_SPACE="${GITOPS_FLUX_MULTI_TENANT_STOREFRONT_WORKLOADS_SPACE:-gitops-flux-multi-tenant-team-storefront-workloads}"
PAYMENTS_BOOTSTRAP_SPACE="${GITOPS_FLUX_MULTI_TENANT_PAYMENTS_BOOTSTRAP_SPACE:-gitops-flux-multi-tenant-team-payments-bootstrap}"
PAYMENTS_WORKLOADS_SPACE="${GITOPS_FLUX_MULTI_TENANT_PAYMENTS_WORKLOADS_SPACE:-gitops-flux-multi-tenant-team-payments-workloads}"
LOYALTY_BOOTSTRAP_SPACE="${GITOPS_FLUX_MULTI_TENANT_LOYALTY_BOOTSTRAP_SPACE:-gitops-flux-multi-tenant-team-loyalty-bootstrap}"
LOYALTY_WORKLOADS_SPACE="${GITOPS_FLUX_MULTI_TENANT_LOYALTY_WORKLOADS_SPACE:-gitops-flux-multi-tenant-team-loyalty-workloads}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders a single-cluster Flux multi-tenancy repo with
kustomize, once for the platform's cluster-level bootstrap and twice per
team (its platform-authored bootstrap, its own workloads), then uploads
each render into its own ConfigHub Space with "cub variant upload". A
ConfigHub Space belongs to exactly one Component, so the platform's
bootstrap and a team's workloads always go to separate Spaces, even for the
same team; that separation is also what lets their ownership and access
differ. This script does not bootstrap Flux and does not touch a
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
      rbac.yaml, guardrails.yaml   -> Namespace, ServiceAccount, Role, RoleBinding, ResourceQuota, NetworkPolicy
      sync.yaml                    -> the tenant's own GitRepository + Kustomization,
                                       serviceAccountName + targetNamespace: team-storefront
      workloads/                   -> the tenant's own app, reconciled by sync.yaml, not by this Kustomization
    tenants/base/team-payments/    -> same shape
    tenants/base/team-loyalty/     -> same shape

  Least-privilege model: the platform's own Kustomization creates every
  team's namespace, ServiceAccount, Role and RoleBinding. Each team's own
  Kustomization then reconciles only inside its own namespace, impersonating
  its own ServiceAccount. No team's Kustomization can reach another team's
  namespace, because no RoleBinding anywhere grants it. Each team's Role
  can read, but not write, its NetworkPolicy, ResourceQuota and LimitRange,
  so a team cannot loosen the platform's guardrails from its own folder.

  Space model: a ConfigHub Space belongs to exactly one Component, so this
  example gives each team two Spaces, not one: a bootstrap Space (Component
  tenant-bootstrap, Owner platform) and a workloads Space (Component
  tenant-workloads, Owner team-<name>). That keeps the platform's guardrails
  and the team's own app in Spaces with different, correct ownership,
  instead of one upload silently re-labelling the other's Space.

This example will:
- render clusters/shared with "kustomize build"
- render tenants/base/team-storefront, team-payments and team-loyalty
  (their rbac.yaml, guardrails.yaml and sync.yaml)
- render each team's workloads/ folder separately
- upload the cluster-level render into ConfigHub Space "$PLATFORM_SPACE"
- upload each team's bootstrap render into that team's own bootstrap Space,
  and each team's workloads render into that team's own workloads Space,
  using "cub variant upload --component <tenant-bootstrap|tenant-workloads> --variant <team> --owner <platform|team-name>"

Units: "cub variant upload" makes one Unit per rendered resource, not one per
upload. A team's bootstrap upload becomes a Unit each for its Namespace,
ServiceAccount, Role, RoleBinding, ResourceQuota, NetworkPolicy, GitRepository
and Kustomization. ./verify.sh prints the resource count for each render.

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$PLATFORM_SPACE" with the cluster-level render
- creates (or updates) Space "$STOREFRONT_BOOTSTRAP_SPACE" with
  team-storefront's platform-authored bootstrap
- creates (or updates) Space "$STOREFRONT_WORKLOADS_SPACE" with
  team-storefront's own workloads
- creates (or updates) Space "$PAYMENTS_BOOTSTRAP_SPACE" with
  team-payments' platform-authored bootstrap
- creates (or updates) Space "$PAYMENTS_WORKLOADS_SPACE" with
  team-payments' own workloads
- creates (or updates) Space "$LOYALTY_BOOTSTRAP_SPACE" with
  team-loyalty's platform-authored bootstrap
- creates (or updates) Space "$LOYALTY_WORKLOADS_SPACE" with
  team-loyalty's own workloads

This example never runs "flux bootstrap", never reconciles anything, and
never creates or mutates a live Kubernetes cluster.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg platformSpace "$PLATFORM_SPACE" \
    --arg storefrontBootstrapSpace "$STOREFRONT_BOOTSTRAP_SPACE" \
    --arg storefrontWorkloadsSpace "$STOREFRONT_WORKLOADS_SPACE" \
    --arg paymentsBootstrapSpace "$PAYMENTS_BOOTSTRAP_SPACE" \
    --arg paymentsWorkloadsSpace "$PAYMENTS_WORKLOADS_SPACE" \
    --arg loyaltyBootstrapSpace "$LOYALTY_BOOTSTRAP_SPACE" \
    --arg loyaltyWorkloadsSpace "$LOYALTY_WORKLOADS_SPACE" \
    '{
      example_name: "gitops-flux-multi-tenant",
      mutates: false,
      mutates_confighub: true,
      mutates_live_infra: false,
      spaces: [
        $platformSpace,
        $storefrontBootstrapSpace, $storefrontWorkloadsSpace,
        $paymentsBootstrapSpace, $paymentsWorkloadsSpace,
        $loyaltyBootstrapSpace, $loyaltyWorkloadsSpace
      ],
      uploads: [
        {space: $platformSpace, component: "cluster-control", variant: "shared", owner: "platform"},
        {space: $storefrontBootstrapSpace, component: "tenant-bootstrap", variant: "storefront", owner: "platform"},
        {space: $storefrontWorkloadsSpace, component: "tenant-workloads", variant: "storefront", owner: "team-storefront"},
        {space: $paymentsBootstrapSpace, component: "tenant-bootstrap", variant: "payments", owner: "platform"},
        {space: $paymentsWorkloadsSpace, component: "tenant-workloads", variant: "payments", owner: "team-payments"},
        {space: $loyaltyBootstrapSpace, component: "tenant-bootstrap", variant: "loyalty", owner: "platform"},
        {space: $loyaltyWorkloadsSpace, component: "tenant-workloads", variant: "loyalty", owner: "team-loyalty"}
      ],
      units_per_upload: "one Unit per rendered resource, created by cub variant upload",
      cluster: "shared",
      teams: ["team-storefront", "team-payments", "team-loyalty"],
      namespaces: ["flux-system", "team-storefront", "team-payments", "team-loyalty"],
      apps: ["storefront", "payments-api", "loyalty-api"],
      space_per_component: true,
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

# Each team gets two Spaces, never one: a ConfigHub Space belongs to exactly
# one Component, so uploading tenant-bootstrap and tenant-workloads into the
# same Space would re-link that Space to whichever Component uploaded last,
# and overwrite its labels (Owner, Environment) along with it. Separate
# Spaces keep the platform's bootstrap and the team's own workloads each in
# a Space with the right owner.
upload_team() {
  local team="$1"
  local bootstrap_space="$2"
  local workloads_space="$3"
  cub variant upload \
    --component tenant-bootstrap --variant "$team" --environment Platform --owner platform \
    --space "$bootstrap_space" \
    "$VAR_DIR/rendered-bootstrap-$team.yaml"
  cub variant upload \
    --component tenant-workloads --variant "$team" --environment Team --owner "team-$team" \
    --namespace "team-$team" \
    --space "$workloads_space" \
    "$VAR_DIR/rendered-workloads-$team.yaml"
  echo "Uploaded team-$team's bootstrap to Space $bootstrap_space and workloads to Space $workloads_space."
}

upload_team storefront "$STOREFRONT_BOOTSTRAP_SPACE" "$STOREFRONT_WORKLOADS_SPACE"
upload_team payments "$PAYMENTS_BOOTSTRAP_SPACE" "$PAYMENTS_WORKLOADS_SPACE"
upload_team loyalty "$LOYALTY_BOOTSTRAP_SPACE" "$LOYALTY_WORKLOADS_SPACE"

echo "Uploaded the cluster-level render to Space $PLATFORM_SPACE."
echo "Next: ./verify.sh"
