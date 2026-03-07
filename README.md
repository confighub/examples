# ConfigHub Examples

This repo contains examples that demonstrate how [ConfigHub](https://confighub.com) works in multiple scenarios.

## Fast Entry

- AI-led landing page: [CLAUDE_LANDING.md](./CLAUDE_LANDING.md)
- Start guide: [START_HERE.md](./START_HERE.md)
- Persona quickstart: [PERSONA_QUICKSTART.md](./PERSONA_QUICKSTART.md)

## Examples Catalog

- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component setup.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.
- [`cub-up`](./cub-up/README.md): one-command app/platform bundles with assert + GUI checkpoints.
- [`incubator`](./incubator/README.md): experimental flows before promotion.

## Prerequisites

```bash
cub auth login
```

For `cub-up` flows, use either `cub-up` binary or a `cub` build that supports `cub up`.

## Run Checks

```bash
./scripts/verify.sh
```

## Run Modes for `cub-up` Scenarios

```bash
# Human-led
./scripts/cub-up-human-flow.sh app ./cub-up/global-app dev <existing-target>

# AI-led
./scripts/cub-up-ai-flow.sh app ./cub-up/global-app dev <existing-target>

# Human + AI
./scripts/cub-up-pair-flow.sh app ./cub-up/global-app dev <existing-target>
```

## General Script Behavior

Scripts are designed to be additive and explicit. Read scripts before running them in shared environments.
