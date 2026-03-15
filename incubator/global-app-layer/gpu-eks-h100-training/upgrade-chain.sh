#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state

operator_tag="${1:-${DEFAULT_OPERATOR_TAG}}"

echo "==> Updating base image tag to :${operator_tag}"
cub function do set-image-reference gpu-operator ":${operator_tag}" --space "$(base_space)" --unit "$(unit_name base)"

echo "==> Propagating upgrades through the materialized chain"
cub unit push-upgrade --space "$(base_space)" "$(unit_name base)"
cub unit push-upgrade --space "$(platform_space)" "$(unit_name platform)"
cub unit push-upgrade --space "$(accelerator_space)" "$(unit_name accelerator)"
cub unit push-upgrade --space "$(os_space)" "$(unit_name os)"
cub unit push-upgrade --space "$(recipe_space)" "$(unit_name recipe)"

echo "==> Refreshing explicit recipe manifest"
refresh_recipe_manifest_unit "${TARGET_REF:-}"

echo "Upgrade propagation complete. Run ./verify.sh to inspect the chain."
