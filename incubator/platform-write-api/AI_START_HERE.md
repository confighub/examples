# AI Start Here

## What this example is for

Demonstrates ConfigHub as the **write API for platform config** that GitOps-heavy
teams are missing. The scenario: you have hundreds of services, all config in Git,
no programmatic way to update a field without a full clone/branch/edit/PR cycle.

ConfigHub solves this by being the mutation plane. This example proves it with
a concrete service (`inventory-api`) across three environments.

## CRITICAL: Demo pacing

**When walking a human through this demo, you MUST pause after every stage.**

Do not race ahead. After each stage:

1. Run the command(s) for that stage
2. Print the full output on screen — do not summarize or abbreviate
3. Explain what the output means in plain English
4. If there is a GUI URL, print it and say "You can also see this in the GUI: [URL]"
5. **STOP and ask** "Ready to continue?" or "Want to look at this more?"
6. Only proceed when the human says to continue

The human must be able to study each stage before you move on. This is a demo,
not a script execution. The value is in *seeing and understanding* each step.

## Demo stages (in order)

### Stage 0: "What does this example do?" (read-only)

```bash
./setup.sh --explain
```

Print the full output. Explain: "This shows what the setup script would create
in ConfigHub. Nothing has been created yet."

**Pause.** Wait for the human.

### Stage 1: "Create the config in ConfigHub" (mutates)

Ask first: "This will create 3 spaces and 3 units in ConfigHub. OK to proceed?"

```bash
./setup.sh
```

Print the full output. Then verify:

```bash
cub space list --where "Labels.App = 'inventory-api'" --json
```

Print the result. Explain: "These three spaces now exist in ConfigHub. Each one
represents an environment for inventory-api."

GUI: "You can see these in the GUI right now. Open https://hub.confighub.com,
click Units in the sidebar, and type 'inventory-api' in the search bar. You'll
see three units in a data grid — one per environment."

**Pause.** Wait for the human. Give them time to open the GUI if they want.

### Stage 2: "What is your config, across all environments?" (read-only)

```bash
./compare.sh
```

Print the **full table**. Do not truncate. Explain each column:
- FIELD: the config property name
- DEV / STAGE / PROD: the value in each environment
- ROUTE: what you're allowed to do with this field

Point out: "Right now, if you wanted to know what reservationMode is set to in
prod, you'd have to clone a repo, find the right YAML file, and parse it. Here
it's one command."

GUI: "In the GUI, you can click into each unit to see its field values, but you
have to look at one environment at a time. There is no side-by-side comparison
view across environments yet — that's a planned GUI feature. The CLI table above
is currently the best way to see all three environments at once."

**Pause.** Wait for the human.

### Stage 3: "Who owns each field?" (read-only)

```bash
./field-routes.sh prod
```

Print the **full output**. Explain the three route types:
- **mutable-in-ch**: "You can change this directly in ConfigHub. No Git, no PR."
- **lift-upstream**: "This needs to go back to the source repo as a code change."
- **generator-owned**: "The platform controls this. You cannot change it here."

Say: "This is field-level governance. Every field has exactly one classification.
There is no ambiguity about what you can and cannot change."

GUI: "In the GUI, click into the inventory-api-prod unit and look at the rendered
YAML. You can see the field values, but there are no route badges or ownership
annotations yet — every field looks equally editable. The colored route badges
(green=mutable, yellow=lift-upstream, red=generator-owned) are a planned GUI
feature. For now, the CLI output above is where you see the routing rules."

**Pause.** Wait for the human.

### Stage 4: "The write API — old way vs new way" (read-only preview)

```bash
./mutate.sh --explain
```

Print the **full output** showing the 8-step GitOps way vs the 1-command
ConfigHub way. Let the comparison sink in.

Say: "On the left is what you do today. On the right is what ConfigHub gives you."

**Pause.** Wait for the human.

### Stage 5: "Do the mutation" (mutates)

Ask first: "This will change reservationMode for prod from strict to optimistic.
One API call to ConfigHub. OK to proceed?"

```bash
./mutate.sh
```

Print the full output. Then immediately show the before/after:

```bash
./compare.sh
```

Print the **full table again**. Point to the `*` on prod's reservationMode:
"See the asterisk? That means this value now diverges from the upstream default.
ConfigHub recorded this mutation with your author identity, a timestamp, and
the change description."

If possible, also show the mutation history:

```bash
cub mutation list --space inventory-api-prod --json inventory-api 2>/dev/null || echo "(mutation list not available in current CLI version)"
```

GUI: "Now go back to the GUI. Click on the inventory-api unit in space
inventory-api-prod. Look at the rendered ConfigMap — the value of
FEATURE_INVENTORY_RESERVATIONMODE should now say 'optimistic'. Then check
the Activity or History tab on the unit — you should see the mutation with the
description 'rollout: reservation mode strict → optimistic', your username,
and a timestamp. That's the audit trail."

**Pause.** Wait for the human. Give them time to click through the GUI.

### Stage 6: "Does this survive when the generator re-renders?" (read-only)

```bash
./refresh-preview.sh prod
```

Print the **full output**. Explain:
- `PRESERVE`: "Your mutation wins. The generator won't overwrite this."
- `REFRESH`: "This field matches upstream, no conflict."

Say: "This is the key question for any generated config system. When the
platform re-renders, does your change survive? The answer is yes, because
the merge policy is PRESERVE for mutable-in-ch fields. Today this is
simulated client-side. Server-side enforcement is in development."

GUI: "There is no GUI equivalent for this refresh preview yet. The Promotion
view shows diffs for upgrades and applies, but there's no 'what would happen
on re-render' preview. That's a planned feature. The CLI output above is the
current way to see per-field merge decisions."

**Pause.** Wait for the human.

### Stage 7 (optional): "Prove the app sees it" (requires Java)

Only offer this if Java and Maven are installed. Ask first.

```bash
cd ../springboot-platform-app/upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic mvn spring-boot:run -q -Dspring-boot.run.profiles=prod &
sleep 10
curl -s http://localhost:8081/api/inventory/summary | jq
```

Print the full JSON response. Point to `"reservationMode": "optimistic"`.

Say: "The running app reports the new value. The env var
FEATURE_INVENTORY_RESERVATIONMODE maps to feature.inventory.reservationMode
via Spring Boot relaxed binding. This is a real HTTP response from a real
running app."

GUI: "No GUI involvement here — this is a local app proof. The app is running
on your machine and you can curl it directly."

Kill the background process when done.

**Pause.** Wait for the human.

### Stage 8: "Cleanup"

```bash
./mutate.sh --revert
./cleanup.sh
```

Print the output. Say: "Everything is cleaned up. The spaces and units are gone."

## Safe first steps (non-demo, exploration mode)

If the human is not doing a demo but just exploring:

```bash
# Read-only — no ConfigHub mutation
./setup.sh --explain
./compare.sh
./field-routes.sh prod
./refresh-preview.sh prod
```

All scripts fall back to fixture files when ConfigHub is unavailable.

## Capability branching

| If you have... | You can run... |
|----------------|---------------|
| Just the repo | All `--explain` modes, `compare.sh`, `field-routes.sh`, `refresh-preview.sh` (fixture fallback) |
| `cub auth login` | `setup.sh`, `mutate.sh`, `cleanup.sh` (ConfigHub mutation) |
| Java 21 + Maven | `../springboot-platform-app/upstream/app/` — run the actual app |

## What mutates what

| Script | What it creates/changes |
|--------|------------------------|
| `setup.sh` | 3 spaces + 3 units in ConfigHub |
| `mutate.sh` | Sets `FEATURE_INVENTORY_RESERVATIONMODE=optimistic` on prod unit |
| `mutate.sh --revert` | Reverts `FEATURE_INVENTORY_RESERVATIONMODE` to `strict` |
| `cleanup.sh` | Deletes all spaces labeled `ExampleName=platform-write-api` |

Everything else is read-only.

## GUI deep links

ConfigHub GUI URLs follow these patterns. Use them when guiding humans
to see the same data in the browser.

| What to show | URL pattern |
|-------------|------------|
| All units (filtered) | `https://hub.confighub.com/units?q=inventory-api` |
| Specific space | `https://hub.confighub.com/spaces/{space-name}` |
| Specific unit | `https://hub.confighub.com/spaces/{space-name}/units/{unit-name}` |
| Promotion view | `https://hub.confighub.com/tools/promotion` |
| GitOps Import | `https://hub.confighub.com/tools/gitops-import` |

Note: Deep link support for individual fields, mutation history, or diff
views is not available yet. Guide the user to navigate manually when needed.
If the exact URL patterns above are not working, the base URL is
`https://hub.confighub.com` — navigate from there.

## New: diff.sh

Compare only the differences between two environments:

```bash
./diff.sh dev prod        # Human-readable differences
./diff.sh --json dev prod # Machine-readable
```

This is a quick way to answer "what's different between dev and prod?"
without reading the full comparison table.
