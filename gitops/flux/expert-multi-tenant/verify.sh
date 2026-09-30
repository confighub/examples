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

# Fails with a message when a rendered file lacks a line matching a pattern.
# A bare "grep -q" under "set -e" would abort with no output at all.
need() {
  local file="$1" pattern="$2" message="$3"
  if ! grep -q -- "$pattern" "$file"; then
    echo "$message" >&2
    exit 1
  fi
}

require_cmd kustomize
require_cmd jq

mkdir -p "$VAR_DIR"

echo "==> Checking that setup.sh --explain-json is valid JSON with the expected fields"
EXPLAIN_JSON_OUT="$("$SCRIPT_DIR/setup.sh" --explain-json)"
echo "$EXPLAIN_JSON_OUT" | jq . >/dev/null

for field in example_name mutates mutates_confighub mutates_live_infra spaces uploads cluster teams namespaces apps evaluation_modes; do
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

echo "==> Counting the Units each upload would create (cub variant upload makes one Unit per resource)"
for f in "$VAR_DIR"/rendered-*.yaml; do
  echo "    $(basename "$f"): $(grep -c '^kind: ' "$f") resources"
done

echo "==> Checking the platform bootstrap layer runs with the platform's own identity"
f="$VAR_DIR/rendered-cluster-control.yaml"
need "$f" "^kind: Kustomization$" "clusters/shared renders no Flux Kustomization"
need "$f" "  name: tenants$" "clusters/shared should hold the Kustomization named tenants"
need "$f" "path: ./gitops/flux/expert-multi-tenant/tenants/base" "The tenants Kustomization should point at ./gitops/flux/expert-multi-tenant/tenants/base"
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
  need "$f" "^kind: Namespace$" "team-$team's bootstrap has no Namespace"
  need "$f" "  name: team-$team$" "team-$team's bootstrap has nothing named team-$team"
  need "$f" "^kind: ServiceAccount$" "team-$team's bootstrap has no ServiceAccount"
  need "$f" "^kind: Role$" "team-$team's bootstrap has no Role"
  need "$f" "^kind: RoleBinding$" "team-$team's bootstrap has no RoleBinding"
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

# Prints one line for every way a rendered binding is not scoped to team $2:
# a ClusterRoleBinding at all, a RoleBinding outside the team's namespace, or
# any subject other than the team's own ServiceAccount in the team's own
# namespace. Prints nothing when every binding is the team's own. Reads the
# fixed block layout kustomize build prints: subjects are a "- kind:" list at
# the top level, metadata.namespace is indented two spaces.
binding_violations() {
  awk -v team="$2" '
    function flush(   i, who) {
      if (kind == "ClusterRoleBinding") {
        print "ClusterRoleBinding " bname ": a tenant is bound by a namespaced RoleBinding, never a cluster-wide one"
      } else if (kind == "RoleBinding") {
        if (bns != team)
          print "RoleBinding " bname " is in namespace " (bns == "" ? "(none)" : bns) ", not " team
        if (nsub == 0)
          print "RoleBinding " bname " has no subjects"
        for (i = 1; i <= nsub; i++) {
          if (sk[i] != "ServiceAccount" || sn[i] != team || sns[i] != team) {
            who = sk[i] " " (sns[i] == "" ? "" : sns[i] "/") sn[i]
            print "RoleBinding " bname " has a subject that is not " team "\047s own ServiceAccount: " who
          }
        }
      }
      kind = ""; bname = ""; bns = ""; nsub = 0; section = ""
    }
    /^---$/ { flush(); next }
    /^kind: / { kind = $2; next }
    /^[a-zA-Z]/ { section = $1; sub(/:$/, "", section); next }
    section == "metadata" && /^  name: / { bname = $2; next }
    section == "metadata" && /^  namespace: / { bns = $2; next }
    section == "subjects" && /^- / { nsub++; sk[nsub] = ""; sn[nsub] = ""; sns[nsub] = "" }
    section == "subjects" && /^(- |  )kind: / { sk[nsub] = $NF }
    section == "subjects" && /^(- |  )name: / { sn[nsub] = $NF }
    section == "subjects" && /^(- |  )namespace: / { sns[nsub] = $NF }
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
  violations="$(binding_violations "$f" "team-$team")"
  if [[ -n "$violations" ]]; then
    echo "team-$team's role binding is not scoped to team-$team alone:" >&2
    echo "$violations" >&2
    echo "Every RoleBinding in a team's bootstrap must sit in that team's namespace and" >&2
    echo "name only that team's own ServiceAccount as its subject. A foreign subject" >&2
    echo "gives another team's Kustomization this team's rights, and this team's" >&2
    echo "namespace is no longer its own." >&2
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
  if ! grep -q "^kind: ResourceQuota$" "$f"; then
    echo "team-$team has no ResourceQuota in its rendered bootstrap" >&2
    echo "Without one, this team can consume as much of the shared cluster as it likes." >&2
    exit 1
  fi
  if ! grep -q "^kind: NetworkPolicy$" "$f"; then
    echo "team-$team has no NetworkPolicy in its rendered bootstrap" >&2
    echo "Without one, pods in other namespaces can reach this team's pods." >&2
    exit 1
  fi
  if grep -q "namespaceSelector:" "$f"; then
    echo "team-$team's NetworkPolicy should not admit traffic from other namespaces" >&2
    exit 1
  fi
done

echo "==> Checking every team's own Kustomization impersonates its own ServiceAccount and targets its own namespace"
for team in storefront payments loyalty; do
  f="$VAR_DIR/rendered-bootstrap-$team.yaml"
  need "$f" "^kind: GitRepository$" "team-$team's bootstrap has no GitRepository for its own source"
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
need "$VAR_DIR/rendered-workloads-storefront.yaml" "  name: storefront$" "team-storefront's workloads should render the storefront app"
need "$VAR_DIR/rendered-workloads-storefront.yaml" "^kind: Service$" "team-storefront's workloads should render a Service"
need "$VAR_DIR/rendered-workloads-payments.yaml" "  name: payments-api$" "team-payments' workloads should render the payments-api app"
need "$VAR_DIR/rendered-workloads-payments.yaml" "^kind: Service$" "team-payments' workloads should render a Service"
need "$VAR_DIR/rendered-workloads-loyalty.yaml" "  name: loyalty-api$" "team-loyalty's workloads should render the loyalty-api app"
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
