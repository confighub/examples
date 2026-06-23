#!/usr/bin/env bash
#
# ctrlplane-on-confighub: map a Ctrlplane "System" bundle onto a ConfigHub
# governed-app plan.
#
# READ-ONLY by default. The mapper never touches ConfigHub, a cluster, or live
# infra. The only mutating path is `./setup.sh --apply`, which is explicitly
# gated and prints what it will create first.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source_arg="${here}/systems"

usage() {
  cat <<'EOF'
Usage: ./setup.sh [--explain | --explain-json | --cub-commands | --apply] [--source PATH]

  --explain        human-readable mapping plan (default, read-only)
  --explain-json   machine-readable View Packet as JSON (read-only)
  --cub-commands   the cub commands the plan implies, printed only (read-only)
  --apply          create the ConfigHub objects (MUTATES ConfigHub; gated)
  --source PATH    Ctrlplane System file or directory (default: ./systems)

Read-only first:
  ./setup.sh --explain
  ./setup.sh --explain-json | jq
EOF
}

mode="explain"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) mode="explain" ;;
    --explain-json) mode="json" ;;
    --cub-commands) mode="cub-commands" ;;
    --apply) mode="apply" ;;
    --source) shift; source_arg="${1:?--source needs a path}" ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
  shift
done

case "${mode}" in
  explain)      exec python3 "${here}/map.py" --source "${source_arg}" --mode explain ;;
  json)         exec python3 "${here}/map.py" --source "${source_arg}" --mode json ;;
  cub-commands) exec python3 "${here}/map.py" --source "${source_arg}" --mode cub-commands ;;
  apply)
    echo "==> --apply will CREATE ConfigHub spaces and units. Plan first:"
    echo
    python3 "${here}/map.py" --source "${source_arg}" --mode explain
    echo
    echo "Auth check (read-only):"
    if ! cub space list >/dev/null 2>&1; then
      echo "ConfigHub auth is not active. Run 'cub auth login' and retry." >&2
      exit 1
    fi
    echo
    echo "NOTE: the --apply path is the next proof step for this example and is"
    echo "not yet verified end-to-end against a live ConfigHub space. Review the"
    echo "generated commands before running them:"
    echo "  ./setup.sh --cub-commands"
    echo
    echo "Refusing to auto-create objects in this POC. Generate, review, run by hand."
    exit 0
    ;;
esac
