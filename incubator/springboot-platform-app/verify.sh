#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUMMARY_JSON="$ROOT_DIR/example-summary.json"
ROUTES_YAML="$ROOT_DIR/operational/field-routes.yaml"

required_files=(
  "$ROOT_DIR/README.md"
  "$ROOT_DIR/AI_START_HERE.md"
  "$ROOT_DIR/prompts.md"
  "$ROOT_DIR/contracts.md"
  "$SUMMARY_JSON"
  "$ROUTES_YAML"
  "$ROOT_DIR/upstream/app/pom.xml"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/InventoryApiApplication.java"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/api/InventoryController.java"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/api/InventoryFeatureProperties.java"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/api/InventoryRuntimeProperties.java"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/api/InventoryItem.java"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/api/InventorySummary.java"
  "$ROOT_DIR/upstream/app/src/main/java/com/example/inventory/api/InventoryService.java"
  "$ROOT_DIR/upstream/app/src/main/resources/application.yaml"
  "$ROOT_DIR/upstream/app/src/main/resources/application-stage.yaml"
  "$ROOT_DIR/upstream/app/src/main/resources/application-prod.yaml"
  "$ROOT_DIR/upstream/app/src/test/java/com/example/inventory/api/InventoryControllerHttpTest.java"
  "$ROOT_DIR/upstream/app/src/test/java/com/example/inventory/api/InventoryControllerProdHttpTest.java"
  "$ROOT_DIR/upstream/platform/runtime-policy.yaml"
  "$ROOT_DIR/upstream/platform/slo-policy.yaml"
  "$ROOT_DIR/operational/configmap.yaml"
  "$ROOT_DIR/operational/deployment.yaml"
  "$ROOT_DIR/operational/service.yaml"
  "$ROOT_DIR/changes/01-mutable-in-ch.md"
  "$ROOT_DIR/changes/02-lift-upstream.md"
  "$ROOT_DIR/changes/03-generator-owned.md"
  "$ROOT_DIR/confighub-setup.sh"
  "$ROOT_DIR/confighub-cleanup.sh"
  "$ROOT_DIR/confighub-verify.sh"
  "$ROOT_DIR/confighub/inventory-api-dev.yaml"
  "$ROOT_DIR/confighub/inventory-api-stage.yaml"
  "$ROOT_DIR/confighub/inventory-api-prod.yaml"
)

for file in "${required_files[@]}"; do
  [[ -f "$file" ]] || {
    echo "missing required file: $file" >&2
    exit 1
  }
done

jq -e '.example_name == "springboot-platform-app"' "$SUMMARY_JSON" >/dev/null
jq -e '.proof_type | startswith("structural")' "$SUMMARY_JSON" >/dev/null
jq -e '.mutates_confighub == false and .mutates_live_infra == false' "$SUMMARY_JSON" >/dev/null
jq -e '.behaviors | length == 3' "$SUMMARY_JSON" >/dev/null
jq -e '.behaviors[].name' "$SUMMARY_JSON" >/dev/null

grep -q 'defaultAction: mutable-in-ch' "$ROUTES_YAML"
grep -q 'defaultAction: lift-upstream' "$ROUTES_YAML"
grep -q 'defaultAction: generator-owned' "$ROUTES_YAML"

echo "ok: springboot-platform-app fixtures are consistent"
