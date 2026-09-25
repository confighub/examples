#!/usr/bin/env bash
# Run all three e2e flows sequentially.
#
# Prereqs: gitops-import infrastructure must be running.
#   global-app-layer/e2e/bin/create-cluster
#   global-app-layer/e2e/bin/install-argocd
#   global-app-layer/e2e/bin/setup-apps
#   CUB_SPACE=<space> global-app-layer/e2e/bin/install-worker
#
# Usage:
#   ./e2e/run-all.sh                      # run all 3 flows
#   ./e2e/run-all.sh brownfield           # run just brownfield
#   ./e2e/run-all.sh greenfield bridge    # run greenfield and bridge

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

FLOWS="${*:-brownfield greenfield bridge}"
PASSED=0
FAILED=0

for flow in ${FLOWS}; do
  case "${flow}" in
    brownfield|1) script="${SCRIPT_DIR}/01-brownfield.sh" ;;
    greenfield|2) script="${SCRIPT_DIR}/02-greenfield.sh" ;;
    bridge|3)     script="${SCRIPT_DIR}/03-bridge.sh" ;;
    *)
      echo "Unknown flow: ${flow}. Use: brownfield, greenfield, bridge." >&2
      exit 1
      ;;
  esac

  echo ""
  echo "========================================"
  echo "  Running: ${flow}"
  echo "========================================"
  echo ""

  if bash "${script}"; then
    PASSED=$((PASSED + 1))
  else
    FAILED=$((FAILED + 1))
    echo "FAILED: ${flow}" >&2
  fi
done

echo ""
echo "========================================"
echo "  E2E Summary: ${PASSED} passed, ${FAILED} failed"
echo "========================================"

if [[ "${FAILED}" -gt 0 ]]; then
  exit 1
fi
