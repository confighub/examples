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
- [`initiatives-demo`](./initiatives-demo/README.md): 5 compliance initiatives backed by Kyverno CEL policies, with sample Kubernetes units to evaluate.
- [`custom-workers`](./custom-workers/): worker extension examples, including bridge, function, and policy workers.
- [`pilot-example-addons-manager`](./pilot-example-addons-manager/README.md): standalone generated operational app with Variant-first GUI, CLI sibling, production ConfigHub browser OAuth registration, and live-binding proof gates.
- [`cost-management-app`](./cost-management-app/README.md): generated operational app with a real cost engine: org-wide waste findings priced from config data, then a finding-owned dry run, short-lived exact review, explicit execution confirmation, revision-verified mutation, and receipt. The reduction plane beside [`cost-estimator`](./cost-estimator/README.md)'s enforcement plane.
- [`configboard`](./configboard/README.md): BI-style dashboards over the configuration in your organization — what version of what is where, which guardrails are failing, how many resources you manage, how long changes take to land. Read-only and seeds nothing. Dashboards are `AppConfig/YAML` units, so they carry revision history and promote like any other config, and every panel prints its equivalent `cub` command. Includes a 3-minute silent [demo video](./configboard/demo/configboard-demo.mp4) and [transcript](./configboard/demo/TRANSCRIPT.md).

## Recommended Starting Points

- If you want to explore Initiatives and compliance workflows, start with [`initiatives-demo`](./initiatives-demo/README.md).
- If you want the quickest no-cluster path, start with [`promotion-demo-data`](./promotion-demo-data/README.md).
- If you want to understand Generators as functions on config data, start with
  [`spring-platform`](./spring-platform/), then move to
  [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas)
  for the product path, including the Spring ConfigHub Initiative GUI proof.
- If you want to deploy to a cluster, see the [GitOps operators guide](https://docs.confighub.com/guide/gitops/) — configuration is published as an OCI Release that Argo CD or Flux pulls.
- If you want to see the state of an existing organization at a glance, start with
  [`configboard`](./configboard/README.md) — it seeds nothing, and its
  [demo video](./configboard/demo/configboard-demo.mp4) covers the six bundled
  dashboards in three minutes.
- If you want worker extensibility, start with [`custom-workers`](./custom-workers/).

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
- Shared libraries the examples are built on, rather than examples themselves:
  [`webkit`](./webkit/README.md) — the auth shell, fleet scope and snapshot,
  config-data access, and RBAC engine behind the web consoles
  ([`configboard`](./configboard/README.md),
  [`promoter`](./promoter/README.md),
  [`rbac-manager`](./rbac-manager/README.md),
  [`sec-scanner`](./sec-scanner/README.md),
  [`fleet-ql`](./fleet-ql/README.md), and
  [`cost-estimator`](./cost-estimator/README.md)) — and
  [`managerkit`](./managerkit), the Go equivalent for the `cub-*` fleet managers.
  The ConfigHub API client and auth flow themselves are not here: those are the
  published [`@confighub/api`](https://www.npmjs.com/package/@confighub/api),
  [`@confighub/rtk-query`](https://www.npmjs.com/package/@confighub/rtk-query), and
  [`@confighub/react-auth`](https://www.npmjs.com/package/@confighub/react-auth)
  packages from [confighub/js-sdk](https://github.com/confighub/js-sdk).
- Incubator and experimental paths: [`incubator/README.md`](./incubator/README.md)
- App mutation and platform flow: [`spring-platform/springboot-platform-app-centric`](./spring-platform/springboot-platform-app-centric/README.md)
- Standalone operational app shape: [`pilot-example-addons-manager`](./pilot-example-addons-manager/README.md)

`cub-scout` remains useful as companion material and as a source of comparison fixtures:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
