# ConfigHub Examples

Runnable examples for ConfigHub.

## Where To Start

- Quickest no-cluster intro: [promotion-demo-data](./promotion-demo-data/README.md)
- GitOps import, compare, and experimental flows: [incubator/README.md](./incubator/README.md)
- App mutation and platform flow: [spring-platform/springboot-platform-app-centric](./spring-platform/springboot-platform-app-centric/README.md)
- Worker extensibility: [custom-workers](./custom-workers/)
- Example authoring contract: [EXAMPLE_CONTRACT_STANDARD.md](./EXAMPLE_CONTRACT_STANDARD.md)

## Safe First Checks

For a read-only first pass:

```bash
./scripts/verify.sh
cub context list --json
cub space list --json
cub target list --space "*" --json
```

If you are not logged in yet, run `cub auth login` before the `cub` commands.

## Stable Examples

- [campaigns-demo](./campaigns-demo/README.md): compliance campaigns backed by Kyverno CEL policies, with sample Kubernetes units to evaluate
- [promotion-demo-data](./promotion-demo-data/README.md): no-cluster demo data for learning ConfigHub's App-Deployment-Target model and promotion flow
- [gitops-import](./gitops-import/README.md): canonical Argo CD GitOps import example and docs companion
- [custom-workers](./custom-workers/): worker extension examples, including bridge, function, and policy workers
- [global-app](./global-app/README.md): classic multi-service app example
- [helm-platform-components](./helm-platform-components/README.md): platform component example for Helm-managed infrastructure
- [vm-fleet](./vm-fleet/README.md): VM fleet operations example

## Good First Choices

- Want the simplest ConfigHub model first: [promotion-demo-data](./promotion-demo-data/README.md)
- Want compare, import, or app-layout work: [incubator/README.md](./incubator/README.md)
- Want a live app-centric mutation story: [springboot-platform-app-centric](./spring-platform/springboot-platform-app-centric/README.md)
- Want a classic multi-service example: [global-app](./global-app/README.md)

## Companion Material

`cub-scout` remains useful as companion material and as a source of comparison fixtures:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
