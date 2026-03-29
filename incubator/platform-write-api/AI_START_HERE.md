# AI Start Here

## CRITICAL: Demo pacing

When walking a human through this example, pause after every stage.

After each stage:

1. run only that stage's command(s)
2. show the output faithfully; if it is long, keep the important section visible
3. explain what the output means in plain English
4. say whether the stage mutated ConfigHub or only read fixtures/state
5. print the GUI checkpoint when one exists
6. say what the GUI shows today
7. say what the GUI does not show yet
8. name the GUI feature ask and cite an issue if one is known
9. tell the human to inspect the GUI before continuing
10. stop and ask `Ready to continue?`

This example is about understanding the mutation plane, not racing through shell commands.

## Suggested prompt

```text
Read incubator/platform-write-api/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Do not continue until I say continue.
```

## What this example teaches

This example makes the ConfigHub write-API story concrete for one service, `inventory-api`, across three environments.

It proves three routing outcomes:

| Scenario | Route | Example | Script |
|----------|-------|---------|--------|
| Apply here | `mutable-in-ch` | flip a feature flag | `./mutate.sh` |
| Lift upstream | `lift-upstream` | enable Redis caching | `./lift-upstream.sh` |
| Block / escalate | `generator-owned` | change datasource URL | `./block-escalate.sh` |

## Capability branching

| If you have... | You can run... |
|----------------|---------------|
| Just the repo | `./setup.sh --explain`, `./setup.sh --explain-json`, `./compare.sh --json`, `./field-routes.sh --json`, `./refresh-preview.sh --json`, `./lift-upstream.sh`, `./block-escalate.sh` |
| `cub auth login` | `./setup.sh`, `./mutate.sh`, `./cleanup.sh` |
| Java 21 + Maven | the runnable sibling at `../springboot-platform-app/upstream/app/` |

All read-only scripts fall back to local fixtures when ConfigHub is unavailable.

## Stage 1: Preview the example shape (read-only)

Run:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
```

Explain:

- this is a ConfigHub-only proof, not a live deployment example
- the example creates three spaces and one unit per space
- the machine-readable plan tells the AI what will be created and what cleanup removes

GUI now: nothing has been created yet.

GUI gap: there is no single preview page that shows “what this example will create” before you run it.

GUI feature ask: a preflight example plan view that mirrors `--explain-json`. No issue filed yet.

PAUSE: let the human decide whether to stay read-only or create the objects.

## Stage 2: Compare the three environments first (read-only)

Run:

```bash
./compare.sh
./compare.sh --json | jq
```

Explain:

- this is the baseline before any mutation
- `dev`, `stage`, and `prod` all show the same key fields in one place
- the `--json` output is the stable AI contract for route-aware comparison

GUI now: if the example has not been set up yet, there is no GUI checkpoint for this stage.

GUI gap: there is no route-aware compare page that shows environment drift with the same compactness as the CLI table.

GUI feature ask: one compare view for a unit family across environments. No issue filed yet.

PAUSE: make sure the human understands the baseline before moving into routing.

## Stage 3: Show why each field takes a different path (read-only)

Run:

```bash
./field-routes.sh prod
./field-routes.sh prod --json | jq
```

Explain:

- `feature.inventory.*` is app-owned and mutable in ConfigHub
- `spring.cache.*` is app-owned but routes upstream because it changes the app contract
- `spring.datasource.*` is platform-owned and blocked

Now show the **field lineage** - where does each field come from?

Run:

```bash
../springboot-platform-app/generator/render.sh --explain-field spring.datasource.url
../springboot-platform-app/generator/render.sh --explain-field feature.inventory.reservationMode
```

Explain:

- `spring.datasource.url` is BLOCKED because the generator injects it from platform policy (not app inputs)
- `feature.inventory.reservationMode` is MUTABLE because it comes from app inputs

This is **provenance** - understanding where field values come from determines how they can be changed.

GUI now: if objects already exist, open ConfigHub and inspect the `inventory-api-prod` unit YAML.

GUI gap: the GUI does not annotate fields with their route or owner, so the user cannot see why one field is safe and another is blocked.

GUI feature ask: field-level route badges and ownership hints in the unit data view. No issue filed yet.

PAUSE: this is the key mental model for the rest of the demo.

## Stage 4: Create the ConfigHub objects (mutates ConfigHub only)

Ask:

`This will create 3 spaces, 3 units, and no live-cluster resources. OK?`

Run:

```bash
./setup.sh
cub space list --where "Labels.ExampleName = 'platform-write-api'" --json | jq '.[].Space.Slug'
```

Explain:

- this writes ConfigHub state only
- no Git repo is changed
- no cluster is touched

GUI now: open ConfigHub and inspect `inventory-api-dev`, `inventory-api-stage`, and `inventory-api-prod`.

GUI gap: the spaces view does not tell you these spaces belong to one example story with one shared route model.

GUI feature ask: example-aware grouping for spaces created by one walkthrough. No issue filed yet.

PAUSE: wait until the human has confirmed the spaces are visible.

## Stage 5: Show the write API itself (mutates ConfigHub only)

Run:

```bash
./mutate.sh --explain
./mutate.sh
```

Explain:

- the write-API proof is the direct mutation from `strict` to `optimistic`
- this is audited, attributable, and reversible
- this is not yet a live deployment proof because there is no real target here

Now show the **provenance** - the mutation history:

Run:

```bash
cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

Explain:

- Every mutation is recorded with who, when, and why
- The change description "apply-here: reservation mode rollout" is in the audit trail
- Your author identity and timestamp are captured
- This is the **provenance** for direct mutations - the decision log that ConfigHub provides

GUI now: open the `inventory-api` unit in `inventory-api-prod` and inspect the rendered Deployment data. Click "History" to see the mutation with your change description.

GUI gap: the mutation history does not highlight the exact field delta inline.

GUI feature ask: field-level mutation diffs in the history view. No issue filed yet.

PAUSE: let the human inspect the changed field and mutation history before verifying preservation.

## Stage 6: Verify that the mutation survives refresh (read-only)

Run:

```bash
./compare.sh
./refresh-preview.sh prod
./refresh-preview.sh prod --json | jq
```

Explain:

- the `*` marker in `./compare.sh` means prod now diverges intentionally
- `./refresh-preview.sh` should show `PRESERVE` for the mutable ConfigHub override
- this is the heart of the “ConfigHub is the mutation plane” claim

GUI now: the unit data should still show `FEATURE_INVENTORY_RESERVATIONMODE=optimistic`.

GUI gap: there is no “survives refresh because of route policy” explanation in the GUI.

GUI feature ask: preserve/refresh decision explanations for mutated fields. No issue filed yet.

PAUSE: make sure the human has seen both the divergence and the preservation logic.

## Stage 7: Running App Sees The Change (optional, requires Java)

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
cd ../../..
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

Explain:

- The running app reports `optimistic` - the mutation is visible
- Real HTTP response, real Spring Boot app
- This proves ConfigHub mutations can be replayed to a live app

GUI now: no GUI involvement — this is a local app proof.

PAUSE: if skipped, explain this is optional and move to the next stage.

## Stage 8: Show the other two routing outcomes (read-only)

Run:

```bash
./lift-upstream.sh
./lift-upstream.sh --render-diff
./block-escalate.sh
./block-escalate.sh --render-attempt
```

Explain:

- Redis caching is a structural change and routes upstream
- datasource changes are platform-owned and should be blocked
- the block/escalate example is honest about its current boundary: client-side story today, server-side enforcement still missing

GUI now: there is no first-class review queue for lift-upstream or block/escalate actions in this example.

GUI gap: the product cannot yet show one timeline that connects intent, route decision, PR handoff, and blocked attempts.

GUI feature ask: route-aware request timeline plus blocked-request review queue. Server-side blocking is tracked in `cub-gen #207`.

PAUSE: this is where the user decides whether they care more about direct mutation, upstream routing, or policy enforcement.

## Stage 9: Cleanup

Run:

```bash
./mutate.sh --revert
./cleanup.sh
```

Explain:

- this reverts the demo mutation before deleting the example spaces
- cleanup only removes ConfigHub objects labeled for this example
- no Git or cluster cleanup is needed because nothing outside ConfigHub was touched
