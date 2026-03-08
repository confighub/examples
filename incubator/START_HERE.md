# Start Here: ConfigHub Examples Without Extra Complexity

If ConfigHub feels overwhelming, use this flow first.

## One Command Verification

```bash
./scripts/verify.sh
```

This checks:

- wrapper script syntax
- bundle shape (`up.yaml` + manifests)
- `cub-up` bundle directories

## Minimal Mental Model

- **Unit**: desired configuration text.
- **Target**: where apply/destroy runs.
- **Worker**: process connected to target operations.
- **Assertions**: explicit state checks after each major command.

## Run Modes

Choose one contract and keep it explicit:

- Human-led: `./scripts/cub-up-human-flow.sh ...`
- AI-led: `./scripts/cub-up-ai-flow.sh ...`
- Human+AI pair: `./scripts/cub-up-pair-flow.sh ...`

## First Demo Command

```bash
CUB_UP_ON_EXISTS=fresh CUB_UP_STALE_ACTION=fresh \
./scripts/cub-up-ai-flow.sh app ./incubator/cub-up/global-app dev <existing-target>
```

Why this default:

- avoids stale/reused slugs in demos
- keeps assertions and GUI checkpoints visible
- reduces confusion about old/disconnected state
