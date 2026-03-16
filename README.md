# ConfigHub Examples

This repo contains examples that demonstrate how [ConfigHub](https://confighub.com) works in multiple scenarios.

## Examples Catalog

- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component setup.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.
- [`incubator`](./incubator/README.md): experimental examples, especially the recipes-and-layers work and small `cub run` seed fixtures.

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
