# Examples Incubator

This directory contains experimental examples for ConfigHub. Start by picking your reason.

## Why ConfigHub?

ConfigHub is a management layer for operational configuration. It imports, validates, mutates, and delivers config through governed workflows. See [WHY_CONFIGHUB.md](./WHY_CONFIGHUB.md) for the full explanation.

## Choose By Reason

| Reason | What you want | Start here |
|--------|---------------|------------|
| **Import** | See what you already have in Git, clusters, or controllers | [gitops-import-argo](./gitops-import-argo/README.md) or [gitops-import-flux](./gitops-import-flux/README.md) |
| **Mutate** | Make controlled changes through a governed write API | [platform-write-api](./platform-write-api/README.md) |
| **Apply** | Deploy real workloads through real targets | [springboot-platform-app-centric](./springboot-platform-app-centric/README.md) (start here) or [global-app-layer/single-component](./global-app-layer/single-component/README.md) |
| **Model** | Represent layered or governed config structures | [global-app-layer](./global-app-layer/README.md) |

## Delivery Matrix

ConfigHub supports multiple delivery modes. Know which one you need:

| Delivery Mode | Description | Status |
|---------------|-------------|--------|
| **Direct Kubernetes** | Worker applies YAML via `kubectl apply` | Simplest real proof. Fully working. |
| **Flux OCI** | Worker publishes OCI artifact, Flux reconciles | Current standard controller path. |
| **Argo OCI** | Worker publishes to ConfigHub-native OCI origin, Argo reconciles | Implemented. Claim it only when controller and live evidence are shown. |
| **Renderer-only** | Worker sends payloads to a renderer (e.g., `ArgoCDRenderer`) | Companion path. Not the same as OCI delivery. |

For controller-oriented delivery: **Flux OCI** is the current standard. **Argo OCI** is implemented, but it still needs the same controller and live evidence discipline.

`ArgoCDRenderer` is a valid renderer path but should not be confused with Argo OCI delivery. See [global-app-layer/contracts.md](./global-app-layer/contracts.md) for payload compatibility.

## Entry Paths

- For humans: [`../START_HERE.md`](../START_HERE.md)
- Why ConfigHub: [`WHY_CONFIGHUB.md`](./WHY_CONFIGHUB.md)
- For AI assistants: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Incubator AI protocol: [`AGENTS.md`](./AGENTS.md)
- Fuller incubator AI guide: [`AI-README-FIRST.md`](./AI-README-FIRST.md)
- Planning and milestones: [`planning/README.md`](./planning/README.md)

## Reality Rules

- "End-to-end" means every step is real: ConfigHub stores the config, the mutation is real, apply uses a non-`Noop` target, a real app or controller receives the change, and verification checks live behavior.
- `Noop` targets are opt-in only. If an example uses `Noop`, it must say so explicitly and it does not count as 100% real end-to-end.
- Some examples are still real live examples without being ConfigHub-apply examples. Those are valid, but they must be labeled honestly as import, evidence, or controller-layout demos.

## Example Reality Guide

- `100% real e2e through ConfigHub apply`: [`global-app-layer/single-component`](./global-app-layer/single-component/README.md), [`global-app-layer/frontend-postgres`](./global-app-layer/frontend-postgres/README.md), and [`global-app-layer/realistic-app`](./global-app-layer/realistic-app/README.md) when you bind them to a real non-`Noop` target and verify the deployed result on the cluster.
- `Both options`: [`global-app-layer`](./global-app-layer/README.md) is ConfigHub-first and read-only-first by default, but its worked examples can continue to a real live apply path. [`global-app-layer/gpu-eks-h100-training`](./global-app-layer/gpu-eks-h100-training/README.md) is the clearest "both options" example because it can stay structural or continue into real direct and `fluxoci` delivery branches. [`springboot-platform-app-centric`](./springboot-platform-app-centric/README.md) (and its underlying [`springboot-platform-app`](./springboot-platform-app/README.md)) supports both real Kubernetes deployment (`--with-targets`) and Noop simulation (default).
- `Real live, but not ConfigHub apply`: [`import-from-live`](./import-from-live/README.md), [`graph-export`](./graph-export/README.md), [`combined-git-live`](./combined-git-live/README.md), [`gitops-import-argo`](./gitops-import-argo/README.md), [`gitops-import-flux`](./gitops-import-flux/README.md), [`custom-ownership-detectors`](./custom-ownership-detectors/README.md), [`orphans`](./orphans/README.md), [`watch-webhook`](./watch-webhook/README.md), [`flux-boutique`](./flux-boutique/README.md), [`platform-example`](./platform-example/README.md), [`apptique-flux-monorepo`](./apptique-flux-monorepo/README.md), [`apptique-argo-applicationset`](./apptique-argo-applicationset/README.md), and [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md).
- `Simulated, offline, or Noop-only`: [`connect-and-compare`](./connect-and-compare/README.md), [`import-from-bundle`](./import-from-bundle/README.md), [`connected-summary-storage`](./connected-summary-storage/README.md), [`artifact-workflow`](./artifact-workflow/README.md), [`fleet-import`](./fleet-import/README.md), [`demo-data-adt`](./demo-data-adt/README.md), [`lifecycle-hazards`](./lifecycle-hazards/README.md), [`platform-write-api`](./platform-write-api/README.md), and [`promotion-demo-data-verify`](./promotion-demo-data-verify/README.md).

One important current example is not real e2e today:

- [`platform-write-api`](./platform-write-api/README.md): real ConfigHub mutation story, but no live delivery; no cluster is touched.

## Where The Argo And Flux Examples Went

If you are looking for older Argo or Flux names, these are the current examples:

| Looking for | Current example |
|---|---|
| Argo import from GitHub | [`gitops-import-argo`](./gitops-import-argo/README.md) |
| Flux import from GitHub, including podinfo and D2 contrast | [`gitops-import-flux`](./gitops-import-flux/README.md) |
| Argo ApplicationSet app-style layout | [`apptique-argo-applicationset`](./apptique-argo-applicationset/README.md) |
| Argo app-of-apps layout | [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md) |
| Flux monorepo app-style layout | [`apptique-flux-monorepo`](./apptique-flux-monorepo/README.md) |
| Flux multi-service fan-out from one `GitRepository` | [`flux-boutique`](./flux-boutique/README.md) |
| Mixed Flux-managed plus native platform ownership | [`platform-example`](./platform-example/README.md) |
| Git plus live comparison against a real cluster | [`combined-git-live`](./combined-git-live/README.md) |

## Standard Stories

If you need one standard Argo story and one standard Flux story, use these:

- `Standard Argo story`: [`gitops-import-argo`](./gitops-import-argo/README.md), centered on the real Argo-synced guestbook applications. Lead with the healthy guestbook path first. Treat the brownfield contrast fixtures as a second-pass follow-on, not the front door.
- `Standard Flux story`: [`gitops-import-flux`](./gitops-import-flux/README.md), centered on the real Flux-managed `podinfo` path. Lead with `podinfo` first. Treat the D2 contrast fixtures as a second-pass follow-on, not the front door.
- `5-10 minute bar`: by minute 10, the human should have one concrete reason to care. That means a healthy controller-owned app they can inspect now, plus a clear ConfigHub discover/import step or imported result. If setup time eats the whole window, the story is not ready as the standard front door.

## Choose By Reason

If the example names are not helping, pick by the reason it exists:

- `I need the clearest Argo import proof`: [`gitops-import-argo`](./gitops-import-argo/README.md)
- `I need the clearest Flux import proof`: [`gitops-import-flux`](./gitops-import-flux/README.md)
- `I need to explain why ConfigHub is a write API for config`: [`platform-write-api`](./platform-write-api/README.md)
- `I need a real app plus real deploy/apply proof`: [`springboot-platform-app-centric`](./springboot-platform-app-centric/README.md) (app-centric front door) or [`springboot-platform-app`](./springboot-platform-app/README.md) (implementation detail)
- `I need the smallest layered recipe walkthrough`: [`global-app-layer/single-component`](./global-app-layer/single-component/README.md)
- `I need a small app-level layered recipe`: [`global-app-layer/frontend-postgres`](./global-app-layer/frontend-postgres/README.md)
- `I need the most realistic layered app in this package`: [`global-app-layer/realistic-app`](./global-app-layer/realistic-app/README.md)
- `I need the NVIDIA-shaped layered stack story`: [`global-app-layer/gpu-eks-h100-training`](./global-app-layer/gpu-eks-h100-training/README.md)
- `I need Git versus live evidence on one cluster`: [`combined-git-live`](./combined-git-live/README.md)
- `I need ownership or orphan evidence`: [`custom-ownership-detectors`](./custom-ownership-detectors/README.md) or [`orphans`](./orphans/README.md)
- `I need bounded-procedure design work, not a runnable front door`: [`cub-proc`](./cub-proc/README.md)

## Current Experiments

### No-Cluster Evidence And Inspection

- [`demo-data-adt`](./demo-data-adt/README.md): scan-first App-Deployment-Target example showing labeled workload fixtures plus immediate static risk findings.
- [`lifecycle-hazards`](./lifecycle-hazards/README.md): migration-risk example adapted from `cub-scout`, showing hook inventory and Helm-to-Argo lifecycle hazards from one manifest file.

### Offline Import And Aggregation

- [`fleet-import`](./fleet-import/README.md): multi-cluster aggregation from two existing cluster import JSONs into one unified proposal.

### Live Import, Comparison, Ownership, Topology, And Orphans

- [`gitops-import-argo`](./gitops-import-argo/README.md): standard Argo story, built around the healthy guestbook import path first, with stronger contrast fixtures adapted from `cub-scout` available later.
- [`gitops-import-flux`](./gitops-import-flux/README.md): standard Flux story, built around the healthy `podinfo` import path first, with D2 contrast fixtures available later.
- [`combined-git-live`](./combined-git-live/README.md): Git-plus-live compare example adapted from `cub-scout`, showing aligned, git-only, and cluster-only results against a real cluster.
- [`custom-ownership-detectors`](./custom-ownership-detectors/README.md): platform-team ownership example adapted from `cub-scout`, showing custom owner names in `map`, `explain`, and `trace` without touching ConfigHub.
- [`orphans`](./orphans/README.md): unmanaged-resource example adapted from `cub-scout`, showing how `Native` resources surface in orphan inventory on a live cluster.
- [`watch-webhook`](./watch-webhook/README.md): event-streaming example adapted from `cub-scout`, showing one-shot `watch --webhook` delivery into a local receiver.
- [`flux-boutique`](./flux-boutique/README.md): Flux microservice fan-out example adapted from `cub-scout`, showing one GitRepository traced through five Kustomizations and services.
- [`platform-example`](./platform-example/README.md): mixed-ownership example adapted from `cub-scout`, showing Flux-managed platform resources beside unmanaged orphan resources.

### App-Style GitOps Layouts

- [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md): Argo app-of-apps layout with one root Application and one child Application per environment.

### Structural And Model Examples

- [`springboot-platform-app-centric`](./springboot-platform-app-centric/README.md): App-centric front door for the Spring Boot mutation story. One app, three deployments, three target modes, three mutation outcomes. Start here.
- [`springboot-platform-app`](./springboot-platform-app/README.md): Spring Boot app/platform implementation detail with one app and three natural mutation routes. Full fixture and proof reference.
- [`promotion-demo-data-verify`](./promotion-demo-data-verify/README.md): verification wrapper for the stable `promotion-demo-data` example.

### Advanced Composition And Deployment Variants

- [`global-app-layer`](./global-app-layer/README.md): recipes and layers package with specs plus worked examples, including a GPU-flavored chain.

### Operational Model And Supporting Design Work

- [`cub-proc`](./cub-proc/README.md): public incubator design work for bounded procedures, `Operation` records, and `cub-proc`, grounded in the runnable examples in this repo.
- [`cub-proc-fixtures`](./cub-proc-fixtures/README.md): tiny direct and delegated apply fixtures preserved from the earlier `cub-up` exploration.
- [`vmcluster-from-scratch`](./vmcluster-from-scratch.md): note for thinking about real cluster bootstrap as cluster first, target second, worker mostly hidden.
- [`vmcluster-nginx-path`](./vmcluster-nginx-path.md): note for the smallest real-cluster follow-on path, from ready target to one reachable workload.

## Where To Start By Goal

If the goal is the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, do not start with `global-app-layer`.

Start with one standard story:
- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- Argo: [`gitops-import-argo`](./gitops-import-argo/README.md) with the guestbook path first
- Flux: [`gitops-import-flux`](./gitops-import-flux/README.md) with the `podinfo` path first

Use the contrast-heavy siblings and follow-ons only after one of those standard stories has already created value.

Use these before or after the standard stories when you need a narrower follow-on:
- No-cluster evidence first:
  - [`../connect-and-compare`](./connect-and-compare/README.md)
  - [`../import-from-bundle`](./import-from-bundle/README.md)
  - [`../connected-summary-storage`](./connected-summary-storage/README.md)
  - [`../artifact-workflow`](./artifact-workflow/README.md)
  - [`fleet-import`](./fleet-import/README.md)
  - [`demo-data-adt`](./demo-data-adt/README.md)
  - [`lifecycle-hazards`](./lifecycle-hazards/README.md)
- Other live import, comparison, ownership, topology, and orphan paths:
  - [`../import-from-live`](./import-from-live/README.md)
  - [`combined-git-live`](./combined-git-live/README.md)
  - [`custom-ownership-detectors`](./custom-ownership-detectors/README.md)
  - [`../graph-export`](./graph-export/README.md)
  - [`orphans`](./orphans/README.md)
  - [`watch-webhook`](./watch-webhook/README.md)
  - [`flux-boutique`](./flux-boutique/README.md)
  - [`platform-example`](./platform-example/README.md)
- App-style layouts:
  - [`../apptique-flux-monorepo`](./apptique-flux-monorepo/README.md)
  - [`../apptique-argo-applicationset`](./apptique-argo-applicationset/README.md)
  - [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md)
- Worker and model examples:
  - [`../custom-workers`](../custom-workers)
  - [`../promotion-demo-data`](../promotion-demo-data/README.md)
  - [`springboot-platform-app`](./springboot-platform-app/README.md)

Use [`global-app-layer`](./global-app-layer/README.md) after that for layered recipe structure, deployment units, and the NVIDIA-shaped chain model.

If the goal is to understand the next operational layer that these examples point toward, use:

- [`cub-proc`](./cub-proc/README.md)

If the goal is to think about real clusters and targets from scratch, use:

- [`vmcluster-from-scratch`](./vmcluster-from-scratch.md)

If the goal is to see the smallest live deployment after that bootstrap, use:

- [`vmcluster-nginx-path`](./vmcluster-nginx-path.md)

For both of those paths, the authoritative runnable implementation is Jesper's [`cub-vmcluster`](https://github.com/jesperfj/cub-vmcluster). The incubator pages here explain how that flow fits into the broader ConfigHub example and target model.

## Examples Outside Incubator

- [`../gitops-import`](../gitops-import/README.md): canonical GitOps import example and docs companion.
- [`../global-app`](../global-app/README.md): classic multi-service app example.
- [`../promotion-demo-data`](../promotion-demo-data/README.md): canonical App-Deployment-Target promotion example, no cluster required.
- [`../custom-workers`](../custom-workers): official worker extension examples, including bridge workers, function workers, and policy or validation workers.
- [`../helm-platform-components`](../helm-platform-components/README.md): platform component setup example.
- [`../vm-fleet`](../vm-fleet/README.md): VM fleet operations example.

## Purpose

- Keep examples easy to trust and review.
- Iterate quickly on UX and operational-model ideas.
- Promote only after clear validation.

## Rules

- Keep changes additive and easy to diff.
- Include verification commands.
- Do not break existing root examples.
- For major examples, follow [`ai-example-playbook.md`](./ai-example-playbook.md).
