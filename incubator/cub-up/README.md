# `cub-up` Incubator Bundles

These bundles are the experimental lane for `cub-up`.

Stable examples remain in [`/cub-up`](../../cub-up/README.md).

## Bundles

- `global-app`: application workload example.
- `argocd-guestbook`: ArgoCD platform-style example.

## Run

```bash
# App bundle
./scripts/cub-up-human-flow.sh app ./incubator/cub-up/global-app staging <existing-target>

# Platform bundle
./scripts/cub-up-human-flow.sh platform ./incubator/cub-up/argocd-guestbook dev <existing-target>
```

## Notes

- `--preflight` should be enabled by default.
- Use `--assert` and GUI checkpoints.
- Prefer `--on-exists fresh` for demo runs.
