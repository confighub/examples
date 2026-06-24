# AI Start Here: Initiatives Demo

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
Read initiatives-demo/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Do not continue until I say continue.
```

## What This Example Teaches

After this demo, the human will understand:
- What ConfigHub initiatives are (compliance tracking views)
- ConfigHub's **component model**: one Space per component, with the well-known
  component labels (Component, Variant, Owner, Team, Layer) on the Space
- How initiatives filter units **across spaces** by Space labels
  (`Space.Labels.Component`, `Space.Labels.Owner`)
- Initiative metadata: priority, status, deadlines
- How Kyverno CEL policies can validate units
- How the same app model (aichat, eshop, etc.) works across demos

No cluster required to explore. The optional vet-kyverno triggers require the
worker that `setup.sh` installs into a local kind cluster.

## Stage 1: "Understand Initiatives" (read-only)

```bash
cd initiatives-demo
cat README.md | head -70
```

What to explain:
- 5 compliance initiatives, each backed by a Kyverno CEL policy
- 18 units sourced from the promotion demo's app model (aichat, website, docs, eshop, portal)
- Each component gets its own Space; the platform Space `initiatives-demo` holds
  the shared Views, Filters, and Triggers but no application units
- Initiatives are Views with filters and metadata in Labels/Annotations
- Use case: track remediation progress, set priorities, run automated checks

GUI now: No GUI checkpoint for this stage — model orientation is CLI-only.

GUI gap: No visual initiative overview before setup.

GUI feature ask: Initiative template gallery. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 2: "Create The Demo Data" (mutates ConfigHub)

```bash
./setup.sh
```

What to explain:
- Creates the platform Space `initiatives-demo` (workers, filters, views, triggers)
- Creates one Space per component (aichat, portal, eshop, docs, website), each
  stamped with the component labels and `Purpose=initiatives-demo`
- Creates 18 units from the promotion demo YAML files, in their component Space
- Creates 5 initiative views whose filters select units across the component spaces
- If a worker is detected, triggers are attached (disabled by default, then
  enabled one at a time)

GUI now: ConfigHub → Spaces → filter by `Purpose=initiatives-demo` → see the
component spaces and their units, plus the `initiatives-demo` platform space.

GUI gap: No initiative dashboard with progress indicators.

GUI feature ask: Initiative overview page with status badges and progress bars. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 3: "Explore Units" (read-only)

```bash
# List the demo's spaces
cub space list --where "Labels.Purpose = 'initiatives-demo'"

# List units in one component space
cub unit list --space aichat --json | jq '.[].Unit.Slug'

# List units across all component spaces by Space label
cub unit list --space "*" --where "Space.Labels.Component = 'eshop'" --json | jq '.[].Unit.Slug'
cub unit list --space "*" --where "Space.Labels.Owner = 'Support'" --json | jq '.[].Unit.Slug'
```

What to explain:
- Each component has its own Space, labeled with `Component` (aichat, eshop, …)
  and `Owner` (Support, Product, Marketing) — the labels live on the Space
- `--space "*"` searches across spaces; the `Space.Labels.*` predicates select
  units by their Space's labels
- Each initiative filters units by these Space labels

GUI now: ConfigHub → Spaces → `aichat` → Units

GUI gap: No unit compliance summary at a glance.

GUI feature ask: Compliance badge on unit list. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 4: "Explore Initiatives" (read-only)

```bash
# List all initiative views (they live in the platform space)
cub view list --space initiatives-demo --where "Labels.initiative = 'true'" --json | jq '.[].View.Slug'

# Filter by priority
cub view list --space initiatives-demo --where "Labels.initiative-priority = 'HIGH'" --json | jq '.[].View.Slug'

# Filter by status
cub view list --space initiatives-demo --where "Labels.initiative-status = 'in_progress'" --json | jq '.[].View.Slug'
```

What to explain:
- Initiatives are marked with `initiative=true` label
- Priority: HIGH, MEDIUM, LOW
- Status: draft, in_progress, completed
- Each initiative has a description and deadline in annotations

GUI now: ConfigHub → Views → filter by `initiative=true`

GUI gap: No dedicated initiative list view with columns for priority/status/deadline.

GUI feature ask: Initiative management dashboard. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 5: "Inspect An Initiative" (read-only)

```bash
# Get details of a specific initiative
cub view get --space initiatives-demo --json liveness-and-readiness-probes | jq

# See the initiative metadata
cub view get --space initiatives-demo --json liveness-and-readiness-probes | jq '.View.Annotations'
```

What to explain:
- `initiative-description`: Human-readable goal
- `initiative-deadline`: Target completion date
- `initiative-trigger-id`: Associated vet-kyverno trigger (if worker available)
- The View's filter selects which units this initiative evaluates, across spaces
  (this one: `Space.Labels.Owner = 'Support'`)

GUI now: ConfigHub → Views → `liveness-and-readiness-probes` → see filter and annotations.

GUI gap: No inline policy preview.

GUI feature ask: Initiative detail page with embedded policy viewer. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 6: "Cleanup"

```bash
./cleanup.sh
```

This removes the component spaces and the `initiatives-demo` platform space
(and the local kind cluster).

## Optional: vet-kyverno Triggers

`setup.sh` already installs a vet-kyverno worker into a local kind cluster and
attaches a trigger per initiative. To exercise one, mutate a unit and watch the
result land in `ApplyWarnings` (advisory) or `ApplyGates` (the enforced
"Disallow Host Ports" initiative):

```bash
# Mutate a unit, then inspect the failure messages behind any gate/warning
cub unit get aichat-redis --space aichat -o "jq=.Unit.ApplyWarnings"
cub unit get aichat-redis --space aichat -o "jq=.Unit.ValidationResults"
```

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh` | 6 spaces (1 platform + 5 component), 18 units, 5 views/filters/triggers, workers |
| `cub space list` | Nothing |
| `cub unit list` | Nothing |
| `cub view list` | Nothing |
| `cub view get` | Nothing |
| `./cleanup.sh` | Deletes all the demo's spaces + kind cluster |

## Related Files

- [README.md](./README.md)
- [promotion-demo-data/](../promotion-demo-data/) — source YAML files
