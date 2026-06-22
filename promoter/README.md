# promoter — Promotion workflows on ConfigHub

A Kargo-like promotion UI, specialized for ConfigHub's component/variant
organization. You build a workflow of sequential stages; for each stage you
choose which **variant** of which **component** it deploys; promoting a stage
performs a real ConfigHub upstream upgrade — the same `cub unit update --patch
--upgrade` that moves config down a clone chain.

Unlike the other examples in this repo, promoter does **not** seed data. It is
a webapp that runs on top of an existing component/variant layout and reuses
ConfigHub auth, deployed the same way as ConfigHub's UI previews (see
[deploy/README.md](deploy/README.md)). Point it at an org seeded with
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
stages:
  - name: dev
    components:
      - { component: eshop, variant: us-dev-1 }
  - name: staging
    components:
      - { component: eshop, variant: us-staging-1 }
status:
  staging/eshop: { state: succeeded, promotedRevision: 7, at: "...", by: "..." }
```

## Promotion is a real upgrade — or it's disabled

Promoting a component into a stage upgrades that variant-Space's units from
their upstream link. This only works when the chosen variant is actually a
downstream clone of the previous stage's variant. The Promote button inspects
the link topology first and is **disabled with a reason** when the links don't
line up — it never silently copies data you didn't ask for.

## Stage status

ConfigHub's built-in apply/live status fields are being phased out, and the
long-term plan is for agents to report stage health from Argo and other sources
into ConfigHub. Until then, status is **manual**: recorded per (stage,
component) in the workflow document, set automatically on a successful promotion
and editable from the status chip. The status source is a pluggable
`StatusProvider` (`app/src/model/status.ts`) — an `AgentReportedStatusProvider`
stub documents the future shape, swappable without UI changes.

## Develop

See [app/README.md](app/README.md): `npm install && npm run dev`
(http://localhost:5181), paste a `cub auth get-token` token when prompted.

## Boundaries

This is a desired-state promotion tool. It is **not** a CD controller (Argo/
Flux still reconcile to clusters), **not** a live health monitor (that's the
future agent-reported status), and it does not author component config itself —
it orchestrates promotions across variants that already exist.
