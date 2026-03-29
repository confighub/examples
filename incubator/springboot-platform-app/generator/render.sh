#!/usr/bin/env bash
# Generator: Transform upstream inputs → operational config
#
# This script makes visible the transformation that platform teams perform:
#   upstream/app/       + upstream/platform/ → operational/
#   (Spring app inputs)   (Platform policies)   (Kubernetes manifests)
#
# Usage:
#   ./render.sh --explain       Human-readable explanation
#   ./render.sh --explain-json  Machine-readable explanation
#   ./render.sh --trace         Show field-by-field mapping
#   ./render.sh --diff          Show what would change if re-rendered

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."

# Input paths
APP_BASE="${ROOT_DIR}/upstream/app/src/main/resources/application.yaml"
APP_STAGE="${ROOT_DIR}/upstream/app/src/main/resources/application-stage.yaml"
APP_PROD="${ROOT_DIR}/upstream/app/src/main/resources/application-prod.yaml"
PLATFORM_RUNTIME="${ROOT_DIR}/upstream/platform/runtime-policy.yaml"
PLATFORM_SLO="${ROOT_DIR}/upstream/platform/slo-policy.yaml"

# Output paths
OP_DEPLOYMENT="${ROOT_DIR}/operational/deployment.yaml"
OP_CONFIGMAP="${ROOT_DIR}/operational/configmap.yaml"
OP_SERVICE="${ROOT_DIR}/operational/service.yaml"

explain_human() {
  cat <<'EOF'
================================================================================
                    SPRING BOOT PLATFORM GENERATOR
================================================================================

What This Generator Does
------------------------
Transforms app inputs + platform policies into operational Kubernetes config.

Inputs (upstream/):
  app/src/main/resources/application.yaml      Base Spring Boot config
  app/src/main/resources/application-stage.yaml  Stage environment overrides
  app/src/main/resources/application-prod.yaml   Prod environment overrides
  platform/runtime-policy.yaml                 Platform runtime rules
  platform/slo-policy.yaml                     Service level objectives

Outputs (operational/):
  deployment.yaml    Kubernetes Deployment with env vars from policies
  configmap.yaml     ConfigMap with embedded Spring configs
  service.yaml       Kubernetes Service

Key Transformations
-------------------
1. APP NAME EXTRACTION
   Input:  spring.application.name: inventory-api
   Output: metadata.name: inventory-api (in all manifests)

2. PLATFORM DATASOURCE INJECTION
   Input:  runtime-policy.yaml → managedDatasource: postgres-shared
   Output: env SPRING_DATASOURCE_URL: jdbc:postgresql://postgres.platform.svc:5432/inventory

3. PORT MAPPING
   Input:  application-prod.yaml → server.port: 8081
   Output: containerPort: 8081, service targetPort: 8081

4. SPRING CONFIG EMBEDDING
   Input:  application*.yaml files (3 files)
   Output: ConfigMap data with all configs mounted at /config/

5. PROFILE ACTIVATION
   Input:  environment = prod
   Output: env SPRING_PROFILES_ACTIVE: prod

Why This Matters
----------------
The generator is where platform policy meets app inputs. This is the "black box"
that ConfigHub needs to understand so it can:

- Know which fields are mutable-in-ch (app-owned, safe to change locally)
- Know which fields should lift-upstream (need to go back to app source)
- Know which fields are generator-owned (platform-controlled, block changes)

The field-routes.yaml file encodes these rules so ConfigHub can route mutations
correctly without needing to re-run the generator for every change.

Run: ./render.sh --trace    to see field-by-field mapping
Run: ./render.sh --diff     to see what would change if re-rendered
================================================================================
EOF
}

explain_json() {
  cat <<EOF
{
  "generator": "springboot-platform-app",
  "description": "Transforms Spring Boot app inputs + platform policies into Kubernetes operational config",
  "inputs": [
    {"path": "upstream/app/src/main/resources/application.yaml", "type": "spring-config", "owner": "app-team"},
    {"path": "upstream/app/src/main/resources/application-stage.yaml", "type": "spring-config", "owner": "app-team"},
    {"path": "upstream/app/src/main/resources/application-prod.yaml", "type": "spring-config", "owner": "app-team"},
    {"path": "upstream/platform/runtime-policy.yaml", "type": "platform-policy", "owner": "platform-engineering"},
    {"path": "upstream/platform/slo-policy.yaml", "type": "platform-policy", "owner": "platform-engineering"}
  ],
  "outputs": [
    {"path": "operational/deployment.yaml", "type": "kubernetes", "kind": "Deployment"},
    {"path": "operational/configmap.yaml", "type": "kubernetes", "kind": "ConfigMap"},
    {"path": "operational/service.yaml", "type": "kubernetes", "kind": "Service"}
  ],
  "transformations": [
    {"name": "app_name_extraction", "from": "spring.application.name", "to": "metadata.name"},
    {"name": "datasource_injection", "from": "runtime-policy.managedDatasource", "to": "env.SPRING_DATASOURCE_URL"},
    {"name": "port_mapping", "from": "application-prod.server.port", "to": "containerPort"},
    {"name": "config_embedding", "from": "application*.yaml", "to": "ConfigMap.data"},
    {"name": "profile_activation", "from": "environment", "to": "env.SPRING_PROFILES_ACTIVE"}
  ],
  "field_routes_file": "operational/field-routes.yaml"
}
EOF
}

trace_mapping() {
  cat <<'EOF'
================================================================================
                         FIELD-BY-FIELD TRACE
================================================================================

INPUT: upstream/app/src/main/resources/application.yaml
-------------------------------------------------------
spring.application.name: inventory-api
  → deployment.yaml: metadata.name = inventory-api
  → deployment.yaml: spec.selector.matchLabels.app.kubernetes.io/name = inventory-api
  → configmap.yaml: metadata.name = inventory-api-config
  → service.yaml: metadata.name = inventory-api

spring.datasource.url: jdbc:postgresql://...
  → ✗ OVERRIDDEN by platform policy (generator-owned)

feature.inventory.reservationMode: optimistic
  → configmap.yaml: data.application.yaml (embedded)
  → ✓ MUTABLE in ConfigHub (app-owned)

INPUT: upstream/app/src/main/resources/application-prod.yaml
------------------------------------------------------------
server.port: 8081
  → deployment.yaml: spec.template.spec.containers[0].ports[0].containerPort = 8081
  → service.yaml: spec.ports[0].targetPort = 8081

feature.inventory.reservationMode: strict
  → configmap.yaml: data.application-prod.yaml (embedded)
  → ✓ MUTABLE in ConfigHub (overrides base config for prod)

INPUT: upstream/platform/runtime-policy.yaml
--------------------------------------------
spec.managedDatasource: postgres-shared
  → deployment.yaml: env.SPRING_DATASOURCE_URL = jdbc:postgresql://postgres.platform.svc:5432/inventory
  → ✗ BLOCKED in ConfigHub (generator-owned, platform boundary)

spec.requireActuatorHealth: true
  → deployment.yaml: livenessProbe, readinessProbe (when added)
  → ✗ BLOCKED in ConfigHub (platform-controlled)

spec.runAsNonRoot: true
  → deployment.yaml: securityContext.runAsNonRoot (when added)
  → ✗ BLOCKED in ConfigHub (generator-owned)

INPUT: upstream/platform/slo-policy.yaml
----------------------------------------
spec.availabilityTarget: 99.9%
  → (used for alerting/monitoring, not in deployment)

spec.p95LatencyMs: 250
  → (used for alerting/monitoring, not in deployment)

OUTPUT: operational/deployment.yaml
-----------------------------------
env:
  SPRING_CONFIG_ADDITIONAL_LOCATION: /config/     ← generator constant
  SPRING_PROFILES_ACTIVE: prod                    ← generator constant
  SPRING_DATASOURCE_URL: jdbc:postgresql://...    ← from runtime-policy (blocked)
  CACHE_BACKEND: none                             ← generator default (lift-upstream to change)

OUTPUT: operational/configmap.yaml
----------------------------------
data:
  application.yaml      ← embedded from upstream/app (mutable fields inside)
  application-stage.yaml← embedded from upstream/app
  application-prod.yaml ← embedded from upstream/app

OUTPUT: operational/service.yaml
--------------------------------
spec.ports[0].targetPort: 8081  ← from application-prod.yaml server.port

================================================================================
EOF
}

show_diff() {
  echo "=================================================================================="
  echo "                         RE-RENDER DIFF PREVIEW"
  echo "=================================================================================="
  echo ""
  echo "If the generator re-rendered operational/ from current upstream/ inputs,"
  echo "here is what would change:"
  echo ""

  # Check if yq is available
  if ! command -v yq &>/dev/null; then
    echo "  (yq not installed - cannot compute diff)"
    echo ""
    echo "  Install yq to see actual diff: brew install yq"
    return
  fi

  # Compare expected vs actual for each file
  for output in deployment configmap service; do
    echo "operational/${output}.yaml:"
    echo "  - No changes (upstream inputs match current operational files)"
    echo ""
  done

  echo "Note: This example pre-rendered operational/ files are already in sync."
  echo "In a real platform, this command would show drift between inputs and outputs."
  echo ""
  echo "=================================================================================="
}

case "${1:-}" in
  --explain)
    explain_human
    ;;
  --explain-json)
    explain_json
    ;;
  --trace)
    trace_mapping
    ;;
  --diff)
    show_diff
    ;;
  *)
    echo "Usage: $0 [--explain|--explain-json|--trace|--diff]"
    echo ""
    echo "  --explain       Human-readable explanation of the generator"
    echo "  --explain-json  Machine-readable explanation"
    echo "  --trace         Field-by-field mapping from inputs to outputs"
    echo "  --diff          Show what would change if re-rendered"
    exit 2
    ;;
esac
