# `cub-up` DRAFT / EXPERIMENTAL

These bundles are for a `cub-up` flow and are intentionally small.  Likely to be deprecated when we have a cub plugin model.

## Bundles

- `global-app`: application workload example.
- `argocd-guestbook`: ArgoCD platform-style example.

## Run Directly

```bash
# App bundle
cub-up app ./cub-up/global-app --env staging --target <existing-target> --assert --open-ui

# Platform bundle
cub-up platform ./cub-up/argocd-guestbook --env dev --target <existing-target> --assert --open-ui
```

## Run Modes

```bash
# Human-led
./scripts/cub-up-human-flow.sh app ./cub-up/global-app staging <existing-target>

# AI-led
./scripts/cub-up-ai-flow.sh app ./cub-up/global-app staging <existing-target>

# Human + AI pair mode
./scripts/cub-up-pair-flow.sh app ./cub-up/global-app staging <existing-target>
```

## Notes

- `--target` can override targets for all units in the bundle.
- `--preflight` should be enabled to show wiring/status before mutation.
- Use `--assert` to stop on first failed state check.
- Use `--open-ui` for GUI checkpoints.
- Use `--on-exists` and stale flags to control reuse:
  - `--on-exists reuse|fresh|fail|prompt`
  - `--stale-after 24h`
  - `--stale-action warn|fresh|fail|prompt`
