# AI Start Here

## What this example is for

Demonstrates ConfigHub as the **write API for platform config** that GitOps-heavy
teams are missing. The scenario: you have hundreds of services, all config in Git,
no programmatic way to update a field without a full clone/branch/edit/PR cycle.

ConfigHub solves this by being the mutation plane. This example proves it with
a concrete service (`inventory-api`) across three environments.

## Safe first steps

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

## Exact commands to run

```bash
# 1. Preview
./setup.sh --explain

# 2. Create (mutates ConfigHub)
./setup.sh

# 3. Inspect
./compare.sh
./field-routes.sh prod

# 4. Mutate (the "write API" proof)
./mutate.sh

# 5. Verify mutation
./compare.sh
./refresh-preview.sh prod

# 6. Cleanup
./cleanup.sh
```

## What mutates what

| Script | What it creates/changes |
|--------|------------------------|
| `setup.sh` | 3 spaces + 3 units in ConfigHub |
| `mutate.sh` | Sets `FEATURE_INVENTORY_RESERVATIONMODE=optimistic` on prod unit |
| `cleanup.sh` | Deletes all spaces labeled `ExampleName=platform-write-api` |

Everything else is read-only.
