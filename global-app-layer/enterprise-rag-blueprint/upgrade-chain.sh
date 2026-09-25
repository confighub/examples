#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state

new_llm_model="${1:-${DEFAULT_LLM_MODEL_NAME}}"
new_embed_model="${2:-${DEFAULT_EMBED_MODEL_NAME}}"

usage() {
  cat <<EOF_USAGE
Usage:
  ./upgrade-chain.sh [<llm-model>] [<embed-model>]

Examples:
  ./upgrade-chain.sh llama3.2:3b nomic-embed-text         # Ollama-style refs
  ./upgrade-chain.sh llama-3.1-70b-instruct nv-embedqa-e5-v5
EOF_USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

echo "==> Bumping nim-llm MODEL_NAME to ${new_llm_model} at the profile layer"
cub function do set-env nim-llm "MODEL_NAME=${new_llm_model}" --space "$(profile_space)" --unit "$(unit_name nim-llm profile)"

echo "==> Bumping nim-embedding EMBED_MODEL_NAME to ${new_embed_model} at the profile layer"
cub function do set-env nim-embedding "EMBED_MODEL_NAME=${new_embed_model}" --space "$(profile_space)" --unit "$(unit_name nim-embedding profile)"

echo "==> Bumping rag-server defaults at the profile layer to match"
cub function do set-env rag-server "MODEL_NAME=${new_llm_model}" --space "$(profile_space)" --unit "$(unit_name rag-server profile)"
cub function do set-env rag-server "EMBED_MODEL_NAME=${new_embed_model}" --space "$(profile_space)" --unit "$(unit_name rag-server profile)"

echo "==> Propagating upgrades through the materialized chain"
for component in "${COMPONENTS[@]}"; do
  cub unit push-upgrade --space "$(profile_space)" "$(unit_name "${component}" profile)"
  cub unit push-upgrade --space "$(recipe_space)" "$(unit_name "${component}" recipe)"
done

echo "==> Refreshing explicit recipe manifest"
refresh_recipe_manifest_unit "${DIRECT_TARGET_REF:-${TARGET_REF:-}}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"

echo "Upgrade propagation complete. Run ./verify.sh to inspect the chain."
echo "Note: deployment-layer overrides (e.g. tenant-local LLM_HOST when STACK=ollama) are preserved."
