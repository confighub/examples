# AI Start Here

Use this page as the single AI-oriented handoff page for the current incubator work.

If you need the stricter protocol first, read [AGENTS.md](./AGENTS.md).
If you need the fuller incubator AI guide, read [AI-README-FIRST.md](./AI-README-FIRST.md).

Default rule:

- start in read-only mode
- prefer JSON output
- only mutate ConfigHub when the human asks for that next step

## 0) Prerequisites

```bash
cd <your-examples-checkout>
export CONFIGHUB_AGENT=1
```

## 1) Read-Only First

Start by inspecting the repo without mutating ConfigHub:

```bash
git rev-parse --show-toplevel
./scripts/verify.sh
rg --files incubator
```

If the human wants connected read-only inspection:

```bash
cub auth login
cub space list --json
cub target list --space "*" --json
cd incubator/global-app-layer
./find-runs.sh --json | jq
```

What these commands do not mutate:

- they do not create spaces or units
- they do not write config data
- they do not apply to a cluster

## 2) Stable machine-readable commands

Preferred contracts for AI use:

| Command | Output contract | Mutates anything? |
|---|---|---|
| `cub space list --json` | JSON array of spaces | no |
| `cub target list --space "*" --json` | JSON array of targets | no |
| `cub unit get --space <space> --json <unit>` | JSON object for one unit | no |
| `cub function do --dry-run --json ...` | JSON invocation response | no config write |
| `cub unit apply --dry-run --json ...` | JSON apply preview | no live apply |

If you want to run against a live target, have one ready in your active space.

Quick connected check:

```bash
cub target list --json
```

To discover currently active layered-example runs without knowing the prefix:

```bash
cd incubator/global-app-layer
./find-runs.sh
./find-runs.sh realistic-app --json | jq
```

## 3) Recommended first mutating path

If the human wants the current GitOps import wedge, start with the published docs for the overall story:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)

Then use the runnable incubator examples here.

If the human wants the Argo import path, start here:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
```

That example is for:

- GitHub + Argo + AI/CLI + ConfigHub
- import and evidence
- read-only preview before cluster setup

If the human wants the matching Flux import path, start here:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
```

That example is for:

- GitHub + Flux + AI/CLI + ConfigHub
- import and evidence
- read-only preview before cluster setup

If instead the human wants the layered recipe path rather than the GitOps import wedge, start with the realistic layered app example:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

If you want to wire a real target immediately:

```bash
./setup.sh <prefix> <space/target>
./verify.sh
```

If the human first needs a crisp, read-only app/platform example for authority
vs provenance before any live flow, start here:

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh
```

That example is for:

- one Spring Boot service
- one platform boundary
- three natural mutation routes:
  - `mutable in CH`
  - `lift upstream`
  - `generator-owned`

## 4) Quick demo data (no cluster required)

For exploring ConfigHub's promotion UI without a live target:

```bash
cd promotion-demo-data
./setup.sh
./cleanup.sh
```

For CI/AI verification, see [promotion-demo-data-verify](./promotion-demo-data-verify/).

This creates 49 spaces and ~154 units using the **App-Deployment-Target** model:

- **App** → label + units (e.g., `aichat`, `eshop`, `platform`)
- **Target** → infra space with target object (e.g., `us-prod-1`)
- **Deployment** → `{target}-{app}` space (e.g., `us-prod-1-eshop`)

Uses the noop bridge, so no Kubernetes cluster is needed. This is the canonical multi-env model for ConfigHub.

## 5) Smaller and larger options

If the human wants worker extension patterns, policy checks, or validation workers instead of GitOps import, use the stable worker examples:

- [../custom-workers/hello-world-bridge](../custom-workers/hello-world-bridge/README.md)
- [../custom-workers/hello-world-function](../custom-workers/hello-world-function/README.md)
- [../custom-workers/kube-score](../custom-workers/kube-score/README.md)
- [../custom-workers/kyverno](../custom-workers/kyverno/README.md)
- [../custom-workers/kyverno-server](../custom-workers/kyverno-server/README.md)
- [../custom-workers/opa-gatekeeper](../custom-workers/opa-gatekeeper/README.md)

Smallest:

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh
```

GPU-flavored:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

## 6) What success looks like

You should be able to see:

- the setup plan before any mutation
- explicit spaces and units created
- variant-chain structure preserved
- recipe manifest materialized
- verification passing against the created ConfigHub objects

## 7) Tiny direct vs delegated fixtures

If you need the smallest possible direct and delegated apply inputs for design work around `cub-proc`, use:

- [cub-proc-fixtures](./cub-proc-fixtures/README.md)

These are preserved reference fixtures, not the main walkthrough.

## 8) Public `cub-proc` design work

If the human wants the current public design work for bounded procedures and `Operation` records, use:

- [cub-proc](./cub-proc/README.md)

That directory keeps the design work next to the runnable examples it is based on:

- proposed `Operation` model
- procedure candidates grounded in real examples
- the `promotion-demo-data` evidence note

Do not make the current examples depend on `cub-proc`. Use those docs to understand the next layer, not to block current runnable paths.

If the human is asking how to think about real clusters, workers, and targets from scratch, use:

- [vmcluster-from-scratch](./vmcluster-from-scratch.md)
- [vmcluster-nginx-path](./vmcluster-nginx-path.md)

That note uses the simplest user-facing order:

- boot cluster
- wait for target
- deploy workloads

Use the worker as troubleshooting or implementation detail unless the human is explicitly working on worker and target UX.

If the human wants the smallest live deployment after that bootstrap, use:

- [vmcluster-nginx-path](./vmcluster-nginx-path.md)

That keeps the order simple:

- bootstrap cluster
- wait for target
- bind one small workload
- apply and verify

For the actual runnable implementation of those `vmcluster` flows, point to:

- [jesperfj/cub-vmcluster](https://github.com/jesperfj/cub-vmcluster)

The incubator pages here are for mental model, integration with ConfigHub examples, and future `cub-proc` shaping.

## 9) Related Pages

- Repo-level AI path: [../AI_START_HERE.md](../AI_START_HERE.md)
- Published GitOps import docs: [docs.confighub.com/get-started/examples/gitops-import/](https://docs.confighub.com/get-started/examples/gitops-import/)
- Incubator AI protocol: [AGENTS.md](./AGENTS.md)
- Incubator AI guide: [AI-README-FIRST.md](./AI-README-FIRST.md)
- Reusable example playbook: [ai-example-playbook.md](./ai-example-playbook.md)
- Reusable example template: [ai-example-template.md](./ai-example-template.md)
- Start guide: [global-app-layer/README.md](./global-app-layer/README.md)
- How it works: [global-app-layer/how-it-works.md](./global-app-layer/how-it-works.md)
