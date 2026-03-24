#!/usr/bin/env bash
# Scenario 2: Lift a change upstream to the source repo.
#
# Some config changes require structural modifications (new dependencies,
# new config sections, new secrets). These can't be done as direct ConfigHub
# mutations — they need to go back to the source repo so the generator
# produces the right shape going forward.
#
# This script demonstrates the "lift-upstream" routing: ConfigHub identifies
# the change, produces a deterministic diff bundle, and routes it to Git.
#
# Usage:
#   ./lift-upstream.sh                # Show the routing decision
#   ./lift-upstream.sh --render-diff  # Show the GitHub-ready patch
#   ./lift-upstream.sh --explain      # What this demonstrates
#   ./lift-upstream.sh --json         # Machine-readable output

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_explain() {
  cat <<'EOF'
lift-upstream — Scenario 2: Route a change back to code

The story:
  The app team wants Redis caching. Someone says "we need
  spring.cache.type=redis in prod." But this isn't a feature flag flip —
  it requires new Maven dependencies, new application.yaml entries, and
  possibly new connection secrets.

Why it can't be a direct mutation:
  - Needs spring-boot-starter-data-redis in pom.xml
  - Needs spring.data.redis.host and connection config
  - The generator must produce the right shape going forward
  - A direct ConfigHub mutation would be overwritten on next refresh

What ConfigHub gives you:
  1. The field-route model says: spring.cache.* is "lift-upstream"
  2. The tooling produces a deterministic diff bundle
  3. The diff is GitHub-ready — paste it into a PR

What this proves:
  - ConfigHub knows which fields need to go upstream
  - The activity log stays in ConfigHub even when the edit lands in Git
  - Different change types get different paths

What is NOT yet proven:
  - Automated PR creation (the diff exists but gh pr create is not called)
  - Round-trip: PR merged → generator re-runs → ConfigHub refreshes
EOF
}

show_routing() {
  cat <<'EOF'
=== Lift Upstream — Routing Decision ===

Requested change:
  spring.cache.type: none → redis

Field route lookup:
  FIELD                   CURRENT     ROUTE
  spring.cache.type       none        lift-upstream
  spring.cache.*          —           lift-upstream

Decision: LIFT UPSTREAM
  This field is classified as "lift-upstream" because changing it requires
  structural modifications to the application:
    - New Maven dependency: spring-boot-starter-data-redis
    - New config section: spring.data.redis.*
    - Possibly new secret: redis connection credentials

  Direct mutation in ConfigHub is not appropriate. The change must go back
  to the source repo so the generator produces the correct shape.

Next step:
  ./lift-upstream.sh --render-diff    # See the GitHub-ready patch
EOF
}

show_diff() {
  cat <<'EOF'
=== Lift Upstream — GitHub-Ready Patch ===

The following changes would enable Redis caching for inventory-api.
This is a deterministic diff bundle — same inputs always produce the same patch.

--------------------------------------------------------------------------------
FILE: upstream/app/pom.xml
--------------------------------------------------------------------------------
@@ -45,6 +45,10 @@
         <artifactId>spring-boot-starter-web</artifactId>
     </dependency>

+    <dependency>
+        <groupId>org.springframework.boot</groupId>
+        <artifactId>spring-boot-starter-data-redis</artifactId>
+    </dependency>
+
     <dependency>
         <groupId>org.springframework.boot</groupId>
         <artifactId>spring-boot-starter-test</artifactId>

--------------------------------------------------------------------------------
FILE: upstream/app/src/main/resources/application-prod.yaml
--------------------------------------------------------------------------------
@@ -8,3 +8,10 @@ spring:
   datasource:
     url: ${SPRING_DATASOURCE_URL:jdbc:postgresql://prod-db:5432/inventory}

+  cache:
+    type: redis
+
+  data:
+    redis:
+      host: ${SPRING_REDIS_HOST:redis-prod.internal}
+      port: ${SPRING_REDIS_PORT:6379}

--------------------------------------------------------------------------------
FILE: confighub/inventory-api-prod.yaml (regenerated)
--------------------------------------------------------------------------------
@@ -22,6 +22,8 @@ spec:
           env:
             - name: SPRING_PROFILES_ACTIVE
               value: "prod"
+            - name: SPRING_REDIS_HOST
+              value: "redis-prod.internal"
             - name: SPRING_DATASOURCE_URL
               value: "jdbc:postgresql://prod-db:5432/inventory"

--------------------------------------------------------------------------------

To apply this change:
  1. Copy this patch to a new branch in the source repo
  2. Open a PR with description: "Enable Redis caching for inventory-api"
  3. After merge, the generator will re-render and ConfigHub will refresh

ConfigHub activity log:
  [RECORDED] Intent: enable Redis caching for prod
  [ROUTED]   Path: lift-upstream → source repo PR
  [PENDING]  Awaiting: PR merge + generator refresh
EOF
}

show_json() {
  cat <<'ENDJSON'
{
  "scenario": "lift-upstream",
  "requested_change": {
    "field": "spring.cache.type",
    "from": "none",
    "to": "redis"
  },
  "routing": {
    "route": "lift-upstream",
    "reason": "structural_change_required",
    "blocked": false
  },
  "required_changes": [
    {
      "file": "upstream/app/pom.xml",
      "change": "add_dependency",
      "artifact": "spring-boot-starter-data-redis"
    },
    {
      "file": "upstream/app/src/main/resources/application-prod.yaml",
      "change": "add_config_section",
      "keys": ["spring.cache.type", "spring.data.redis.host", "spring.data.redis.port"]
    },
    {
      "file": "confighub/inventory-api-prod.yaml",
      "change": "regenerate",
      "new_env_vars": ["SPRING_REDIS_HOST"]
    }
  ],
  "automation_status": {
    "diff_bundle": "available",
    "pr_creation": "not_implemented",
    "round_trip": "not_implemented"
  }
}
ENDJSON
}

case "${1:-}" in
  --explain)     show_explain; exit 0 ;;
  --render-diff) show_diff; exit 0 ;;
  --json)        show_json; exit 0 ;;
  "")            show_routing; exit 0 ;;
  *)             echo "Usage: $0 [--explain|--render-diff|--json]" >&2; exit 2 ;;
esac
