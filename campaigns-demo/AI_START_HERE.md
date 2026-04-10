# AI Start Here: Campaigns Demo

Read [`README.md`](./README.md) first. This page explains how to demo it.

## CRITICAL: Demo Pacing

When walking a human through this example, pause after every stage.

After each stage:
1. Run the commands for that stage
2. Show full output (do not summarize)
3. Explain what the output means
4. If there is a GUI checkpoint, print it
5. Ask "Ready to continue?"
6. Wait for the human before proceeding

## Suggested Prompt

```text
Read campaigns-demo/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Do not continue until I say continue.
```

## What This Example Teaches

After this demo, the human will understand:
- What ConfigHub campaigns are (compliance tracking views)
- How campaigns filter units by App and AppOwner labels
- Campaign metadata: priority, status, deadlines
- How Kyverno CEL policies can validate units
- How the same app model (aichat, eshop, etc.) works across demos

No cluster required. Optional vet-kyverno trigger requires a worker.

## Stage 1: "Understand Campaigns" (read-only)

```bash
cd campaigns-demo
cat README.md | head -70
```

What to explain:
- 5 compliance campaigns, each backed by a Kyverno CEL policy
- 18 units sourced from the promotion demo's app model (aichat, website, docs, eshop, portal)
- Campaigns are Views with filters and metadata in Labels/Annotations
- Units are labeled with App and AppOwner — same labels used in the promotion demo
- Use case: track remediation progress, set priorities, run automated checks

GUI now: No GUI checkpoint for this stage — model orientation is CLI-only.

GUI gap: No visual campaign overview before setup.

GUI feature ask: Campaign template gallery. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 2: "Create The Demo Data" (mutates ConfigHub)

```bash
./setup.sh
```

What to explain:
- Creates space `campaigns-demo`
- Creates 18 units from the promotion demo YAML files
- Creates 5 campaign views with filters and metadata
- If a worker is detected, triggers are attached (disabled by default)

GUI now: ConfigHub → Spaces → `campaigns-demo` → see units and views.

GUI gap: No campaign dashboard with progress indicators.

GUI feature ask: Campaign overview page with status badges and progress bars. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 3: "Explore Units" (read-only)

```bash
# List all units in the demo space
cub unit list --space campaigns-demo --json | jq '.[].Unit.Slug' | head -20

# Filter by app
cub unit list --space campaigns-demo --where "Labels.App = 'aichat'" --json | jq '.[].Unit.Slug'
cub unit list --space campaigns-demo --where "Labels.App = 'eshop'" --json | jq '.[].Unit.Slug'

# Filter by owner
cub unit list --space campaigns-demo --where "Labels.AppOwner = 'Support'" --json | jq '.[].Unit.Slug'
```

What to explain:
- Units are labeled by App (aichat, eshop, etc.) and AppOwner (Support, Product, Marketing)
- Same labels as the promotion demo — consistent data model across examples
- Each campaign filters units by these labels

GUI now: ConfigHub → Units → filter by `App=aichat`

GUI gap: No unit compliance summary at a glance.

GUI feature ask: Compliance badge on unit list. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 4: "Explore Campaigns" (read-only)

```bash
# List all campaign views
cub view list --space campaigns-demo --where "Labels.campaign = 'true'" --json | jq '.[].View.Slug'

# Filter by priority
cub view list --space campaigns-demo --where "Labels.campaign-priority = 'HIGH'" --json | jq '.[].View.Slug'

# Filter by status
cub view list --space campaigns-demo --where "Labels.campaign-status = 'in_progress'" --json | jq '.[].View.Slug'
```

What to explain:
- Campaigns are marked with `campaign=true` label
- Priority: HIGH, MEDIUM, LOW
- Status: draft, in_progress, completed
- Each campaign has a description and deadline in annotations

GUI now: ConfigHub → Views → filter by `campaign=true`

GUI gap: No dedicated campaign list view with columns for priority/status/deadline.

GUI feature ask: Campaign management dashboard. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 5: "Inspect A Campaign" (read-only)

```bash
# Get details of a specific campaign
cub view get --space campaigns-demo --json liveness-and-readiness-probes | jq

# See the campaign metadata
cub view get --space campaigns-demo --json liveness-and-readiness-probes | jq '.View.Annotations'
```

What to explain:
- `campaign-description`: Human-readable goal
- `campaign-deadline`: Target completion date
- `campaign-trigger-id`: Associated vet-kyverno trigger (if worker available)
- The View filter selects which units this campaign evaluates (AppOwner = Support)

GUI now: ConfigHub → Views → `liveness-and-readiness-probes` → see filter and annotations.

GUI gap: No inline policy preview.

GUI feature ask: Campaign detail page with embedded policy viewer. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 6: "Cleanup"

```bash
./cleanup.sh
```

This removes the `campaigns-demo` space and all its contents.

## Optional: vet-kyverno Triggers

If you have a worker with vet-kyverno support:

```bash
# Create a worker
cub worker create --space campaigns-demo kyverno-worker

# Start cub-worker (in another terminal)
cub-worker

# Re-run setup to attach triggers
./setup.sh

# Enable a trigger in the GUI and mutate a unit to fire it
```

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh` | 1 space, 18 units, 5 views, optional triggers |
| `cub unit list` | Nothing |
| `cub view list` | Nothing |
| `cub view get` | Nothing |
| `./cleanup.sh` | Deletes the demo space |

## Related Files

- [README.md](./README.md)
- [promotion-demo-data/](../promotion-demo-data/) — source YAML files
