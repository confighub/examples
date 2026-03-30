# AI Start Here: Spring Boot Platform (Platform-Centric)

## CRITICAL: Demo pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Print the FULL output on screen — do not summarize or abbreviate
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

This is a demo, not a script execution. The value is in understanding each step.

## Suggested prompt

```
Read spring-platform/springboot-platform-platform-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Don't move on until I say continue.
```

## What this example teaches

This is the **third** in a sequence of three related examples:

| # | Example | Focus |
|---|---------|-------|
| 1 | `springboot-platform-app` | Generator story (cub-gen) |
| 2 | `springboot-platform-app-centric` | One app: App → Deployments → Targets |
| 3 | **This example** | One platform, multiple apps: Platform → Apps → Deployments → Targets |

You'll understand:
- How a Platform organizes multiple apps with shared policies
- What the platform provides (managed services) vs controls (blocked fields)
- How apps inherit platform policies
- The difference between app-owned and platform-owned fields

## Stages

### Stage 1: "What is this platform?" (read-only)

Run: `cat platform-map.json | jq`

Print the full JSON. Explain:
- The platform is `springboot-platform` with managed datasource, hardening, observability
- Two apps run on it: `inventory-api` (3 deployments) and `catalog-api` (2 deployments)
- Platform controls `spring.datasource.*` and `securityContext.*` (blocked for all apps)
- Each app has its own mutable fields (feature flags)

**PAUSE.** Wait for the human.

---

### Stage 2: "What does the platform provide?" (read-only)

Run: `./platform.sh --summary`

Print the full output. Explain:
- managed-datasource: PostgreSQL with HA, encryption, backups
- runtime-hardening: Security defaults (runAsNonRoot, mTLS)
- observability: Health endpoints, SLO targets
- Both apps inherit these capabilities automatically

**PAUSE.** Wait for the human.

---

### Stage 3: "What apps run on this platform?" (read-only)

Run: `./platform.sh --apps`

Print the full output. Explain:
- inventory-api has 3 deployments (dev, stage, prod)
- catalog-api has 2 deployments (dev, prod)
- Each app has its own app-owned fields (feature flags)
- Both share platform-controlled fields (datasource, security)

**PAUSE.** Wait for the human.

---

### Stage 4: "What will setup create?" (read-only)

Run: `./setup.sh --explain`

Print the full output. Explain:
- 1 infra space (server worker)
- 5 app spaces (3 for inventory-api, 2 for catalog-api)
- 5 units, 5 noop targets
- Default mode is noop: apply workflow works without a cluster

**PAUSE.** Wait for the human.

---

### Stage 5: "Create the platform" (mutates ConfigHub)

Ask: "This will create 6 spaces, 5 units, and 5 noop targets. OK?"

Run: `./setup.sh`

Print the full output. Then verify:

Run: `cub space list --where "Labels.Platform = 'springboot-platform'" --json | jq '.[].Space.Slug'`

Explain: Six spaces now exist, all tagged with the same platform.

GUI now: Open https://hub.confighub.com → click **Spaces** → filter by label `Platform=springboot-platform`

**PAUSE.** Wait for the human.

---

### Stage 6: "Why is this field blocked?" (field lineage)

This is about **provenance** - understanding where field values come from.

Run: `./platform.sh --explain-field spring.datasource.url`

Print the full output. Explain:
- This field is platform-owned, applies to ALL apps on this platform
- The platform provides managed-datasource with HA, encryption, backups
- The generator injects this from platform policy, not app inputs
- App teams cannot change it — they must contact #platform-support

Then show an app-owned field:

Run: `./platform.sh --explain-field feature.inventory.reservationMode`

Explain:
- This field is app-owned (inventory-api only)
- It comes from app inputs, not platform policy
- Safe to change directly in ConfigHub

The difference in **field lineage** determines the mutation route:
- Platform policy → blocked
- App inputs (runtime tuning) → mutable
- App inputs (code change needed) → lift-upstream

**PAUSE.** Wait for the human.

---

### Stage 7: "Try a mutation on each app" (mutates ConfigHub)

Ask: "This will change feature flags on both apps. OK?"

For inventory-api:
```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: enable optimistic reservation mode" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic
```

For catalog-api:
```bash
cub function do --space catalog-api-prod --unit catalog-api \
  --change-desc "demo: enable recommendations" \
  set-env catalog-api FEATURE_CATALOG_RECOMMENDATIONSENABLED=true
```

Explain: Both mutations succeed because they're app-owned fields. Platform-owned fields would be blocked.

Now show the **provenance** - the mutation history:

```bash
cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

Print the output. Explain:
- Every mutation is recorded with who, when, and why
- The change description you provided is in the audit trail
- This is the provenance for apply-here mutations

**PAUSE.** Wait for the human.

---

### Stage 7b: "Running app sees the change" (optional, requires Java)

Ask: "Do you want to see the running app confirm the mutation? This requires Java 21+ and Maven."

If yes, run:

```bash
cd ../springboot-platform-app/upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic \
  mvn spring-boot:run -q -Dspring-boot.run.profiles=prod \
  -Dspring-boot.run.arguments="--server.port=8081" &
APP_PID=$!
sleep 10  # Wait for app to start
curl -s http://localhost:8081/api/inventory/summary | jq
kill $APP_PID
cd ../../springboot-platform-platform-centric
```

What you should see:

```json
{
  "service": "inventory-api",
  "environment": "prod",
  "reservationMode": "optimistic",
  "cacheBackend": "none"
}
```

Explain: The inventory-api app reports `optimistic` - the mutation is visible. This proves mutations on this platform's apps can be verified locally.

**PAUSE.** Wait for the human.

---

### Stage 8: "Cleanup"

Run: `./cleanup.sh`

Print the output. Explain: All spaces with the platform label have been deleted.

---

## Key insight

The platform-centric view answers: "I manage multiple apps — how do I organize shared policies?"

- Platform provides capabilities that all apps inherit
- Platform controls fields that should never diverge per-app
- Apps can still mutate their own feature flags
- One place to see all apps on the platform: `./platform.sh --apps`

## The three examples compared

| Question | Example to Use |
|----------|----------------|
| "How does the generator work?" | `springboot-platform-app` |
| "Show me one app across environments" | `springboot-platform-app-centric` |
| "Show me one platform with multiple apps" | **This example** |
