# ConfigHub Examples

This repo contains runnable examples for ConfigHub.

## Start Here

- AI assistants: [`AGENTS.md`](./AGENTS.md)
- AI assistants with more context: [`AI-README-FIRST.md`](./AI-README-FIRST.md)
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

- If you want to explore Campaigns and compliance workflows, start with [`campaigns-demo`](./campaigns-demo/README.md). ([AI guide](./campaigns-demo/AI_START_HERE.md))
- If you want the quickest no-cluster path, start with [`promotion-demo-data`](./promotion-demo-data/README.md). ([AI guide](./promotion-demo-data/AI_START_HERE.md))
- If you want the platform/generator model, start with [`spring-platform`](./spring-platform/). ([AI guide](./spring-platform/springboot-platform-app-centric/AI_START_HERE.md))
- If you want GitOps import, start with [`gitops-import`](./gitops-import/README.md) and the [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/).
- If you want worker extensibility, start with [`custom-workers`](./custom-workers/).
- If you want a classic multi-service example, use [`global-app`](./global-app/README.md).

## Runnable Generator Product Paths

This repo is mostly ConfigHub-first examples. If you want repo-side generator
provenance, governed edits, and runnable product examples, use `cub-gen`:

- Helm: [`cub-gen/examples/helm-paas`](https://github.com/confighub/cub-gen/tree/main/examples/helm-paas) for values ownership, governed ALLOW/BLOCK proof, layered overlay tracing, and connected/live Helm proof
- Score: [`cub-gen/examples/scoredev-paas`](https://github.com/confighub/cub-gen/tree/main/examples/scoredev-paas) for `score.yaml` provenance, workload contract proof, connected evidence, and standalone runtime proof
- Spring Boot: [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) for the Spring ownership and embedded-config product path

## Companion Material

`cub-scout` remains useful as companion material and as a source of comparison fixtures:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
