# promoter — Promotion workflows on ConfigHub

A Kargo-like promotion UI, specialized for ConfigHub's component/variant
organization. You build a workflow of sequential stages; for each stage you
choose which **variant** of which **component** it deploys; promoting a stage
performs a real ConfigHub upstream upgrade — the same `cub unit update --patch
--upgrade` that moves config down a clone chain.

Unlike the other examples in this repo, promoter does **not** seed data. It is
a webapp that runs on top of an existing component/variant layout, built on the
published ConfigHub JS SDK ([confighub/js-sdk](https://github.com/confighub/js-sdk)):
data via [`@confighub/rtk-query`](https://www.npmjs.com/package/@confighub/rtk-query)
and browser-direct login via
[`@confighub/react-auth`](https://www.npmjs.com/package/@confighub/react-auth).
It deploys as a static SPA (see [deploy/README.md](deploy/README.md)). Point it
at an org seeded with
[`../promotion-demo-data`](../promotion-demo-data) for a realistic catalog.

## Concepts → ConfigHub mapping

| Promoter concept | ConfigHub |
|---|---|
| **Component** | the `Component` label on a Space (e.g. `eshop`) |
| **Variant** | a Space, identified by its `Variant` label (e.g. `us-prod-1`); holds that component's units |
| **Stage** | a named step in a workflow; names, per component, the variant it deploys |
| **Workflow** | an ordered list of stages, stored as one `AppConfig/YAML` unit |
| **Promote** | upgrade the stage's variant-Space units from their upstream (the previous stage's variant) via `patchUnit --upgrade` |

"Variant X of component Y" is the Space where `Component=Y` and `Variant=X`.

## Storage

The app creates-or-updates its own `promoter` Space and stores each workflow as
an `AppConfig/YAML` unit (labelled `app=promoter`) whose YAML body is the
workflow document. No Space metadata of the components/variants is ever
modified — a variant may participate in many workflows, so the membership lives
only in the workflow document.

```yaml
apiVersion: promoter.confighub.com/v1
name: web-release
statusLabel: Status          # Space-label key to read each variant's live status from
stages:
  - name: dev
    components:
      - { component: eshop, variant: us-dev-1 }
  - name: staging
    components:
      - { component: eshop, variant: us-staging-1 }
```

The document holds only the pipeline shape — it does **not** store status.

## Promotion is a real upgrade — or it's disabled

Promoting a component into a stage upgrades that variant-Space's units from
their upstream link. This only works when the chosen variant is actually a
downstream clone of the previous stage's variant. The Promote button inspects
the link topology first and is **disabled with a reason** when the links don't
line up — it never silently copies data you didn't ask for.

## Stage status — owned by ConfigHub, not the workflow

Status belongs to ConfigHub: it manages the live resources behind each variant
Space, so it knows their health. The app **reads** each variant's status from a
**label on its Space** (key from `statusLabel`, default `Status`) and **never
writes it** — the workflow tool doesn't independently manage status. The
pipeline view polls every 5s, so changes appear live; each stage shows a status
rolled up from its components, and the Promote gate for a stage only opens once
its upstream stage is `succeeded`.

Today an operator sets the label (or a CLI command, simulating a future agent);
eventually agents watching Argo/etc. write it. Either way the UI reads the same
label. Simulate a change:

```bash
cub space update --patch <variant-space> --label "Status=Progressing"
cub space update --patch <variant-space> --label "Status=Ready"
```

Label values map leniently: `Ready`/`Healthy`/`Synced` → ready, `Progressing`/
`Deploying`/`Pending` → in progress, `Degraded`/`Failed` → failed, missing →
no status. The read side is a pluggable `StatusProvider`
(`app/src/model/status.ts`); swapping the source needs no UI changes.

## Develop

See [app/README.md](app/README.md): register a dev OAuth client, then
`npm install && npm run dev` (http://localhost:5181) and click **Log in**.

## Boundaries

This is a desired-state promotion tool. It is **not** a CD controller (Argo/
Flux still reconcile to clusters), **not** a live health monitor (that's the
future agent-reported status), and it does not author component config itself —
it orchestrates promotions across variants that already exist.
