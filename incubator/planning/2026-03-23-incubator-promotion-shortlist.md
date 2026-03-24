# Promotion Shortlist: Incubator Examples

**Date:** 2026-03-23
**Status:** Proposed
**Scope:** `/Users/alexis/Public/github-repos/examples/incubator`

## Purpose

Turn the roadmap's "decide promotions" phase into a concrete shortlist.

This is not a final promotion decision log. It is a practical recommendation for which incubator examples are strongest candidates to graduate first, which ones are good second-wave candidates, and which ones should stay incubator for now.

## Promotion Standard

An incubator example is a strong promotion candidate when it has all of the following:

- end-to-end runnable flow
- clear read-only-first entry path
- explicit mutation boundaries
- explicit evidence checklist
- AI-first supporting files:
  - `README.md`
  - `AI_START_HERE.md`
  - `contracts.md`
  - `prompts.md`
  - `setup.sh`
  - `verify.sh`
  - `cleanup.sh`
- no important mismatch between docs and actual command contract
- dedicated kubeconfig handling if it is a live example
- no unresolved upstream caveat at the center of the example's value proposition

## Current Top Candidates

### `incubator/connect-and-compare`

Completed because:

- smallest high-signal no-cluster example
- deterministic and fast
- excellent AI-first shape
- proves immediate value without cluster setup or ConfigHub mutation

It is still a strong candidate if we decide to promote an evidence-first no-cluster example later.

### `incubator/import-from-live`

Completed because:

- closest bridge from single-player cluster reality to ConfigHub
- strong answer to "I have a cluster, now what can I do with this?"
- dry-run proposal generation keeps the mutation boundary honest

It is still a strong candidate if we decide to promote a brownfield discovery example later.

### `incubator/apptique-flux-monorepo`

Completed because:

- self-contained and live-validated
- clear app-style layout with one base and two overlays
- strongest current candidate for "one app, multiple environments"

It is still a strong candidate if we decide to promote one self-contained app-style example later.

### `incubator/import-from-bundle`

Completed because:

- cluster-free import proposal generation
- strong deterministic evidence story
- natural stable sibling to `connect-and-compare`

It is still a strong candidate if we decide to promote an offline import example later.

### `incubator/apptique-argo-applicationset`

Completed because:

- self-contained and live-validated
- strongest Argo app-layout candidate in the repo
- good counterpart to `incubator/apptique-flux-monorepo`

It is still a strong candidate if we decide to promote one self-contained Argo app-style example later.

### `incubator/graph-export`

Completed because:

- clean topology artifact story
- easy to verify with explicit generated outputs
- adds a different kind of evidence example

It is still a strong candidate if we decide to promote a topology-artifact example later.

### `incubator/connected-summary-storage`

Completed because:

- strong reporting and automation story
- fully no-cluster and deterministic
- broadens the evidence set without adding more live infrastructure

It is still a strong candidate if we decide to promote a reporting example later.

### `incubator/artifact-workflow`

Completed because:

- offline bundle inspection and replay story
- no cluster dependency
- good companion to `incubator/import-from-bundle`

It is still a strong candidate if we decide to promote a bundle-focused offline example later.

## Recommended Next Promotion Wave

These are the strongest next candidates right now.

## Strong Second-Wave Candidates

These should stay close to the front of the queue after the first wave.

## Keep In Incubator For Now

These are valuable, but they should stay incubator until either the surrounding contracts settle down or the set is narrower.

### Controller-heavy import examples

- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`

Why:

- high value, but more moving parts
- better as incubator references while the simpler import stories mature

### Examples with upstream caveat-linked behavior

- `incubator/custom-ownership-detectors`
- `incubator/orphans`
- `incubator/fleet-import`

Why:

- they are good and honest examples
- but their current value is partly tied to filed upstream gaps

### Broader or more advanced examples

- `incubator/flux-boutique`
- `incubator/platform-example`
- `incubator/global-app-layer`

Why:

- strong examples, but they are not the smallest trustworthy front door
- keep them as second-stop material for now

### Operational and conceptual material

- `incubator/cub-proc`
- `incubator/vmcluster-from-scratch.md`
- `incubator/vmcluster-nginx-path.md`

Why:

- important for product direction
- not stable example promotions in the same sense

## Recommended Next Decision Sequence

1. Decide whether the next stable addition should be another narrow no-cluster evidence example or a more operational live example
2. Then decide whether the next stable addition should be bundle-focused, reporting-focused, or another narrow evidence example

## Notes

This shortlist intentionally does not treat "most feature-rich" as the same thing as "best first stable example."

The best first promotions should be:

- small
- honest
- repeatable
- easy for a fresh AI session to drive
- strong enough to show immediate ConfigHub value
