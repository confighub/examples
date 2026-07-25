#!/usr/bin/env bash
# Read-only. Reports whether this organization has the dimensions configboard slices
# by, so an empty dashboard is explained before it is opened rather than after.
#
#   ./preflight.sh          human-readable
#   ./preflight.sh --json   machine-readable
#
# Mutates nothing: every call below is a list.
set -euo pipefail

JSON=false
[[ "${1:-}" == "--json" ]] && JSON=true

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "error: $1 not found in PATH" >&2; exit 1; }
}
need cub
need jq

# Fail early and clearly if cub is not authenticated, rather than reporting zeros.
if ! cub space list -o json >/dev/null 2>&1; then
  echo "error: cub is not authenticated against a reachable instance." >&2
  echo "       run: cub auth login" >&2
  exit 1
fi

SPACES_JSON=$(cub space list -o json)
TARGETS_JSON=$(cub target list --space "*" -o json 2>/dev/null || echo '[]')
UNITS_JSON=$(cub unit list --space "*" -o json --select "Slug,TargetID,ToolchainType" 2>/dev/null || echo '[]')

space_count=$(jq 'length' <<<"$SPACES_JSON")
target_count=$(jq 'length' <<<"$TARGETS_JSON")
unit_count=$(jq 'length' <<<"$UNITS_JSON")

# Standard Space labels. Cluster is deliberately absent: it is a Target reference
# (Space.ReleaseTargetID), not a label.
label_count() {
  jq --arg k "$1" '[.[] | (.Space // .) | .Labels // {} | select(has($k))] | length' <<<"$SPACES_JSON"
}
component_spaces=$(label_count Component)
environment_spaces=$(label_count Environment)
region_spaces=$(label_count Region)

label_values() {
  jq -c --arg k "$1" '[.[] | (.Space // .) | .Labels // {} | .[$k] // empty] | unique' <<<"$SPACES_JSON"
}

units_with_target=$(jq '[.[] | (.Unit // .) | select((.TargetID // "") != "")] | length' <<<"$UNITS_JSON")
targets_with_facts=$(jq '[.[] | (.Target // .) | select((.Facts // {}) | has("Cluster.KubernetesVersion"))] | length' <<<"$TARGETS_JSON")
spaces_with_release_target=$(jq '[.[] | (.Space // .) | select((.ReleaseTargetID // "") != "")] | length' <<<"$SPACES_JSON")

if $JSON; then
  jq -n \
    --argjson spaces "$space_count" \
    --argjson units "$unit_count" \
    --argjson targets "$target_count" \
    --argjson component_spaces "$component_spaces" \
    --argjson environment_spaces "$environment_spaces" \
    --argjson region_spaces "$region_spaces" \
    --argjson units_with_target "$units_with_target" \
    --argjson spaces_with_release_target "$spaces_with_release_target" \
    --argjson targets_with_cluster_facts "$targets_with_facts" \
    --argjson environments "$(label_values Environment)" \
    --argjson components "$(label_values Component)" \
    --argjson regions "$(label_values Region)" \
    '{
      example_name: "configboard",
      mutates: false,
      mutates_confighub: false,
      mutates_live_infra: false,
      counts: { spaces: $spaces, units: $units, targets: $targets },
      dimensions: {
        Component: { spaces: $component_spaces, values: $components },
        Environment: { spaces: $environment_spaces, values: $environments },
        Region: { spaces: $region_spaces, values: $regions }
      },
      cluster_dimension: {
        units_with_target: $units_with_target,
        spaces_with_release_target: $spaces_with_release_target,
        targets_with_cluster_facts: $targets_with_cluster_facts
      }
    }'
  exit 0
fi

echo "configboard preflight (read-only)"
echo
printf '  %-34s %s\n' "Spaces" "$space_count"
printf '  %-34s %s\n' "Units" "$unit_count"
printf '  %-34s %s\n' "Targets" "$target_count"
echo
echo "  Space labels (what dashboards slice by)"
printf '    %-32s %s of %s Spaces\n' "Component" "$component_spaces" "$space_count"
printf '    %-32s %s of %s Spaces\n' "Environment" "$environment_spaces" "$space_count"
printf '    %-32s %s of %s Spaces\n' "Region" "$region_spaces" "$space_count"
echo
echo "  Cluster dimension (a Target reference, not a label)"
printf '    %-32s %s of %s Units\n' "bound to a Target" "$units_with_target" "$unit_count"
printf '    %-32s %s of %s Spaces\n' "with a ReleaseTarget" "$spaces_with_release_target" "$space_count"
printf '    %-32s %s of %s Targets\n' "with Cluster.* facts" "$targets_with_facts" "$target_count"
echo

if [[ "$space_count" -eq 0 || "$unit_count" -eq 0 ]]; then
  echo "  This organization has nothing to chart. Seed one of:"
  echo "    ../promotion-demo-data   multi-app, multi-environment"
  echo "    ../initiatives-demo      policies, for compliance panels"
  echo "    ../global-app            multi-service"
  exit 0
fi

gaps=0
if [[ "$environment_spaces" -lt "$space_count" ]]; then
  echo "  Some Spaces have no Environment label — they will fall into '(none)':"
  echo "    cub space update --patch <space> --label Environment=prod"
  gaps=1
fi
if [[ "$component_spaces" -lt "$space_count" ]]; then
  echo "  Some Spaces have no Component label:"
  echo "    cub space update --patch <space> --label Component=checkout"
  gaps=1
fi
if [[ "$units_with_target" -eq 0 ]]; then
  echo "  No Unit is bound to a Target, so every per-cluster panel will be empty."
  echo "  This is expected for base/template Spaces and for config that is not"
  echo "  deployable on its own. See DESIGN.md."
  gaps=1
fi
if [[ "$target_count" -gt 0 && "$targets_with_facts" -eq 0 ]]; then
  echo "  No Target has collected cluster facts, so the Kubernetes-version panel"
  echo "  will be empty. Collect them with:  cub k8s collect <target-slug>"
  gaps=1
fi
[[ "$gaps" -eq 0 ]] && echo "  Full dimension coverage — every bundled panel has data to show."

exit 0
