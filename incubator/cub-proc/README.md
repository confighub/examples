# `cub-proc`

This directory makes the current `cub-proc` design work public inside the main `examples` incubator.

The central idea is simple.

Some important ConfigHub tasks are not single commands. They are bounded procedures with steps, waiting points, assertions, and evidence across multiple systems. Today those procedures are usually represented by shell scripts, terminal output, worker logs, controller state, and human memory. The `cub-proc` design proposes one consistent operational record for those procedures, with `cub-proc` as the CLI over that record.

Earlier drafts used `cub run` as the working name. The public incubator name is now `cub-proc` so it does not collide with the existing `cub run` function namespace in the CLI.

This does not replace the current import-first examples. It builds on them.

The current wedge remains:

- GitHub
- Argo or Flux
- AI or CLI
- ConfigHub
- evidence

The incubator GitOps import examples already show the operational gaps clearly. They prove that ConfigHub can import and organize WET configuration and surface useful evidence. They also show how much manual stitching is still needed when a procedure spans discovery, import, rendering, controller refresh, and live verification.

That is why the `cub-proc` design belongs here.

## What Is Here

- [03-cub-proc-prd.md](./03-cub-proc-prd.md): product framing for `cub-proc` and `Operation` records
- [03-cub-proc-rfc.md](./03-cub-proc-rfc.md): technical RFC for the same idea
- [procedure-candidates.md](./procedure-candidates.md): mapping from proposed `cub-proc` procedures to runnable examples in this repo
- [vmcluster-bootstrap-profile.md](./vmcluster-bootstrap-profile.md): first draft profile for bootstrapping a real `cub-vmcluster` target
- [why-cub-proc-example-promotions.md](./why-cub-proc-example-promotions.md): evidence from `promotion-demo-data`

## Relationship To Current Examples

These docs do not make `cub-proc` a dependency of the current examples.

They do three narrower things:

- explain why the examples point toward a bounded-procedure model
- identify which runnable examples are strong candidates for future `cub-proc` profiles
- keep the operational design work visible in the same public place as the examples it is based on

## Strongest Current Example Anchors

- GitOps import with Argo: [../gitops-import-argo](../gitops-import-argo/README.md)
- GitOps import with Flux: [../gitops-import-flux](../gitops-import-flux/README.md)
- App-Deployment-Target dataset: [../../promotion-demo-data](../../promotion-demo-data/README.md)
- Global app install story: [../../global-app](../../global-app/README.md)
- Layered recipe and deployment variants: [../global-app-layer](../global-app-layer/README.md)
- Real-cluster mental model note: [../vmcluster-from-scratch.md](../vmcluster-from-scratch.md)
- Tiny design fixtures: [../cub-proc-fixtures](../cub-proc-fixtures/README.md)

## Recommended Reading Order

If you are new to the idea, read:

1. [procedure-candidates.md](./procedure-candidates.md)
2. [03-cub-proc-prd.md](./03-cub-proc-prd.md)
3. [03-cub-proc-rfc.md](./03-cub-proc-rfc.md)

If you want the strongest current worked evidence for a procedure profile, read:

1. [why-cub-proc-example-promotions.md](./why-cub-proc-example-promotions.md)

## Current Position

The current examples should stay strong without `cub-proc`.

The job of this incubator directory is to make the next operational layer legible and testable against real examples, not to force the examples to wait for a new command surface.
