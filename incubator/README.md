# Examples Incubator

This directory is for experimental examples before promotion to stable examples.

Current focus:

Keep building AI-first examples in `examples/incubator` that give one person a fast reason to use ConfigHub, especially by adapting the best `cub-scout` flows into official, evidence-first examples.

## Entry Paths

- For humans: [`../START_HERE.md`](../START_HERE.md)
- For AI assistants: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Incubator AI protocol: [`AGENTS.md`](./AGENTS.md)
- Fuller incubator AI guide: [`AI-README-FIRST.md`](./AI-README-FIRST.md)

## Current Experiments

### No-Cluster Evidence And Inspection

- [`demo-data-adt`](./demo-data-adt/README.md): scan-first App-Deployment-Target example showing labeled workload fixtures plus immediate static risk findings.
- [`lifecycle-hazards`](./lifecycle-hazards/README.md): migration-risk example adapted from `cub-scout`, showing hook inventory and Helm-to-Argo lifecycle hazards from one manifest file.
- [`artifact-workflow`](./artifact-workflow/README.md): offline bundle example adapted from `cub-scout`, showing inspect, replay, and summarize against a copied debug bundle.

### Offline Import And Aggregation

- [`fleet-import`](./fleet-import/README.md): multi-cluster aggregation from two existing cluster import JSONs into one unified proposal.

### Live Import, Comparison, Ownership, Topology, And Orphans

- [`gitops-import-argo`](./gitops-import-argo/README.md): Argo GitOps import example built from the stable `gitops-import` assets plus stronger contrast fixtures adapted from `cub-scout`.
- [`gitops-import-flux`](./gitops-import-flux/README.md): Flux GitOps import example built from `cub-scout`'s D2 and podinfo fixtures with the same import-and-evidence verification model as the Argo sibling.
- [`combined-git-live`](./combined-git-live/README.md): Git-plus-live compare example adapted from `cub-scout`, showing aligned, git-only, and cluster-only results against a real cluster.
- [`custom-ownership-detectors`](./custom-ownership-detectors/README.md): platform-team ownership example adapted from `cub-scout`, showing custom owner names in `map`, `explain`, and `trace` without touching ConfigHub.
- [`orphans`](./orphans/README.md): unmanaged-resource example adapted from `cub-scout`, showing how `Native` resources surface in orphan inventory on a live cluster.
- [`watch-webhook`](./watch-webhook/README.md): event-streaming example adapted from `cub-scout`, showing one-shot `watch --webhook` delivery into a local receiver.
- [`flux-boutique`](./flux-boutique/README.md): Flux microservice fan-out example adapted from `cub-scout`, showing one GitRepository traced through five Kustomizations and services.
- [`platform-example`](./platform-example/README.md): mixed-ownership example adapted from `cub-scout`, showing Flux-managed platform resources beside unmanaged orphan resources.

### App-Style GitOps Layouts

- [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md): Argo app-of-apps layout with one root Application and one child Application per environment.

### Structural And Model Examples

- [`springboot-platform-app`](./springboot-platform-app/README.md): Spring Boot app/platform example for authority vs provenance, with one app and three natural mutation routes.
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

Use this order:
- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- No-cluster evidence first:
  - [`../connect-and-compare`](../connect-and-compare/README.md)
  - [`../import-from-bundle`](../import-from-bundle/README.md)
  - [`../connected-summary-storage`](../connected-summary-storage/README.md)
  - [`fleet-import`](./fleet-import/README.md)
  - [`demo-data-adt`](./demo-data-adt/README.md)
  - [`lifecycle-hazards`](./lifecycle-hazards/README.md)
  - [`artifact-workflow`](./artifact-workflow/README.md)
- Then live import, comparison, ownership, topology, and orphans:
  - [`../import-from-live`](../import-from-live/README.md)
  - [`gitops-import-argo`](./gitops-import-argo/README.md)
  - [`gitops-import-flux`](./gitops-import-flux/README.md)
  - [`combined-git-live`](./combined-git-live/README.md)
  - [`custom-ownership-detectors`](./custom-ownership-detectors/README.md)
  - [`../graph-export`](../graph-export/README.md)
  - [`orphans`](./orphans/README.md)
  - [`watch-webhook`](./watch-webhook/README.md)
  - [`flux-boutique`](./flux-boutique/README.md)
  - [`platform-example`](./platform-example/README.md)
- Then app-style layouts:
  - [`../apptique-flux-monorepo`](../apptique-flux-monorepo/README.md)
  - [`../apptique-argo-applicationset`](../apptique-argo-applicationset/README.md)
  - [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md)
- Then worker and model examples:
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

## Stable Examples (outside incubator)

- [`../connect-and-compare`](../connect-and-compare/README.md): stable no-cluster evidence example showing doctor, compare, and history without a live cluster.
- [`../import-from-live`](../import-from-live/README.md): stable brownfield discovery example showing live-cluster dry-run proposal generation before any ConfigHub mutation.
- [`../import-from-bundle`](../import-from-bundle/README.md): stable offline import example showing dry-run proposal generation from a copied bundle fixture.
- [`../connected-summary-storage`](../connected-summary-storage/README.md): stable reporting example showing stored connected summaries plus dry-run Slack digest generation from local storage.
- [`../apptique-flux-monorepo`](../apptique-flux-monorepo/README.md): stable app-style Flux example showing one base plus dev and prod overlays with dedicated kubeconfig handling.
- [`../apptique-argo-applicationset`](../apptique-argo-applicationset/README.md): stable Argo ApplicationSet example showing one generator and one generated Application per environment.
- [`../graph-export`](../graph-export/README.md): stable live topology example showing `graph.v1` JSON plus DOT, SVG, and HTML artifacts from one local cluster.
- [`promotion-demo-data`](../promotion-demo-data/README.md): creates 49 spaces and ~154 units using the App-Deployment-Target model. Uses noop bridge, no cluster required. Canonical example of ConfigHub's multi-env promotion model.
- [`../custom-workers`](../custom-workers): official worker extension examples, including bridge workers, function workers, and policy or validation workers.

## Purpose

- Keep stable examples easy to trust and review.
- Iterate quickly on UX and operational-model ideas.
- Promote only after clear validation.

## Rules

- Keep changes additive and easy to diff.
- Include verification commands.
- Do not break existing stable examples.
- For major examples, follow [`ai-example-playbook.md`](./ai-example-playbook.md).
