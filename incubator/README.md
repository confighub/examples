# Examples Incubator

This directory is for experimental examples before promotion to stable examples.

## Entry Paths

- For humans: [`../START_HERE.md`](../START_HERE.md)
- For AI assistants: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Incubator AI protocol: [`AGENTS.md`](./AGENTS.md)
- Fuller incubator AI guide: [`AI-README-FIRST.md`](./AI-README-FIRST.md)

## Current Experiments

- [`cub-proc`](./cub-proc/README.md): public incubator design work for bounded procedures, `Operation` records, and `cub-proc`, grounded in the runnable examples in this repo.
- [`gitops-import-argo`](./gitops-import-argo/README.md): incubator Argo GitOps import example built from the stable `gitops-import` assets plus stronger contrast fixtures adapted from `cub-scout`.
- [`gitops-import-flux`](./gitops-import-flux/README.md): incubator Flux GitOps import example built from `cub-scout`'s D2 and podinfo fixtures with the same import-and-evidence verification model as the Argo sibling.
- [`global-app-layer`](./global-app-layer/README.md): recipes and layers package with specs plus four worked examples, including a GPU-flavored chain.
- [`cub-proc-fixtures`](./cub-proc-fixtures/README.md): tiny direct and delegated apply fixtures preserved from the earlier `cub-up` exploration.
- [`promotion-demo-data-verify`](./promotion-demo-data-verify/README.md): verification wrapper for the stable `promotion-demo-data` example.

## Where To Start By Goal

If the goal is the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, do not start with `global-app-layer`.

Start with:
- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- Argo import: [`gitops-import-argo`](./gitops-import-argo/README.md)
- Flux import: [`gitops-import-flux`](./gitops-import-flux/README.md)
- Worker extensibility and policy: [`../custom-workers`](../custom-workers)
- Helm-first story: [`../helm-platform-components`](../helm-platform-components/README.md) or [cub-scout Helm quickstart](https://github.com/confighub/cub-scout/blob/main/docs/reference/cub-track-quickstart-helm.md)
- App-Deployment-Target story: [`../promotion-demo-data`](../promotion-demo-data/README.md)
- Microservice app styles: [cub-scout apptique examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)

Use [`global-app-layer`](./global-app-layer/README.md) after that for layered recipe structure, deployment units, and the NVIDIA-shaped chain model.

If the goal is to understand the next operational layer that these examples point toward, use:

- [`cub-proc`](./cub-proc/README.md)

## Stable Examples (outside incubator)

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
