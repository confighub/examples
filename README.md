# ConfigHub Examples

This repo contains examples that demonstrate how [ConfigHub](https://confighub.com) works in multiple scenarios.

## Start Here

- For humans: [START_HERE.md](./START_HERE.md)
- For AI assistants (short protocol): [AGENTS.md](./AGENTS.md)
- For AI assistants: [AI-README-FIRST.md](./AI-README-FIRST.md)

## Examples Catalog

- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component setup.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.
- [`incubator`](./incubator/README.md): experimental examples, especially the recipes-and-layers work and small `cub run` seed fixtures.

## Companion Demos

For the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, the best import and evidence demos live in the companion repo:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Argo import demo](https://github.com/confighub/cub-scout/tree/main/examples/argo-import-confighub-demo)
- [Flux import demo](https://github.com/confighub/cub-scout/tree/main/examples/flux-import-confighub-demo)
- [Apptique microservice app styles](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)

Use this repo for the ConfigHub-side examples and layer/deployment-unit stories.

## Prerequisites

```bash
cub auth login
```

## Run Checks

```bash
./scripts/verify.sh
```

## General Script Behavior

Scripts are designed to be additive and explicit. Read scripts before running them in shared environments.
