#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

flow_mode="${CUB_UP_FLOW_MODE:-ai}"
if [[ "${flow_mode}" == "pair" ]]; then
  export CUB_UP_ON_EXISTS="${CUB_UP_ON_EXISTS:-prompt}"
  export CUB_UP_STALE_ACTION="${CUB_UP_STALE_ACTION:-prompt}"
else
  export CUB_UP_ON_EXISTS="${CUB_UP_ON_EXISTS:-fresh}"
  export CUB_UP_STALE_ACTION="${CUB_UP_STALE_ACTION:-fresh}"
fi
export CUB_UP_STALE_AFTER="${CUB_UP_STALE_AFTER:-24h}"

echo "AI-led flow (${flow_mode} mode):"
echo "  1. DO X: run cub-up contract"
echo "  2. STATE X1: assertions from --assert"
echo "  3. STATE XY: GUI checkpoints from --open-ui"
echo "  4. STALE POLICY: --stale-after ${CUB_UP_STALE_AFTER} / --stale-action ${CUB_UP_STALE_ACTION}"
if [[ "${flow_mode}" == "pair" ]]; then
  echo "  5. PAIR: human follows GUI checkpoints while AI narrates transitions"
fi
echo

"${script_dir}/cub-up-human-flow.sh" "$@"
