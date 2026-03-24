# ConfigHub Examples

This repo contains examples that demonstrate how [ConfigHub](https://confighub.com) works in multiple scenarios.

## Start Here

- For humans: [START_HERE.md](./START_HERE.md)
- For AI assistants (short protocol): [AGENTS.md](./AGENTS.md)
- For AI assistants: [AI-README-FIRST.md](./AI-README-FIRST.md)

## Examples Catalog

- [`connect-and-compare`](./connect-and-compare/README.md): stable no-cluster evidence and compare example.
- [`import-from-live`](./import-from-live/README.md): stable brownfield discovery and dry-run import proposal example.
- [`apptique-flux-monorepo`](./apptique-flux-monorepo/README.md): stable app-style Flux monorepo example with one base plus dev and prod overlays.
- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`custom-workers`](./custom-workers): worker extension examples using the ConfigHub SDK as normal Go modules.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component setup.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.
- [`incubator`](./incubator/README.md): experimental examples, including the current Argo and Flux import paths plus the recipes-and-layers work.

## Current Wedge

For the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, start here:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- [`connect-and-compare`](./connect-and-compare/README.md)
- [`import-from-live`](./import-from-live/README.md)
- [`apptique-flux-monorepo`](./apptique-flux-monorepo/README.md)
- [`incubator/import-from-bundle`](./incubator/import-from-bundle/README.md)
- [`incubator/fleet-import`](./incubator/fleet-import/README.md)
- [`incubator/demo-data-adt`](./incubator/demo-data-adt/README.md)
- [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md)
- [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md)
- [`custom-workers`](./custom-workers)

These are the runnable examples for evidence, brownfield discovery, app-style GitOps layout, offline import, live import, and worker extensibility in this repo today.

## Companion Material

`cub-scout` remains useful as companion material and as a source of example fixtures, especially for deeper app-style comparisons:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice app styles](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)

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
