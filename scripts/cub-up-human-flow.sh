#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

resolve_cub_cmd() {
  if [[ -n "${CUB_CMD:-}" ]]; then
    echo "${CUB_CMD}"
    return
  fi
  if command -v cub >/dev/null 2>&1; then
    command -v cub
    return
  fi
  echo "cub not found. Install cub and retry." >&2
  exit 1
}

resolve_runner_mode() {
  if [[ -n "${CUB_UP_CMD:-}" ]]; then
    echo "cub-up:${CUB_UP_CMD}"
    return
  fi
  if command -v cub-up >/dev/null 2>&1; then
    echo "cub-up:$(command -v cub-up)"
    return
  fi
  if "${cub_cmd}" up app --help >/dev/null 2>&1; then
    echo "cub-up-subcommand:${cub_cmd}"
    return
  fi
  echo "Need either 'cub-up' or 'cub up' support. Install/update your CLI." >&2
  exit 1
}

ensure_stale_support() {
  case "${runner_mode}" in
    cub-up)
      if ! "${runner_bin}" app --help 2>&1 | grep -q -- "--stale-after"; then
        echo "This cub-up does not support --stale-after/--stale-action. Update CLI." >&2
        exit 1
      fi
      ;;
    cub-up-subcommand)
      if ! "${cub_cmd}" up app --help 2>&1 | grep -q -- "--stale-after"; then
        echo "This cub does not support --stale-after/--stale-action on 'cub up'. Update CLI." >&2
        exit 1
      fi
      ;;
  esac
}

run_cmd() {
  local kind="$1"
  local path="$2"
  local env_name="$3"
  local target_slug="$4"
  local on_exists="$5"
  local stale_after="$6"
  local stale_action="$7"

  local cmd=()
  if [[ "${runner_mode}" == "cub-up" ]]; then
    cmd=(
      "${runner_bin}" "${kind}" "${path}"
      --env "${env_name}"
      --assert
      --open-ui
      --preflight
      --on-exists "${on_exists}"
      --stale-after "${stale_after}"
      --stale-action "${stale_action}"
    )
  else
    cmd=(
      "${cub_cmd}" up "${kind}" "${path}"
      --env "${env_name}"
      --assert
      --open-ui
      --preflight
      --on-exists "${on_exists}"
      --stale-after "${stale_after}"
      --stale-action "${stale_action}"
    )
  fi
  if [[ -n "${target_slug}" ]]; then
    cmd+=(--target "${target_slug}")
  fi

  echo "Human flow:"
  echo "  Entry: ${runner_mode}"
  echo "  1. DO X: run cub-up contract"
  echo "  2. STATE X1: assertions from --assert"
  echo "  3. GUI: follow --open-ui checkpoints"
  echo "  4. STALE POLICY: --stale-after ${stale_after} / --stale-action ${stale_action}"
  echo
  printf 'COMMAND: %q ' "${cmd[@]}"
  echo
  "${cmd[@]}"
}

cub_cmd="$(resolve_cub_cmd)"
if ! "${cub_cmd}" auth get-token >/dev/null 2>&1; then
  echo "Not authenticated. Run: ${cub_cmd} auth login" >&2
  exit 1
fi

runner_info="$(resolve_runner_mode)"
runner_mode="${runner_info%%:*}"
runner_bin="${runner_info#*:}"
ensure_stale_support

kind="${1:-app}"
path="${2:-${repo_root}/incubator/cub-up/global-app}"
env_name="${3:-dev}"
target_slug="${4:-}"

if [[ "${kind}" != "app" && "${kind}" != "platform" ]]; then
  echo "First argument must be 'app' or 'platform'." >&2
  exit 1
fi

if [[ -n "${CUB_UP_ON_EXISTS:-}" ]]; then
  on_exists="${CUB_UP_ON_EXISTS}"
elif [[ -t 0 ]]; then
  on_exists="prompt"
else
  on_exists="fresh"
fi

stale_after="${CUB_UP_STALE_AFTER:-24h}"
if [[ -n "${CUB_UP_STALE_ACTION:-}" ]]; then
  stale_action="${CUB_UP_STALE_ACTION}"
elif [[ -t 0 ]]; then
  stale_action="prompt"
else
  stale_action="fresh"
fi

run_cmd "${kind}" "${path}" "${env_name}" "${target_slug}" "${on_exists}" "${stale_after}" "${stale_action}"
