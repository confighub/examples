#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state

backend_tag="${1:-${DEFAULT_BACKEND_TAG}}"
frontend_tag="${2:-${DEFAULT_FRONTEND_TAG}}"
postgres_tag="${3:-${DEFAULT_POSTGRES_TAG}}"

echo "==> Updating backend base image tag to :${backend_tag}"
cub function do set-image-reference backend ":${backend_tag}" --space "$(base_space)" --unit "$(unit_name backend base)"

echo "==> Updating frontend base image tag to :${frontend_tag}"
cub function do set-image-reference frontend ":${frontend_tag}" --space "$(base_space)" --unit "$(unit_name frontend base)"

echo "==> Updating postgres base image tag to :${postgres_tag}"
cub function do set-image-reference postgres ":${postgres_tag}" --space "$(base_space)" --unit "$(unit_name postgres base)"

echo "==> Propagating upgrades through the materialized chain"
for component in "${COMPONENTS[@]}"; do
  cub unit push-upgrade --space "$(base_space)" "$(unit_name "${component}" base)"
  cub unit push-upgrade --space "$(region_space)" "$(unit_name "${component}" region)"
  cub unit push-upgrade --space "$(role_space)" "$(unit_name "${component}" role)"
  cub unit push-upgrade --space "$(recipe_space)" "$(unit_name "${component}" recipe)"
done

echo "==> Refreshing explicit recipe manifest"
refresh_recipe_manifest_unit "${TARGET_REF:-}"

echo "Upgrade propagation complete. Run ./verify.sh to inspect the chain."
