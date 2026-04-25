# AI Start Here: Promotion Demo Data

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
Read promotion-demo-data/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Do not continue until I say continue.
```

## What This Example Teaches

After this demo, the human will understand:
- The Component-Deployment-Target model (many-to-many relationships)
- How ConfigHub spaces represent deployments
- Multi-dimensional filtering by component, role, region, and owner
- Version skew detection across environments

No cluster required. Uses server-hosted noop workers.

## Stage 1: "Understand The Model" (read-only)

```bash
cd promotion-demo-data
cat README.md | head -60
```

What to explain:
- Component → Deployment → Target is a many-to-many model
- Components: aichat, website, docs, eshop, portal, platform
- Targets: 7 clusters across dev/qa/staging/prod in US/EU
- Deployments: spaces like `us-prod-1-eshop`

GUI now: No GUI checkpoint for this stage — model orientation is CLI-only.

GUI gap: No ER diagram in the GUI.

GUI feature ask: Visual topology diagram showing component-deployment-target relationships. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 2: "Create The Demo Data" (mutates ConfigHub)

```bash
./setup.sh
```

What to explain:
- Creates 49 spaces total
- 7 infrastructure spaces (one per target)
- 42 component deployment spaces
- ~154 units across all spaces
- Uses noop workers (no real cluster needed)

GUI now: ConfigHub → Spaces → filter `ExampleName=demo-data`

GUI gap: No summary view showing space count by dimension.

GUI feature ask: Space aggregation dashboard by label. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 3: "Explore By Component" (read-only)

```bash
# List all demo spaces
cub space list --where "Labels.ExampleName = 'demo-data'" --json | jq '.[].Space.Slug' | head -20

# Filter by component
cub space list --where "Labels.Component = 'eshop'" --json | jq '.[].Space.Slug'

# List units in a specific deployment
cub unit list --space us-prod-1-eshop --json | jq '.[].Unit.Slug'
```

What to explain:
- Each space carries combined labels from Component and Target
- Filter by Component to see all deployments of that component
- Units show the resources that make up each deployment

GUI now: ConfigHub → Spaces → filter by `Component=eshop` → see 7 spaces.

GUI gap: No component-centric grouping view.

GUI feature ask: Component landing page showing all deployments. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 4: "Explore By Environment" (read-only)

```bash
# Filter by target role
cub space list --where "Labels.TargetRole = 'Prod'" --json | jq '.[].Space.Slug'

# Filter by region
cub space list --where "Labels.TargetRegion = 'EU'" --json | jq '.[].Space.Slug'

# Cross-dimensional: prod spaces in US
cub space list --where "Labels.TargetRole = 'Prod' AND Labels.TargetRegion = 'US'" --json | jq '.[].Space.Slug'
```

What to explain:
- TargetRole: Dev, QA, Staging, Prod
- TargetRegion: US, EU
- Multiple clusters can serve the same role (us-dev-1, us-dev-2)

GUI now: ConfigHub → Spaces → filter by `TargetRole=Prod`

GUI gap: No environment matrix view.

GUI feature ask: Role × Region matrix showing deployment counts. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 5: "Detect Version Skew" (read-only)

```bash
# Compare image versions across environments (intentional skew exists)
cub function do get-image api --space us-dev-1-eshop --unit api --output-only
cub function do get-image api --space us-dev-2-eshop --unit api --output-only
cub function do get-image api --space us-prod-1-eshop --unit api --output-only
```

What to explain:
- us-dev-1-eshop has api image `:4.2.0`
- us-dev-2-eshop and us-prod-1-eshop have `:4.2.1`
- This intentional skew demonstrates diff capabilities

GUI now: No GUI checkpoint — version comparison is CLI-only.

GUI gap: No cross-space diff viewer.

GUI feature ask: Version matrix showing image tags across deployments. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 6: "Cleanup"

```bash
./cleanup.sh
```

This removes all 49 spaces and their units.

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh` | 49 ConfigHub spaces, ~154 units, 7 workers, 7 targets |
| `cub space list` | Nothing |
| `cub function do get-image` | Nothing |
| `./cleanup.sh` | Deletes all demo spaces |

## Related Files

- [README.md](./README.md)
