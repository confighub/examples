# ConfigHub Examples

This repo contains examples that demonstrate how [ConfigHub](https://confighub.com) works in multiple scenarios.

## Start Here

- For humans: [START_HERE.md](./START_HERE.md)
- For AI assistants (short protocol): [AGENTS.md](./AGENTS.md)
- For AI assistants: [AI-README-FIRST.md](./AI-README-FIRST.md)

## Examples Catalog

- [`incubator/connect-and-compare`](./incubator/connect-and-compare/README.md): incubator no-cluster evidence and compare example.
- [`incubator/import-from-live`](./incubator/import-from-live/README.md): incubator brownfield discovery and dry-run import proposal example.
- [`incubator/import-from-bundle`](./incubator/import-from-bundle/README.md): incubator offline dry-run import proposal example backed by a copied debug bundle.
- [`incubator/connected-summary-storage`](./incubator/connected-summary-storage/README.md): incubator no-cluster reporting example showing stored connected summaries and dry-run Slack digest generation.
- [`incubator/artifact-workflow`](./incubator/artifact-workflow/README.md): incubator offline bundle inspection, replay, and summarize example using a copied debug bundle.
- [`incubator/apptique-flux-monorepo`](./incubator/apptique-flux-monorepo/README.md): incubator app-style Flux monorepo example with one base plus dev and prod overlays.
- [`incubator/apptique-argo-applicationset`](./incubator/apptique-argo-applicationset/README.md): incubator Argo ApplicationSet example with generated apps per environment.
- [`incubator/graph-export`](./incubator/graph-export/README.md): incubator live topology export example producing JSON, DOT, SVG, and HTML artifacts.
- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`custom-workers`](./custom-workers): worker extension examples using the ConfigHub SDK as normal Go modules.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component setup.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.
- [`incubator`](./incubator/README.md): experimental examples, including the current Argo and Flux import paths plus the recipes-and-layers work.

## Current Wedge

For the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, start here:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- [`connect-and-compare`](./incubator/connect-and-compare/README.md)
- [`import-from-live`](./incubator/import-from-live/README.md)
- [`import-from-bundle`](./incubator/import-from-bundle/README.md)
- [`connected-summary-storage`](./incubator/connected-summary-storage/README.md)
- [`artifact-workflow`](./incubator/artifact-workflow/README.md)
- [`apptique-flux-monorepo`](./incubator/apptique-flux-monorepo/README.md)
- [`apptique-argo-applicationset`](./incubator/apptique-argo-applicationset/README.md)
- [`graph-export`](./incubator/graph-export/README.md)
- [`incubator/fleet-import`](./incubator/fleet-import/README.md)
- [`incubator/demo-data-adt`](./incubator/demo-data-adt/README.md)
- [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md)
- [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md)
- [`custom-workers`](./custom-workers)

These are the runnable examples for evidence, reporting, brownfield discovery, app-style GitOps layout, offline import, live import, and worker extensibility in this repo today.

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
