# ConfigHub Examples

This repo contains runnable examples for ConfigHub.

Stable examples live at the repo root. Experimental and AI-first examples stay in [`incubator/`](./incubator/README.md) until they are ready to promote.

## Start Here

- Humans: [`START_HERE.md`](./START_HERE.md)
- AI assistants: [`AGENTS.md`](./AGENTS.md)
- AI assistants with more context: [`AI-README-FIRST.md`](./AI-README-FIRST.md)
- AI demo pacing and incubator path: [`AI_START_HERE.md`](./AI_START_HERE.md)

## Safe First Checks

For a read-only first pass:

```bash
./scripts/verify.sh
cub context list --json
cub space list --json
cub target list --space "*" --json
```

If you are not logged in yet, run `cub auth login` before the `cub` commands.

## Stable Paths

- [`promotion-demo-data`](./promotion-demo-data/README.md): no-cluster demo data for learning ConfigHub's App-Deployment-Target model and promotion flow.
- [`gitops-import`](./gitops-import/README.md): canonical Argo CD GitOps import example and docs companion.
- [`custom-workers`](./custom-workers/): worker extension examples, including bridge, function, and policy workers.
- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component example for Helm-managed infrastructure.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.

## Incubator Paths

Use [`incubator/README.md`](./incubator/README.md) for the full catalog. The main tracks are:

- No-cluster evidence and offline analysis: [`connect-and-compare`](./incubator/connect-and-compare/README.md), [`import-from-bundle`](./incubator/import-from-bundle/README.md), [`connected-summary-storage`](./incubator/connected-summary-storage/README.md), [`artifact-workflow`](./incubator/artifact-workflow/README.md), [`graph-export`](./incubator/graph-export/README.md), [`fleet-import`](./incubator/fleet-import/README.md), [`demo-data-adt`](./incubator/demo-data-adt/README.md), and [`lifecycle-hazards`](./incubator/lifecycle-hazards/README.md).
- Live discovery and GitOps comparison: [`import-from-live`](./incubator/import-from-live/README.md), [`gitops-import-argo`](./incubator/gitops-import-argo/README.md), [`gitops-import-flux`](./incubator/gitops-import-flux/README.md), [`combined-git-live`](./incubator/combined-git-live/README.md), [`custom-ownership-detectors`](./incubator/custom-ownership-detectors/README.md), [`orphans`](./incubator/orphans/README.md), [`watch-webhook`](./incubator/watch-webhook/README.md), [`flux-boutique`](./incubator/flux-boutique/README.md), and [`platform-example`](./incubator/platform-example/README.md).
- App-style GitOps layouts: [`apptique-flux-monorepo`](./incubator/apptique-flux-monorepo/README.md), [`apptique-argo-applicationset`](./incubator/apptique-argo-applicationset/README.md), and [`apptique-argo-app-of-apps`](./incubator/apptique-argo-app-of-apps/README.md).
- Model and operational experiments: [`springboot-platform-app`](./incubator/springboot-platform-app/README.md), [`platform-write-api`](./incubator/platform-write-api/README.md), [`global-app-layer`](./incubator/global-app-layer/README.md), [`promotion-demo-data-verify`](./incubator/promotion-demo-data-verify/README.md), and [`cub-proc`](./incubator/cub-proc/README.md).

## Recommended Starting Points

- If you want the quickest no-cluster path, start with [`promotion-demo-data`](./promotion-demo-data/README.md) and the no-cluster section in [`START_HERE.md`](./START_HERE.md).
- If you want the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, start with [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/) and then use the import examples under [`incubator/`](./incubator/README.md).
- If you want worker extensibility, start with [`custom-workers`](./custom-workers/).
- If you want the deeper layered recipe model, use [`incubator/global-app-layer`](./incubator/global-app-layer/README.md) after the simpler examples above.

## Companion Material

`cub-scout` remains useful as companion material and as a source of comparison fixtures:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
