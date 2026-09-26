#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_file="${repo_root}/global-app-layer/baseconfig/frontend.yaml"
if [[ $# -gt 1 ]]; then
  echo "Usage: catalog/first-app-local-change.sh [fresh-output-directory]" >&2
  exit 1
fi
if [[ $# -eq 1 ]]; then
  output_dir="$1"
  if [[ -e "$output_dir" ]]; then
    echo "Output directory already exists: $output_dir" >&2
    exit 1
  fi
  mkdir -p "$output_dir"
else
  output_dir="$(mktemp -d "${TMPDIR:-/tmp}/confighub-first-app.XXXXXX")"
fi

if [[ "$(rg -c '^  replicas: 1$' "$source_file")" != "1" ]]; then
  echo "Expected one frontend replica field; source changed, inspect it first." >&2
  exit 1
fi
command -v cub >/dev/null 2>&1 || { echo "cub is required for local config diff" >&2; exit 1; }
command -v sed >/dev/null 2>&1 || { echo "sed is required" >&2; exit 1; }
command -v shasum >/dev/null 2>&1 || { echo "shasum is required" >&2; exit 1; }

cp "$source_file" "$output_dir/before.yaml"
sed 's/^  replicas: 1$/  replicas: 2/' "$output_dir/before.yaml" > "$output_dir/after.yaml"
before_hash="$(shasum -a 256 "$output_dir/before.yaml" | cut -d ' ' -f 1)"
after_hash="$(shasum -a 256 "$output_dir/after.yaml" | cut -d ' ' -f 1)"
if [[ "$before_hash" == "$after_hash" ]]; then
  echo "No change was produced" >&2
  exit 1
fi

cub config diff "$output_dir/before.yaml" "$output_dir/after.yaml" --json --out "$output_dir/result.json"
echo "Local copies and diff: $output_dir"
echo "Before SHA-256: $before_hash"
echo "After SHA-256:  $after_hash"
echo "No ConfigHub or cluster write was made."
