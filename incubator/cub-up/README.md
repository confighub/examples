# `cub-up` Bundles

One-command app/platform bundles with assert + GUI checkpoints.

## Bundles

- `global-app`: application workload example.
- `argocd-guestbook`: ArgoCD platform-style example.

## Run Directly

```bash
# App bundle
cub-up app ./incubator/cub-up/global-app --env staging --target <existing-target> --assert --open-ui

# Platform bundle
cub-up platform ./incubator/cub-up/argocd-guestbook --env dev --target <existing-target> --assert --open-ui
```

## Run Modes

```bash
# Human-led
./scripts/cub-up-human-flow.sh app ./incubator/cub-up/global-app staging <existing-target>

# AI-led
./scripts/cub-up-ai-flow.sh app ./incubator/cub-up/global-app staging <existing-target>

# Human + AI pair mode
./scripts/cub-up-pair-flow.sh app ./incubator/cub-up/global-app staging <existing-target>
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
