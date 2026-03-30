# AI Start Here: Spring Boot Platform App (App-Centric)

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
Read spring-platform/springboot-platform-app-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Don't move on until I say continue.
```

## What this example teaches

One app (`inventory-api`) with three deployments (dev, stage, prod) and three mutation outcomes. You'll understand:
- How ConfigHub organizes apps by deployment
- How targets control where config delivers (noop vs real)
- Why some fields mutate directly, some lift upstream, and some are blocked

## Stages

### Stage 1: "What is this app?" (read-only)

Run: `cat deployment-map.json | jq`

Print the full JSON. Explain:
- The app is `inventory-api`, a Spring Boot service
- Three deployments: dev, stage, prod (each becomes a ConfigHub space)
- Three target modes: unbound, noop (default), real
- Three mutation outcomes: apply-here, lift-upstream, block-escalate

**PAUSE.** Wait for the human.

---

### Stage 2: "What will setup create?" (read-only)

Run: `./setup.sh --explain`

Print the full ADT diagram. Explain:
- The ASCII table shows app → deployments → targets
- Default mode is noop: apply workflow works without a cluster
- The mutation outcomes table shows which fields route where

GUI now: Nothing created yet — this is a preview.

If the human wants the same plan in machine-readable form, also run:

`./setup.sh --explain-json | jq`

**PAUSE.** Wait for the human.

---

### Stage 2b: "How does field ownership work?" (read-only)

Show the **field lineage** - why some fields are mutable vs blocked:

Run: `../springboot-platform-app/generator/render.sh --explain-field spring.datasource.url`

Print the full output. Explain:
- This field is BLOCKED because the generator injects it from platform policy
- It's not in the app inputs at all - it comes from `runtime-policy.yaml`
- This is the provenance: where the field value comes from

Now show a mutable field:

Run: `../springboot-platform-app/generator/render.sh --explain-field feature.inventory.reservationMode`

Explain:
- This field is MUTABLE because it comes from app inputs
- The generator passes it through without transformation
- App team owns it, so ConfigHub can change it directly

For the full transformation trace:

Run: `../springboot-platform-app/generator/render.sh --trace`

GUI now: Nothing — this is CLI-only understanding of the transformation.

**PAUSE.** Wait for the human.

---

### Stage 3: "Create the config" (mutates ConfigHub)

Ask: "This will create 4 spaces (3 env + 1 infra), 3 units, 3 noop targets, and apply all units. OK?"

Run: `./setup.sh`

Print the full output. Then verify:

Run: `cub space list --where "Labels.ExampleName = 'springboot-platform-app'" --json | jq '.[].Space.Slug'`

Explain: Four spaces now exist in ConfigHub.

GUI now: Open https://hub.confighub.com → click **Spaces** in sidebar → you should see `inventory-api-dev`, `inventory-api-stage`, `inventory-api-prod`, and `inventory-api-infra`.

GUI gap: The Spaces list doesn't show which spaces belong to the same app. You have to filter by label or read the names.

GUI ask: An "App" grouping or column in the Spaces view. No issue filed yet.

**PAUSE.** Wait for the human.

---

### Stage 4: "What are the three mutation outcomes?" (read-only)

Run: `./demo.sh`

Print the full output for all three scenarios. Explain:
- **Apply here**: `feature.inventory.*` — change directly in ConfigHub, survives refreshes
- **Lift upstream**: `spring.cache.*` — needs to go back to app source (Redis example)
- **Block/escalate**: `spring.datasource.*` — platform-owned, requires approval

Each outcome is determined by **field lineage** (where the value comes from):

| Outcome | Field Lineage | Provenance |
|---------|---------------|------------|
| Apply here | App inputs, runtime tuning | Mutation history in ConfigHub |
| Lift upstream | App inputs, but needs code change | PR bundle to source repo |
| Block/escalate | Platform policy (not app inputs) | Escalation to platform team |

To see why a specific field routes a certain way, use `--explain-field` (see Stage 2b).

GUI now: Open https://hub.confighub.com → Spaces → click `inventory-api-prod` → click the `inventory-api` unit → see the rendered ConfigMap and Deployment YAML.

GUI gap: There are no route badges showing which fields are mutable vs platform-owned. Every field looks equally editable.

GUI ask: Colored route badges (green=mutable, yellow=lift-upstream, red=blocked) next to each field. Issue #209.

**PAUSE.** Wait for the human.

---

### Stage 5: "Try a mutation" (mutates ConfigHub)

Ask: "This will change the reservation mode for prod from strict to optimistic. OK?"

Run:
```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: reservation mode strict → optimistic" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic
```

Print the output. Then show the change:

Run: `cub unit get --space inventory-api-prod inventory-api --json | jq '.Unit.Objects[] | select(.kind == "Deployment") | .spec.template.spec.containers[0].env[] | select(.name == "FEATURE_INVENTORY_RESERVATIONMODE")'`

Now show the **provenance** - the mutation history:

Run: `cub mutation list --space inventory-api-prod --json inventory-api | jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'`

Print the output. Explain: The env var now says `optimistic`. This mutation:
- Is stored in ConfigHub with your author identity
- Has a timestamp and change description (visible in mutation history)
- Will survive future refreshes (PRESERVE policy for mutable-in-ch fields)

The mutation history is the **audit trail** - every change is recorded with who, when, and why.

GUI now: Open https://hub.confighub.com → Spaces → `inventory-api-prod` → `inventory-api` unit → look at the Deployment YAML → find `FEATURE_INVENTORY_RESERVATIONMODE`. It should say `optimistic`. Click "History" to see the mutation.

GUI gap: The mutation history exists but doesn't highlight which specific fields changed.

GUI ask: Field-level diff in mutation history. Issue #211.

**PAUSE.** Wait for the human.

---

### Stage 6: "Apply to target" (mutates ConfigHub)

Run: `cub unit apply --space inventory-api-prod inventory-api`

Print the output. Explain:
- The unit was applied to the noop target
- Noop accepts the apply but doesn't deliver to a real cluster
- This proves the workflow works; use `--with-targets` for real delivery

GUI now: Open https://hub.confighub.com → Spaces → `inventory-api-prod` → `inventory-api` unit → check the sync status. It should show `Synced` or `ApplyCompleted`.

**PAUSE.** Wait for the human.

---

### Stage 6b: "Running app sees the change" (optional, requires Java)

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
cd ../../springboot-platform-app-centric
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

Explain: The running app reports `optimistic` - the mutation from Stage 5 is visible in a real HTTP response.

**PAUSE.** Wait for the human.

---

### Stage 7: "Explore the flow docs" (read-only, optional)

Run: `cat flows/apply-here.md`

Print the full content. Explain this is a detailed walkthrough of the apply-here mutation path.

Mention: There are also `flows/lift-upstream.md` and `flows/block-escalate.md` for the other two outcomes.

**PAUSE.** Wait for the human.

---

### Stage 8: "Cleanup"

Run: `./cleanup.sh`

Print the output. Explain: All spaces with the example label have been deleted.

---

## Key files

| File | Purpose |
|------|---------|
| `deployment-map.json` | Machine-readable ADT structure |
| `flows/apply-here.md` | Apply-here mutation walkthrough |
| `flows/lift-upstream.md` | Lift-upstream mutation walkthrough |
| `flows/block-escalate.md` | Block/escalate mutation walkthrough |
| `../springboot-platform-app/operational/field-routes.yaml` | Field routing rules |

## Delegation model

This example delegates all implementation to `../springboot-platform-app/`:
- `./setup.sh` → `../springboot-platform-app/confighub-setup.sh`
- `./verify.sh` → `../springboot-platform-app/verify.sh`
- `./cleanup.sh` → `../springboot-platform-app/confighub-cleanup.sh`

No duplication of upstream code, operational config, or worker binaries.
