#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

required_files=(
  "$ROOT_DIR/lift-upstream.sh"
  "$ROOT_DIR/lift-upstream/redis-cache/upstream-app/pom.xml"
  "$ROOT_DIR/lift-upstream/redis-cache/upstream-app/src/main/resources/application.yaml"
  "$ROOT_DIR/lift-upstream/redis-cache/confighub/order-api-dev.yaml"
  "$ROOT_DIR/lift-upstream/redis-cache/confighub/order-api-stage.yaml"
  "$ROOT_DIR/lift-upstream/redis-cache/confighub/order-api-prod.yaml"
)

for file in "${required_files[@]}"; do
  [[ -f "$file" ]] || {
    echo "missing required file: $file" >&2
    exit 1
  }
done

jq -e '.proof_type == "lift-upstream-bundle"' < <("$ROOT_DIR/lift-upstream.sh" --explain-json) >/dev/null

grep -q 'spring-boot-starter-data-redis' "$ROOT_DIR/lift-upstream/redis-cache/upstream-app/pom.xml"
grep -q 'cache:' "$ROOT_DIR/lift-upstream/redis-cache/upstream-app/src/main/resources/application.yaml"
grep -q 'type: redis' "$ROOT_DIR/lift-upstream/redis-cache/upstream-app/src/main/resources/application.yaml"

for file in \
  "$ROOT_DIR/lift-upstream/redis-cache/confighub/order-api-dev.yaml" \
  "$ROOT_DIR/lift-upstream/redis-cache/confighub/order-api-stage.yaml" \
  "$ROOT_DIR/lift-upstream/redis-cache/confighub/order-api-prod.yaml"; do
  grep -q 'type: redis' "$file"
  grep -q 'value: redis' "$file"
done

diff_output="$("$ROOT_DIR/lift-upstream.sh" --render-diff)"
grep -q 'spring-boot-starter-data-redis' <<<"$diff_output"
grep -q 'CACHE_BACKEND' <<<"$diff_output"
grep -q 'type: redis' <<<"$diff_output"

echo "ok: springboot-platform-app lift-upstream bundle is consistent"
