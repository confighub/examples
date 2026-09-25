#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state

gpu_operator_tag="${1:-${DEFAULT_GPU_OPERATOR_TAG}}"
device_plugin_tag="${2:-${DEFAULT_DEVICE_PLUGIN_TAG}}"

echo "==> Updating gpu-operator base image tag to :${gpu_operator_tag}"
cub function do set-image-reference gpu-operator ":${gpu_operator_tag}" --space "$(base_space)" --unit "$(unit_name gpu-operator base)"

echo "==> Updating nvidia-device-plugin base image tag to :${device_plugin_tag}"
cub function do set-image-reference nvidia-device-plugin ":${device_plugin_tag}" --space "$(base_space)" --unit "$(unit_name nvidia-device-plugin base)"

echo "==> Propagating upgrades through the materialized chain"
for component in "${COMPONENTS[@]}"; do
  cub unit push-upgrade --space "$(base_space)" "$(unit_name "${component}" base)"
  cub unit push-upgrade --space "$(platform_space)" "$(unit_name "${component}" platform)"
  cub unit push-upgrade --space "$(accelerator_space)" "$(unit_name "${component}" accelerator)"
  cub unit push-upgrade --space "$(os_space)" "$(unit_name "${component}" os)"
  cub unit push-upgrade --space "$(recipe_space)" "$(unit_name "${component}" recipe)"
done

echo "==> Refreshing explicit recipe manifest"
refresh_recipe_manifest_unit "${DIRECT_TARGET_REF:-${TARGET_REF:-}}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"

echo "Upgrade propagation complete. Run ./verify.sh to inspect the chain."
