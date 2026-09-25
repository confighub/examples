#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state

new_tag="${1:-${DEFAULT_IMAGE_TAG}}"

echo "==> Updating base unit image tag to :${new_tag}"
cub function do set-image-reference backend ":${new_tag}" --space "$(base_space)" --unit "${BASE_UNIT}"

echo "==> Propagating upgrades through the materialized chain"
cub unit push-upgrade --space "$(base_space)" "${BASE_UNIT}"
cub unit push-upgrade --space "$(region_space)" "${REGION_UNIT}"
cub unit push-upgrade --space "$(role_space)" "${ROLE_UNIT}"
cub unit push-upgrade --space "$(recipe_space)" "${RECIPE_UNIT}"

echo "==> Refreshing explicit recipe manifest"
refresh_recipe_manifest_unit "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"

echo "Upgrade propagation complete. Run ./verify.sh to inspect the chain."
