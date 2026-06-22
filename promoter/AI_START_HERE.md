# AI Guide: promoter

A promotion-workflow webapp on top of ConfigHub's component/variant layout.
Kargo-like, but promotion is a real ConfigHub upstream upgrade. Unlike the
seed-and-verify examples, this is a UI app with no `setup.sh` — it reads an
existing org and stores its own state in a `promoter` Space.

## Prerequisites

- An org with component/variant Spaces (Spaces labelled `Component` and
  `Variant`). Seed one with `../promotion-demo-data` if you don't have one.
- [cub CLI](https://docs.confighub.com/get-started/setup/) authenticated
  (`cub auth login`) — only needed to grab a dev token and to inspect results.

## Run it locally

```bash
cd app
npm install
npm run dev          # http://localhost:5181
```

Paste a token from `cub auth get-token` when prompted (dev-only; the hosted
deployment uses the same-origin session cookie automatically).

## What to try

1. **Create a workflow.** "New workflow" → name it → you land in the stage
   editor. Add stages (e.g. `dev`, `staging`, `prod`); in each, add components
   and choose a variant for each. Save.
2. **Inspect storage.** The workflow is a real unit:
   ```bash
   cub unit list --space promoter --where "Labels.app = 'promoter'"
   cub unit get <workflow-slug> --space promoter -o data    # the YAML document
   ```
3. **Promote.** On a non-first stage, click **Promote** on a component. The
   dialog inspects upstream links and lists each unit's readiness. If the chosen
   variant is a downstream clone of the previous stage's variant, promote
   upgrades it (optionally applies). Verify:
   ```bash
   cub revision list <unit> --space <variant-space>
   ```
4. **See the gate.** Pick a variant that is *not* a downstream clone of the
   previous stage's variant — the Promote button reports exactly why it can't
   upgrade rather than copying data.
5. **Status.** A successful promotion records `succeeded` with the new
   revision. The status chip is also manually settable; it persists into the
   workflow YAML (reload to confirm).

## Key files

- `app/src/model/workflow.ts` — the stored document shape + (de)serialize.
- `app/src/model/status.ts` — the pluggable status provider (manual default,
  agent-reported stub).
- `app/src/data/catalog.ts` — reads Spaces grouped by `Component`/`Variant`.
- `app/src/data/storage.ts` — the `promoter`-Space workflow CRUD.
- `app/src/data/promote.ts` — upstream-link inspection + `patchUnit` upgrade.

## Deploy

`deploy/` mirrors the UI-preview hosting pattern (nginx same-origin proxy,
ConfigHub-managed Deployment unit, GitHub Actions image bump). See
[deploy/README.md](deploy/README.md).
