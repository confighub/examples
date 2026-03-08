#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export CUB_UP_FLOW_MODE="pair"
export CUB_UP_ON_EXISTS="${CUB_UP_ON_EXISTS:-prompt}"
export CUB_UP_STALE_ACTION="${CUB_UP_STALE_ACTION:-prompt}"
export CUB_UP_STALE_AFTER="${CUB_UP_STALE_AFTER:-24h}"

"${script_dir}/cub-up-ai-flow.sh" "$@"
