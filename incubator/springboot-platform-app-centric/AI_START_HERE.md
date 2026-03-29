# AI Start Here: Spring Boot Platform App (App-Centric)

This is the app-centric entry point for understanding ConfigHub mutation routing.

## What This Example Teaches

One app (`inventory-api`) with three deployments (dev, stage, prod) and three mutation outcomes:

1. **Apply here**: Direct ConfigHub mutation, survives refreshes
2. **Lift upstream**: Route change back to app source repo
3. **Block / escalate**: Platform-owned field, requires approval

## Machine-Readable Entry

```bash
cat deployment-map.json | jq
```

This gives you:
- The app name and source location
- All three deployments with their spaces
- All three target modes (unbound, noop, real)
- All three mutation outcomes with example fields

## Read-Only Preview

```bash
./setup.sh --explain
```

This shows the App-Deployment-Target view without mutating anything.

## Default Path

```bash
./setup.sh
```

This creates ConfigHub spaces, units, and noop targets. The apply workflow works immediately without a cluster.

## Verify

```bash
./verify.sh
```

This delegates to the parent example's verification.

## Cleanup

```bash
./cleanup.sh
```

This deletes all example-labeled ConfigHub objects.

## Key Files to Read

| File | Purpose |
|------|---------|
| `deployment-map.json` | Machine-readable ADT structure |
| `flows/apply-here.md` | Apply-here mutation walkthrough |
| `flows/lift-upstream.md` | Lift-upstream mutation walkthrough |
| `flows/block-escalate.md` | Block/escalate mutation walkthrough |
| `../springboot-platform-app/operational/field-routes.yaml` | Field routing rules |
| `../springboot-platform-app/example-summary.json` | Full example contract |

## Delegation Model

This example delegates all implementation to `../springboot-platform-app/`:

- `./setup.sh` calls `../springboot-platform-app/confighub-setup.sh`
- `./verify.sh` calls `../springboot-platform-app/verify.sh`
- `./cleanup.sh` calls `../springboot-platform-app/confighub-cleanup.sh`

No duplication of upstream code, operational config, or worker binaries.

## Suggested AI Workflow

1. Read `deployment-map.json` for structure
2. Run `./setup.sh --explain` to preview
3. Run `./setup.sh` to create objects
4. Read one flow doc (`flows/apply-here.md`) to understand mutation routing
5. Try a mutation using `cub function do`
6. Run `./cleanup.sh` when done

## What Not To Do

- Do not duplicate files from `../springboot-platform-app/`
- Do not modify upstream/ or operational/ directories (they live in the parent)
- Do not run setup without reading `--explain` first
