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

echo "==> Checking there are 7 distinct Spaces: 1 platform plus 2 per team"
space_count="$(echo "$EXPLAIN_JSON_OUT" | jq '.spaces | length')"
if [[ "$space_count" -ne 7 ]]; then
  echo "Expected 7 Spaces (1 platform, plus a bootstrap and a workloads Space per team), got $space_count" >&2
  exit 1
fi
unique_space_count="$(echo "$EXPLAIN_JSON_OUT" | jq '.spaces | unique | length')"
if [[ "$unique_space_count" -ne 7 ]]; then
  echo "Expected 7 distinct Space names, found duplicates among: $(echo "$EXPLAIN_JSON_OUT" | jq -c '.spaces')" >&2
  echo "A ConfigHub Space belongs to exactly one Component. Reusing one Space" >&2
  echo "for a team's bootstrap and its workloads would re-link that Space to" >&2
  echo "whichever Component uploaded last, and overwrite its Owner and" >&2
  echo "Environment labels along with it." >&2
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
if ! awk '/^  sourceRef:$/{s=1; next} s && /^    name: /{print $2; exit}' "$f" | grep -qx "flux-system"; then
  echo "The platform bootstrap Kustomization should read from the flux-system GitRepository" >&2
  echo "that flux bootstrap generates; no other source is defined in this example." >&2
  exit 1
fi
if grep -q "serviceAccountName:" "$f"; then
  echo "The platform bootstrap Kustomization should not impersonate any tenant" >&2
  exit 1
fi

echo "==> Checking each team has its own namespace, ServiceAccount, Role and RoleBinding"
for team in storefront payments loyalty; do
  f="$VAR_DIR/rendered-bootstrap-$team.yaml"
  grep -q "^kind: Namespace$" "$f"
  grep -q "  name: team-$team$" "$f"
  grep -q "^kind: ServiceAccount$" "$f"
  grep -q "^kind: Role$" "$f"
  grep -q "^kind: RoleBinding$" "$f"
done

# Prints every roleRef in a rendered stream as "<kind> <name>", one per line.
role_refs() {
  awk '
    /^---$/ { in_ref = 0; next }
    /^roleRef:$/ { in_ref = 1; kind = ""; name = ""; next }
    in_ref && /^  kind: / { kind = $2 }
    in_ref && /^  name: / { name = $2 }
    in_ref && /^[^ ]/ { in_ref = 0 }
    in_ref && kind != "" && name != "" { print kind, name; in_ref = 0 }
  ' "$1"
}

# Prints every rule in a rendered Role or ClusterRole that grants anything
# beyond get, list or watch on something the platform owns: NetworkPolicies,
# ResourceQuotas, LimitRanges, Namespaces, anything in the RBAC API group,
# Flux's own objects, or a wildcard. kustomize build prints rules in a fixed
# block layout, which is what this reads.
platform_owned_writes() {
  awk '
    function flush() {
      if (!in_rule) return
      protected = 0
      for (g in groups) if (g == "rbac.authorization.k8s.io" || g ~ /toolkit\.fluxcd\.io$/ || g == "*") protected = 1
      for (r in res) if (r ~ /^(networkpolicies|resourcequotas|limitranges|namespaces|roles|rolebindings|clusterroles|clusterrolebindings|\*)$/) protected = 1
      write = 0
      for (v in verbs) if (v != "get" && v != "list" && v != "watch") write = 1
      if (protected && write) {
        line = ""
        for (g in groups) line = line " group=" (g == "" ? "core" : g)
        for (r in res) line = line " resource=" r
        for (v in verbs) line = line " verb=" v
        print substr(line, 2)
      }
      delete groups; delete res; delete verbs; in_rule = 0
    }
    /^---$/ { flush(); in_role = 0; in_rules = 0; next }
    /^kind: (Cluster)?Role$/ { in_role = 1; next }
    in_role && /^rules:$/ { in_rules = 1; next }
    in_rules && /^[^ -]/ { flush(); in_rules = 0; next }
    in_rules && /^- / { flush(); in_rule = 1; sub(/^- /, "  ") }
    in_rules && /^  apiGroups:/ { section = "g"; next }
    in_rules && /^  resources:/ { section = "r"; next }
    in_rules && /^  verbs:/ { section = "v"; next }
    in_rules && /^  [a-zA-Z]/ { section = ""; next }
    in_rules && /^  - / {
      item = $0; sub(/^  - /, "", item); gsub(/"/, "", item); gsub(/\047/, "", item)
      if (section == "g") groups[item] = 1
      else if (section == "r") res[item] = 1
      else if (section == "v") verbs[item] = 1
    }
    END { flush() }
  ' "$1"
}

echo "==> Checking each team's grant cannot loosen the platform's guardrails"
for team in storefront payments loyalty; do
  f="$VAR_DIR/rendered-bootstrap-$team.yaml"
  refs="$(role_refs "$f")"
  if [[ "$refs" != "Role team-$team-tenant" ]]; then
    echo "team-$team's RoleBinding should bind only to its own Role team-$team-tenant, found: ${refs:-nothing}" >&2
    echo "The built-in admin and edit ClusterRoles let a team write NetworkPolicies." >&2
    echo "NetworkPolicy allow rules are additive, so a team holding either could add an" >&2
    echo "allow-all policy next to the platform's same-namespace policy and undo it." >&2
    exit 1
  fi
  writes="$(platform_owned_writes "$f")"
  if [[ -n "$writes" ]]; then
    echo "team-$team's Role grants writes on something the platform owns:" >&2
    echo "$writes" >&2
    echo "A tenant must be able to read, never change, its NetworkPolicy, ResourceQuota," >&2
    echo "LimitRange, Namespace, RBAC or Flux objects." >&2
    exit 1
  fi
  # The team must still be able to apply what its own workloads/ folder ships.
  for resource in deployments services; do
    if ! awk -v r="$resource" '/^kind: Role$/{in_role=1} /^---$/{in_role=0} in_role && $0 == "  - " r {found=1} END{exit !found}' "$f"; then
      echo "team-$team's Role does not grant $resource, which its workloads need" >&2
      exit 1
    fi
  done
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
