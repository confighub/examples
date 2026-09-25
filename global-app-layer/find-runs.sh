#!/usr/bin/env bash
set -euo pipefail

require_cub() {
  if ! command -v cub >/dev/null 2>&1; then
    echo "Missing required command: cub" >&2
    exit 1
  fi
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "Missing required command: jq" >&2
    exit 1
  fi
}

usage() {
  cat <<'EOF_USAGE'
Find active global-app-layer runs in live ConfigHub using the labels written by the examples.

Usage:
  ./find-runs.sh [example] [--json]

Examples:
  ./find-runs.sh
  ./find-runs.sh realistic-app
  ./find-runs.sh gpu-eks-h100-training --json
EOF_USAGE
}

normalize_example_name() {
  local input="$1"
  case "${input}" in
    single-component|global-app-layer-single) echo "global-app-layer-single" ;;
    frontend-postgres|global-app-layer-frontend-postgres) echo "global-app-layer-frontend-postgres" ;;
    realistic-app|global-app-layer-realistic-app) echo "global-app-layer-realistic-app" ;;
    gpu-eks-h100-training|global-app-layer-gpu-eks-h100-training) echo "global-app-layer-gpu-eks-h100-training" ;;
    *)
      echo "Unknown example: ${input}" >&2
      usage >&2
      exit 1
      ;;
  esac
}

example_filter=""
json_output=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      json_output=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      if [[ -n "${example_filter}" ]]; then
        echo "Unexpected extra argument: $1" >&2
        usage >&2
        exit 1
      fi
      example_filter="$(normalize_example_name "$1")"
      shift
      ;;
  esac
done

require_cub
require_jq

where_filter="Labels.ExampleName LIKE 'global-app-layer-%'"
if [[ -n "${example_filter}" ]]; then
  where_filter="Labels.ExampleName = '${example_filter}'"
fi

spaces_json="$(cub space list --where "${where_filter}" --json)"

grouped_json="$(printf '%s\n' "${spaces_json}" | jq '
  map(select(.Space != null and .Space.Labels != null and (.Space.Labels.ExampleName // "") != "")) |
  sort_by(.Space.Labels.ExampleChain, .Space.Slug) |
  group_by(.Space.Labels.ExampleChain) |
  map({
    exampleName: (.[0].Space.Labels.ExampleName // ""),
    exampleChain: (.[0].Space.Labels.ExampleChain // ""),
    spaces: [.[].Space.Slug],
    spaceCount: length,
    deploySpaces: [ .[] | select((.Space.Labels.LayerKind // "") == "deployment") | .Space.Slug ],
    deployNamespaces: ([ .[] | select((.Space.Labels.LayerKind // "") == "deployment") | (.Space.Labels.Cluster // empty) ] | unique),
    recipeSpaces: [ .[] | select((.Space.Labels.LayerKind // "") == "recipe") | .Space.Slug ]
  })
')"

if [[ "${json_output}" -eq 1 ]]; then
  printf '%s\n' "${grouped_json}"
  exit 0
fi

count="$(printf '%s\n' "${grouped_json}" | jq 'length')"
if [[ "${count}" -eq 0 ]]; then
  cat <<EOF_EMPTY
No active global-app-layer runs found with:
  cub space list --where "${where_filter}" --json

Useful next checks:
- cub context list --json
- cub space list --where "Labels.ExampleName LIKE 'global-app-layer-%'" --json
- if you expect a local run, check the repo root with: git rev-parse --show-toplevel
EOF_EMPTY
  exit 0
fi

printf '%s\n' "${grouped_json}" | jq -r '
  .[] |
  "Example: " + .exampleName + "\n" +
  "Chain:   " + .exampleChain + "\n" +
  "Spaces:  " + (.spaceCount | tostring) + "\n" +
  "Deploy:  " + (if (.deploySpaces | length) == 0 then "<none>" else (.deploySpaces | join(", ")) end) + "\n" +
  "NS:      " + (if (.deployNamespaces | length) == 0 then "<none>" else (.deployNamespaces | join(", ")) end) + "\n" +
  "Recipe:  " + (if (.recipeSpaces | length) == 0 then "<none>" else (.recipeSpaces | join(", ")) end) + "\n" +
  "All:     " + (.spaces | join(", ")) + "\n"
'
