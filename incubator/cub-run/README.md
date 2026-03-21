# `cub run`

This directory makes the current `cub run` design work public inside the main `examples` incubator.

The central idea is simple.

Some important ConfigHub tasks are not single commands. They are bounded procedures with steps, waiting points, assertions, and evidence across multiple systems. Today those procedures are usually represented by shell scripts, terminal output, worker logs, controller state, and human memory. The `cub run` design proposes one consistent operational record for those procedures, with `cub run` as the CLI over that record.

This does not replace the current import-first examples. It builds on them.

The current wedge remains:

- GitHub
- Argo or Flux
- AI or CLI
- ConfigHub
- evidence

The incubator GitOps import examples already show the operational gaps clearly. They prove that ConfigHub can import and organize WET configuration and surface useful evidence. They also show how much manual stitching is still needed when a procedure spans discovery, import, rendering, controller refresh, and live verification.

That is why the `cub run` design belongs here.

## What Is Here

- [03-cub-run-prd.md](./03-cub-run-prd.md): product framing for `cub run` and `Operation` records
- [03-cub-run-rfc.md](./03-cub-run-rfc.md): technical RFC for the same idea
- [procedure-candidates.md](./procedure-candidates.md): mapping from proposed `cub run` procedures to runnable examples in this repo
- [why-cub-run-example-promotions.md](./why-cub-run-example-promotions.md): evidence from `promotion-demo-data`

## Relationship To Current Examples

These docs do not make `cub run` a dependency of the current examples.

They do three narrower things:

- explain why the examples point toward a bounded-procedure model
- identify which runnable examples are strong candidates for future `cub run` profiles
- keep the operational design work visible in the same public place as the examples it is based on

## Strongest Current Example Anchors

- GitOps import with Argo: [../gitops-import-argo](../gitops-import-argo/README.md)
- GitOps import with Flux: [../gitops-import-flux](../gitops-import-flux/README.md)
- App-Deployment-Target dataset: [../../promotion-demo-data](../../promotion-demo-data/README.md)
- Global app install story: [../../global-app](../../global-app/README.md)
- Layered recipe and deployment variants: [../global-app-layer](../global-app-layer/README.md)
- Tiny design fixtures: [../cub-run-fixtures](../cub-run-fixtures/README.md)

## Recommended Reading Order

If you are new to the idea, read:

1. [procedure-candidates.md](./procedure-candidates.md)
2. [03-cub-run-prd.md](./03-cub-run-prd.md)
3. [03-cub-run-rfc.md](./03-cub-run-rfc.md)

If you want the strongest current worked evidence for a procedure profile, read:

1. [why-cub-run-example-promotions.md](./why-cub-run-example-promotions.md)

## Current Position

The current examples should stay strong without `cub run`.

The job of this incubator directory is to make the next operational layer legible and testable against real examples, not to force the examples to wait for a new command surface.
