# Examples Incubator

This directory is for experimental examples before promotion to stable examples.

## Entry Paths

- For humans: [`../START_HERE.md`](../START_HERE.md)
- For AI assistants: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Incubator AI protocol: [`AGENTS.md`](./AGENTS.md)
- Fuller incubator AI guide: [`AI-README-FIRST.md`](./AI-README-FIRST.md)

## Current Experiments

- [`global-app-layer`](./global-app-layer/README.md): recipes and layers package with specs plus four worked examples, including a GPU-flavored chain.
- [`cub-run-fixtures`](./cub-run-fixtures/README.md): tiny direct and delegated apply fixtures preserved from the earlier `cub-up` exploration.
- [`promotion-demo-data-verify`](./promotion-demo-data-verify/README.md): verification wrapper for the stable `promotion-demo-data` example.

## Where To Start By Goal

If the goal is the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge, do not start with `global-app-layer`.

Start with:
- Argo import: [cub-scout argo-import-confighub-demo](https://github.com/confighub/cub-scout/tree/main/examples/argo-import-confighub-demo)
- Flux import: [cub-scout flux-import-confighub-demo](https://github.com/confighub/cub-scout/tree/main/examples/flux-import-confighub-demo)
- Helm-first story: [`../helm-platform-components`](../helm-platform-components/README.md) or [cub-scout Helm quickstart](https://github.com/confighub/cub-scout/blob/main/docs/reference/cub-track-quickstart-helm.md)
- App-Deployment-Target story: [`../promotion-demo-data`](../promotion-demo-data/README.md)
- Microservice app styles: [cub-scout apptique examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)

Use [`global-app-layer`](./global-app-layer/README.md) after that for layered recipe structure, deployment units, and the NVIDIA-shaped chain model.

## Stable Examples (outside incubator)

- [`promotion-demo-data`](../promotion-demo-data/README.md): creates 49 spaces and ~154 units using the App-Deployment-Target model. Uses noop bridge, no cluster required. Canonical example of ConfigHub's multi-env promotion model.

## Purpose

- Keep stable examples easy to trust and review.
- Iterate quickly on UX and operational-model ideas.
- Promote only after clear validation.

## Rules

- Keep changes additive and easy to diff.
- Include verification commands.
- Do not break existing stable examples.
- For major examples, follow [`ai-example-playbook.md`](./ai-example-playbook.md).
