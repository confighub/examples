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

If the human wants an offline import path with no cluster access, start here:

```bash
cd incubator/import-from-bundle
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- dry-run import proposal generation
- bundle-backed evidence
- no live cluster requirement

If the human wants the next step after that, reading directly from a running cluster before any ConfigHub mutation, start here:

```bash
cd incubator/import-from-live
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- brownfield discovery
- dry-run proposal generation from live state
- mixed Argo, Helm, and native ownership signals
- no default ConfigHub mutation

If the human wants multi-cluster aggregation from two existing cluster imports, start here:

```bash
cd incubator/fleet-import
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- multi-cluster aggregation
- one unified fleet proposal
- no live cluster requirement

If the human wants a scan-first example with App-Deployment-Target-style labels and one immediate real issue, start here:

```bash
cd incubator/demo-data-adt
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- static risk findings
- ADT-style labels and annotations
- no live cluster requirement

If the human wants a no-cluster migration-risk example for Helm hooks under Argo CD, start here:

```bash
cd incubator/lifecycle-hazards
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- hook inventory from one file
- Helm-to-Argo lifecycle hazard detection
- no live cluster requirement

If the human wants the smallest no-cluster evidence path first, start here:

```bash
cd incubator/connect-and-compare
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- standalone signal
- compare output
- history output
- no live cluster requirement

If the human wants the next compare step with a real live cluster, start here:

```bash
cd incubator/combined-git-live
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- Git intent versus live cluster state
- aligned, git-only, and cluster-only findings
- no ConfigHub mutation

If the human wants a small platform-team ownership example rather than an import flow, start here:

```bash
cd incubator/custom-ownership-detectors
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- custom ownership detection from YAML
- live `map`, `explain`, and `trace` evidence
- no ConfigHub mutation

If the human wants shareable topology artifacts from a live cluster, start here:

```bash
cd incubator/graph-export
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

That example is for:

- `graph.v1` JSON export
- DOT, SVG, and HTML renderings
- no ConfigHub mutation

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

## 9) App-Style Flux Layout

If the human wants one concrete app-style example that is not mainly about import, use:

- [apptique-flux-monorepo](./apptique-flux-monorepo/README.md)

This example is good for:

- one app base plus dev and prod overlays
- Flux Kustomization per environment
- ownership and provenance checks with `kubectl`, `flux`, and optional `cub-scout`

It does not mutate ConfigHub by itself. If the human wants to bring a cluster like that into ConfigHub, follow it with:

- [gitops-import-flux](./gitops-import-flux/README.md)

## 10) Fixture-First Compare

If the human wants compare and history evidence without needing a live cluster, use:

- [connect-and-compare](./connect-and-compare/README.md)

This example is good for:

- a fast demo of visible value
- compare output with aligned, git-only, and cluster-only findings
- history output without claiming runtime authority

If the human wants the next compare example that uses a real cluster, use:

- [import-from-live](./import-from-live/README.md)
- [combined-git-live](./combined-git-live/README.md)

This example is good for:

- Git plus live cluster alignment
- one real cluster-only workload
- one real Git-only app
- the same evidence-first compare model against live state

## 11) App-Style Argo Layout

If the human wants the matching Argo app-style example, use:

- [apptique-argo-applicationset](./apptique-argo-applicationset/README.md)

This example is good for:

- one ApplicationSet generator
- one generated Application per environment
- ownership and provenance checks with `kubectl` and optional `cub-scout`

It does not mutate ConfigHub by itself. If the human wants to bring a cluster like that into ConfigHub, follow it with:

- [gitops-import-argo](./gitops-import-argo/README.md)

If the human specifically wants the root-plus-child Argo hierarchy pattern, use:

- [apptique-argo-app-of-apps](./apptique-argo-app-of-apps/README.md)

That example is good for:

- one root Application
- one child Application per environment
- ownership and provenance checks across the Argo hierarchy

## 12) Related Pages

- Repo-level AI path: [../AI_START_HERE.md](../AI_START_HERE.md)
- Published GitOps import docs: [docs.confighub.com/get-started/examples/gitops-import/](https://docs.confighub.com/get-started/examples/gitops-import/)
- Incubator AI protocol: [AGENTS.md](./AGENTS.md)
- Incubator AI guide: [AI-README-FIRST.md](./AI-README-FIRST.md)
- Reusable example playbook: [ai-example-playbook.md](./ai-example-playbook.md)
- Reusable example template: [ai-example-template.md](./ai-example-template.md)
- Start guide: [global-app-layer/README.md](./global-app-layer/README.md)
- How it works: [global-app-layer/how-it-works.md](./global-app-layer/how-it-works.md)
