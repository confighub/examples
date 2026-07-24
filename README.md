# ConfigHub Examples

This repo contains runnable examples for ConfigHub.

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

- [`promotion-demo-data`](./promotion-demo-data/README.md): quickest no-cluster demo data for learning ConfigHub's App-Deployment-Target model and promotion flow.
- [`gitops-import`](./gitops-import/README.md): canonical Argo CD GitOps import example and docs companion.
- [`initiatives-demo`](./initiatives-demo/README.md): 5 compliance initiatives backed by Kyverno CEL policies, with sample Kubernetes units to evaluate.
- [`custom-workers`](./custom-workers/): worker extension examples, including bridge, function, and policy workers.
- [`pilot-example-addons-manager`](./pilot-example-addons-manager/README.md): standalone generated operational app with Variant-first GUI, CLI sibling, production ConfigHub browser OAuth registration, and live-binding proof gates.
- [`cost-management-app`](./cost-management-app/README.md): generated operational app with a real cost engine: org-wide waste findings priced from config data, then a finding-owned dry run, short-lived exact review, explicit execution confirmation, revision-verified mutation, and receipt. The reduction plane beside [`cost-estimator`](./cost-estimator/README.md)'s enforcement plane.
- [`global-app`](./global-app/README.md): classic multi-service app example.
- [`helm-platform-components`](./helm-platform-components/README.md): platform component example for Helm-managed infrastructure.
- [`vm-fleet`](./vm-fleet/README.md): VM fleet operations example.

## Recommended Starting Points

- If you want to explore Initiatives and compliance workflows, start with [`initiatives-demo`](./initiatives-demo/README.md).
- If you want the quickest no-cluster path, start with [`promotion-demo-data`](./promotion-demo-data/README.md).
- If you want to understand Generators as functions on config data, start with
  [`spring-platform`](./spring-platform/), then move to
  [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas)
  for the product path, including the Spring ConfigHub Initiative GUI proof.
- If you want GitOps import, start with [`gitops-import`](./gitops-import/README.md) and the [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/).
- If you want worker extensibility, start with [`custom-workers`](./custom-workers/).
- If you want a classic multi-service example, use [`global-app`](./global-app/README.md).

Note on contract standard: [`EXAMPLE_CONTRACT_STANDARD.md`](./EXAMPLE_CONTRACT_STANDARD.md)

## Companion Material

Some other examples

- Fleet guardrails that analyze config, record a verdict as data, and gate apply
  on it: [`sec-scanner`](./sec-scanner/README.md) (container CVEs),
  [`rbac-manager`](./rbac-manager/README.md) (RBAC hygiene), and
  [`cost-estimator`](./cost-estimator/README.md) (workload cloud cost); the
  cost reduction counterpart is
  [`cost-management-app`](./cost-management-app/README.md).
- Agentic RBAC companion patterns:
  [`rbac-manager-for-agents`](./rbac-manager-for-agents/README.md),
  [`rbac-manager-over-redis`](./rbac-manager-over-redis/README.md), and
  [`redis-platform-with-rbac-guardrails`](./redis-platform-with-rbac-guardrails/README.md)
- Agent-driven fleet managers — each a `cub-*` CLI that manages one domain of
  Kubernetes config as data across a fleet of cluster-Spaces, siblings of
  [`rbac-manager-for-agents`](./rbac-manager-for-agents/README.md):
  [`workload-manager`](./workload-manager/README.md) (workload security and
  reliability posture: security context, resources, probes, PDBs),
  [`namespace-manager`](./namespace-manager/README.md) (namespaces and their
  policy envelope: pod-security labels, default-deny NetworkPolicy, baseline
  RBAC),
  [`network-policy-manager`](./network-policy-manager/README.md) (NetworkPolicy,
  reasoned about with the Namespaces, workloads, and Services it covers),
  [`scheduling-manager`](./scheduling-manager/README.md) (workload placement:
  `nodeSelector`, tolerations, node affinity),
  [`autoscale-manager`](./autoscale-manager/README.md) (autoscaling:
  HorizontalPodAutoscalers and KEDA ScaledObjects),
  [`observability-manager`](./observability-manager/README.md) (observability
  posture: Prometheus ServiceMonitor coverage and telemetry sidecar injection),
  and [`eks-manager`](./eks-manager/README.md) (AWS EKS clusters as Crossplane
  managed resources)
- Platform view builders — read the config data in a Space and project it into a
  different platform representation:
  [`k8s-to-score`](./k8s-to-score/README.md) reads the Kubernetes resources in a
  Space and emits a [Score](https://score.dev) workload spec per Deployment or
  StatefulSet (the inverse of `score-k8s`; read-only)
- Incubator and experimental paths: [`incubator/README.md`](./incubator/README.md)
- App mutation and platform flow: [`spring-platform/springboot-platform-app-centric`](./spring-platform/springboot-platform-app-centric/README.md)
- Standalone operational app shape: [`pilot-example-addons-manager`](./pilot-example-addons-manager/README.md)

`cub-scout` remains useful as companion material and as a source of comparison fixtures:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
