#!/usr/bin/env bash
# build.sh — create the cost database (SQLite) and load pricing into it (idempotent).
#
#   COST_ESTIMATOR_DB   override the database path (default: costdb/cost.db)
#   COST_SOURCE         a pricing JSON to import instead of the curated fixtures
#
# No database server, no Python: the importer is the `costest` Go binary, which
# talks to the SQLite file through a pure-Go driver. Re-running imports only if
# the database is still empty (--if-empty).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
export COST_ESTIMATOR_DB="${COST_ESTIMATOR_DB:-${SCRIPT_DIR}/cost.db}"

command -v go >/dev/null || { echo "ERROR: go not found (needed to build the estimator/importer)" >&2; exit 1; }

COSTEST="${EXAMPLE_DIR}/estimator/costest"
if [[ ! -x "$COSTEST" ]]; then
  echo "  building costest..."
  ( cd "${EXAMPLE_DIR}/estimator" && go build -o costest . )
fi

if [[ -n "${COST_SOURCE:-}" ]]; then
  echo "  importing pricing from ${COST_SOURCE}..."
  "$COSTEST" import --if-empty --source "${COST_SOURCE}"
else
  echo "  importing curated pricing fixtures..."
  "$COSTEST" import --if-empty --fixtures --fixtures-dir "${SCRIPT_DIR}/fixtures"
fi
