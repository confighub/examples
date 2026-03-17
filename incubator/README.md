# Examples Incubator

This directory is for experimental examples before promotion to stable examples.

## Current Experiments

- [`global-app-layer`](./global-app-layer/README.md): recipes and layers package with specs plus four worked examples, including a GPU-flavored chain.
- [`cub-run-fixtures`](./cub-run-fixtures/README.md): tiny direct and delegated apply fixtures preserved from the earlier `cub-up` exploration.
- [`promotion-demo-data-verify`](./promotion-demo-data-verify/README.md): verification wrapper for the stable `promotion-demo-data` example.

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
