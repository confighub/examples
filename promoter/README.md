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
| **Publish** | cut an immutable Release of the stage's variant Space to its OCI Release Target — the artifact a cluster-side controller pulls |

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

## Promoting is not delivering

An upgrade changes desired state in ConfigHub. Nothing reaches a cluster until a
**Release** of the variant Space is published to its OCI Release Target, which a
cluster-side controller (Argo CD, Flux) then pulls. So the dialog offers
publishing as a second, separately confirmed step once the upgrade lands.

The two steps have **different subjects**, and the dialog says so rather than
quietly widening the first approval into the second:

- an upgrade acts on the Units linked upstream — the ones the report listed;
- a Release captures the Space's whole **EffectiveReleaseSet**: every Unit whose
  `TargetID` equals the Space's `ReleaseTargetID`, at its current head, promoted
  or not.

Anything in the second set that was not in the first is listed by name before
the Publish button is offered. Publishing is disabled with a reason when the
Space has no Release Target, when that Target is not an OCI provider, when no
Unit is assigned to it, or when any bundled Unit has an Apply Gate set — the
server refuses a gated Release, and the app does not clear gates as a side
effect.

Publishing untagged bundles each Unit at its head and the server creates a
`release-<num>` Tag on every bundled Revision. Confirming delivery — that the
controller synced and the workloads are running — is outside this app; the Space
status label is what reports it back.

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
