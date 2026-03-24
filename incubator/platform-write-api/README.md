# Platform Write API

## Stack and Scenario

A platform team runs hundreds of services on Kubernetes with GitOps (Flux or Argo).
Git holds all config. CRDs and operators manage the runtime. CI pipelines glue everything together.

The problem: **there is no write API for config.**

- GitHub gives you a URL for a YAML file and a read API.
- There is no programmatic write API for structured config updates.
- So every change requires: clone → branch → edit YAML → commit → push → PR → merge → sync.
- CI becomes the hammer for every nail. Helm charts become the universal answer.
- When everything is in Git, nobody knows what is actually in Git.

This example shows how ConfigHub solves this by becoming the **mutation plane**
for operational config — the write API that platforms are missing.

## What This Proves

1. **Config has a write API.** You can mutate operational config through `cub function do`
   without touching Git, CI, or Helm.
2. **You know what is in your config.** Side-by-side comparison across environments
   with divergence markers shows exactly what differs and why.
3. **Every field has a routing rule.** Some fields are safe to edit directly.
   Some should route upstream. Some are platform-owned and blocked.
4. **Regeneration doesn't destroy your changes.** When upstream re-renders,
   local mutations survive through policy-driven merge (simulated here,
   server-side in production).

## Prerequisites

- `cub` CLI installed and authenticated (`cub auth login`)
- `jq`
- `python3` (for YAML parsing in scripts)

Optional (for the Spring Boot app proof):
- Java 21+, Maven (see `../springboot-platform-app/` for the runnable app)

## What This Reads and Writes

| Script | ConfigHub | Git | Cluster |
|--------|-----------|-----|---------|
| `setup.sh --explain` | reads | - | - |
| `compare.sh` | reads | - | - |
| `field-routes.sh` | reads | - | - |
| `refresh-preview.sh` | reads | - | - |
| `setup.sh` | **writes** (spaces, units) | - | - |
| `mutate.sh` | **writes** (env var mutation) | - | - |
| `cleanup.sh` | **deletes** (example spaces) | - | - |

No script touches Git or any cluster.

## Read-Only Preview

```bash
# What would be created
./setup.sh --explain

# Field routing rules (from fixtures, no ConfigHub needed)
./field-routes.sh prod

# Three-variant comparison (from fixtures)
./compare.sh

# What happens on generator refresh (from fixtures)
./refresh-preview.sh prod
```

## Run It

```bash
# 1. Create the config in ConfigHub
./setup.sh

# 2. Compare across environments
./compare.sh

# 3. See field-level routing
./field-routes.sh prod

# 4. Mutate a field (the write API!)
./mutate.sh

# 5. Compare again — see the divergence marker
./compare.sh

# 6. Preview what would happen on generator refresh
./refresh-preview.sh prod
```

## Expected Output

After `./mutate.sh`, the comparison shows:

```
feature.inventory.reservationMode      optimistic      strict          optimistic*     mutable-in-ch
```

The `*` on prod means: this value has been mutated in ConfigHub and diverges
from the upstream default (`strict`). The route is `mutable-in-ch`, meaning
this mutation is intentional and should survive generator refreshes.

The refresh preview confirms:

```
feature.inventory.reservationMode         PRESERVE
  live:     optimistic
  upstream: strict
  reason:   local override via mutable-in-ch route
```

## Verify It

```bash
./verify.sh
```

## Inspect It In The GUI

After `./setup.sh`:
- Open ConfigHub → navigate to space `inventory-api-prod`
- View the `inventory-api` unit
- See the operational config (ConfigMap, Deployment, Service)

After `./mutate.sh`:
- Check the unit's mutation history
- The change description says "apply-here: reservation mode rollout"

## Cleanup

```bash
./cleanup.sh
```

## What This Does Not Prove

- **Live cluster delivery.** No targets, no pods. Use `../springboot-platform-app/`
  with `--with-targets` for noop target proof.
- **Automated lift-upstream PR creation.** The diff bundle exists but no PR is created.
- **Block/escalate enforcement.** The boundary rules exist but are not server-enforced.
- **Server-side refresh survival.** The merge preview is client-simulated.

These are documented gaps with filed issues (cub-gen #207, #208, #209-#213).
