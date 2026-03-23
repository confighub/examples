#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$ROOT_DIR/lift-upstream/redis-cache"

PAIRS=(
  "upstream/app/pom.xml|$ROOT_DIR/upstream/app/pom.xml|$BUNDLE_DIR/upstream-app/pom.xml"
  "upstream/app/src/main/resources/application.yaml|$ROOT_DIR/upstream/app/src/main/resources/application.yaml|$BUNDLE_DIR/upstream-app/src/main/resources/application.yaml"
  "confighub/inventory-api-dev.yaml|$ROOT_DIR/confighub/inventory-api-dev.yaml|$BUNDLE_DIR/confighub/inventory-api-dev.yaml"
  "confighub/inventory-api-stage.yaml|$ROOT_DIR/confighub/inventory-api-stage.yaml|$BUNDLE_DIR/confighub/inventory-api-stage.yaml"
  "confighub/inventory-api-prod.yaml|$ROOT_DIR/confighub/inventory-api-prod.yaml|$BUNDLE_DIR/confighub/inventory-api-prod.yaml"
)

show_explain() {
  cat <<'EOF'
lift-upstream: GitHub-ready Redis bundle for springboot-platform-app

Proof type: lift-upstream-bundle

This is a read-only proof for the "lift upstream" route.

What it proves:
- the Redis caching request can be expressed as deterministic upstream changes
- the refreshed ConfigHub YAMLs are known for dev, stage, and prod
- a PR-ready diff can be rendered without mutating ConfigHub or Git

What it does NOT yet prove:
- actual GitHub PR creation
- automatic refresh from ConfigHub into a source repo
- policy enforcement for the route

Files changed by the bundle:
- upstream/app/pom.xml
- upstream/app/src/main/resources/application.yaml
- confighub/inventory-api-dev.yaml
- confighub/inventory-api-stage.yaml
- confighub/inventory-api-prod.yaml

Read-only commands:
- ./lift-upstream.sh --explain-json | jq
- ./lift-upstream.sh --render-diff
- ./lift-upstream-verify.sh
EOF
}

show_explain_json() {
  cat <<'EOF'
{
  "example_name": "springboot-platform-app",
  "proof_type": "lift-upstream-bundle",
  "mutates_confighub": false,
  "mutates_live_infra": false,
  "request": "Add Redis-backed caching to the service.",
  "bundle_root": "lift-upstream/redis-cache",
  "render_diff": "./lift-upstream.sh --render-diff",
  "verify": "./lift-upstream-verify.sh",
  "target_files": [
    "upstream/app/pom.xml",
    "upstream/app/src/main/resources/application.yaml",
    "confighub/inventory-api-dev.yaml",
    "confighub/inventory-api-stage.yaml",
    "confighub/inventory-api-prod.yaml"
  ],
  "durable_upstream_changes": [
    "Add spring-boot-starter-data-redis dependency",
    "Set spring.cache.type=redis in the upstream app config"
  ],
  "refreshed_operational_changes": [
    "Set CACHE_BACKEND=redis in each environment deployment",
    "Set spring.cache.type=redis in each ConfigMap application.yaml"
  ]
}
EOF
}

render_diff() {
  local pair target current lifted status
  for pair in "${PAIRS[@]}"; do
    IFS='|' read -r target current lifted <<<"${pair}"
    set +e
    diff -u --label "a/${target}" --label "b/${target}" "${current}" "${lifted}"
    status=$?
    set -e
    if [[ "${status}" -gt 1 ]]; then
      echo "diff failed for ${target}" >&2
      exit "${status}"
    fi
  done
}

case "${1:-}" in
  --explain)
    show_explain
    ;;
  --explain-json)
    show_explain_json
    ;;
  --render-diff)
    render_diff
    ;;
  *)
    cat <<'EOF' >&2
Use one of:
  ./lift-upstream.sh --explain
  ./lift-upstream.sh --explain-json
  ./lift-upstream.sh --render-diff
EOF
    exit 2
    ;;
esac
