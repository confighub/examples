#!/usr/bin/env bash
# build.sh — create the cvedb SQLite database and load CVE data into it (idempotent).
#
#   SEC_SCANNER_OFFLINE=1  load curated fixtures instead of downloading OSV data
#   SEC_SCANNER_DB         override the database path (default: cvedb/cve.db)
#
# No database server, no Python: the importer is the `secscan` Go binary, which
# talks to the SQLite file through a pure-Go driver. Re-running imports only if
# the database is still empty.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
export SEC_SCANNER_DB="${SEC_SCANNER_DB:-${SCRIPT_DIR}/cve.db}"
OSV_ECOSYSTEMS=(Alpine:v3.9 Alpine:v3.10 Alpine:v3.12)

command -v go >/dev/null || { echo "ERROR: go not found (needed to build the scanner/importer)" >&2; exit 1; }

SECSCAN="${EXAMPLE_DIR}/scanner/secscan"
if [[ ! -x "$SECSCAN" ]]; then
  echo "  building secscan..."
  ( cd "${EXAMPLE_DIR}/scanner" && go build -o secscan . )
fi

if [[ "${SEC_SCANNER_OFFLINE:-0}" == "1" ]]; then
  echo "  importing curated fixtures (offline)..."
  "$SECSCAN" import --if-empty --fixtures --fixtures-dir "${SCRIPT_DIR}/fixtures"
else
  echo "  importing OSV ecosystem exports: ${OSV_ECOSYSTEMS[*]}"
  args=(); for e in "${OSV_ECOSYSTEMS[@]}"; do args+=(--osv-zip "$e"); done
  "$SECSCAN" import --if-empty "${args[@]}" \
    || { echo "  OSV import failed; falling back to fixtures"; "$SECSCAN" import --if-empty --fixtures --fixtures-dir "${SCRIPT_DIR}/fixtures"; }
fi
