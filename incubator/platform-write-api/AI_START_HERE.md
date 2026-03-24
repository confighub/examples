# AI Start Here

## What this example is for

Demonstrates ConfigHub as the **write API for platform config** that GitOps-heavy
teams are missing. The scenario: you have hundreds of services, all config in Git,
no programmatic way to update a field without a full clone/branch/edit/PR cycle.

ConfigHub solves this by being the mutation plane. This example proves it with
a concrete service (`inventory-api`) across three environments.

## The three mutation scenarios

This demo proves three kinds of config change, each with different routing:

| Scenario | Route | Example | Script |
|----------|-------|---------|--------|
| 1. Direct mutation | `mutable-in-ch` | Flip a feature flag | `./mutate.sh` |
| 2. Lift upstream | `lift-upstream` | Enable Redis caching | `./lift-upstream.sh` |
| 3. Block/escalate | `generator-owned` | Change datasource URL | `./block-escalate.sh` |

## Safe first steps

```bash
# Read-only — no ConfigHub mutation
./setup.sh --explain
./compare.sh
./field-routes.sh prod
./refresh-preview.sh prod
./lift-upstream.sh
./block-escalate.sh --render-attempt
```

All scripts fall back to fixture files when ConfigHub is unavailable.

## Capability branching

| If you have... | You can run... |
|----------------|---------------|
| Just the repo | All `--explain` modes, `compare.sh`, `field-routes.sh`, `refresh-preview.sh`, `lift-upstream.sh`, `block-escalate.sh` (fixture/simulation) |
| `cub auth login` | `setup.sh`, `mutate.sh`, `cleanup.sh` (ConfigHub mutation) |
| Java 21 + Maven | `../springboot-platform-app/upstream/app/` — run the actual app |

## Exact commands to run

```bash
# 1. Preview
./setup.sh --explain

# 2. Create (mutates ConfigHub)
./setup.sh

# 3. Inspect
./compare.sh
./field-routes.sh prod

# 4. Scenario 1: Direct mutation (the "write API" proof)
./mutate.sh --explain          # Old way (8 steps) vs new way (1 command)
./mutate.sh                    # Do the mutation

# 5. Verify mutation
./compare.sh                   # See the * divergence marker on prod
./refresh-preview.sh prod      # See PRESERVE — mutation survives refresh

# 6. Scenario 2: Lift upstream (structural changes)
./lift-upstream.sh             # Routing decision
./lift-upstream.sh --render-diff  # GitHub-ready patch

# 7. Scenario 3: Block/escalate (platform-owned fields)
./block-escalate.sh            # Routing decision
./block-escalate.sh --render-attempt  # Simulated blocked mutation

# 8. Cleanup
./mutate.sh --revert
./cleanup.sh
```

## What mutates what

| Script | What it creates/changes |
|--------|------------------------|
| `setup.sh` | 3 spaces + 3 units in ConfigHub |
| `mutate.sh` | Sets `FEATURE_INVENTORY_RESERVATIONMODE=optimistic` on prod unit |
| `cleanup.sh` | Deletes all spaces labeled `ExampleName=platform-write-api` |

Everything else is read-only.
