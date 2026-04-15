# ConfigHub Examples

This repo contains runnable examples for ConfigHub.

## Where To Start

- Quickest no-cluster path: [`promotion-demo-data`](./promotion-demo-data/README.md)
- GitOps import, compare, and experimental paths: [`incubator/README.md`](./incubator/README.md)
- App mutation and platform flow: [`spring-platform/springboot-platform-app-centric`](./spring-platform/springboot-platform-app-centric/README.md)
- Worker extensibility: [`custom-workers`](./custom-workers/)
- Example contract standard: [`EXAMPLE_CONTRACT_STANDARD.md`](./EXAMPLE_CONTRACT_STANDARD.md)

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

- [`campaigns-demo`](./campaigns-demo/README.md): 10 compliance campaigns backed by Kyverno CEL policies, with sample Kubernetes units to evaluate.
- [`promotion-demo-data`](./promotion-demo-data/README.md): no-cluster demo data for learning ConfigHub's App-Deployment-Target model and promotion flow.
- [`gitops-import`](./gitops-import/README.md): canonical Argo CD GitOps import example and docs companion.
- [`custom-workers`](./custom-workers/): worker extension examples, including bridge, function, and policy workers.
- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component example for Helm-managed infrastructure.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.

## Recommended Starting Points

- If you want to explore Campaigns and compliance workflows, start with [`campaigns-demo`](./campaigns-demo/README.md).
- If you want the quickest no-cluster path, start with [`promotion-demo-data`](./promotion-demo-data/README.md).
- If you want the platform/generator model, start with [`spring-platform`](./spring-platform/).
- If you want GitOps import, start with [`gitops-import`](./gitops-import/README.md) and the [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/).
- If you want worker extensibility, start with [`custom-workers`](./custom-workers/).
- If you want a classic multi-service example, use [`global-app`](./global-app/README.md).

## Companion Material

`cub-scout` remains useful as companion material and as a source of comparison fixtures:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
