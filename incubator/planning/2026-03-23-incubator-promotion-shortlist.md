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

## Recommended First Promotion Wave

These are the strongest first candidates right now.

### 1. `incubator/connect-and-compare`

Why it stands out:

- smallest high-signal no-cluster example
- deterministic and fast
- excellent AI-first shape
- proves immediate value without cluster setup or ConfigHub mutation

Why it should likely go first:

- it is the easiest example to trust
- it gives one person a fast reason to use the tooling
- it is a good stable anchor for the no-cluster evidence story

### 2. `incubator/import-from-live`

Why it stands out:

- closest bridge from single-player cluster reality to ConfigHub
- strong answer to "I have a cluster, now what can I do with this?"
- dry-run proposal generation keeps the mutation boundary honest

Why it should likely be the first live promotion candidate:

- it aligns directly with the current wedge
- it is easier to explain than the controller-specific import stories
- it gives ConfigHub a clear ingest-and-organize story

### 3. `incubator/apptique-flux-monorepo`

Why it stands out:

- self-contained and live-validated
- clear app-style layout with one base and two overlays
- easier to reason about than the larger Argo siblings

Why it should likely be the first app-style promotion candidate:

- it is the cleanest app-layout story in the current set
- it has a straightforward controller and verification model
- it works well as a template for "one app, multiple environments"

## Strong Second-Wave Candidates

These should stay close to the front of the queue after the first wave.

### `incubator/import-from-bundle`

Strong because:

- cluster-free import proposal generation
- good pair with `connect-and-compare`
- stable bundle-backed evidence story

Held for second wave because:

- it is slightly more specialized than `connect-and-compare`

### `incubator/graph-export`

Strong because:

- clean topology artifact story
- live but narrow
- easy to verify with explicit generated outputs

Held for second wave because:

- it is more about sharing evidence than about loading data into ConfigHub

### `incubator/apptique-argo-applicationset`

Strong because:

- self-contained and live-validated
- strong Argo app-layout story

Held for second wave because:

- `apptique-flux-monorepo` is slightly smaller and simpler as the first app-style stable example

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

1. Promote one no-cluster example first: `connect-and-compare`
2. Promote one live ingest example second: `import-from-live`
3. Promote one app-style example third: `apptique-flux-monorepo`
4. Only then decide whether the next stable additions should be Argo-focused, topology-focused, or bundle-focused

## Notes

This shortlist intentionally does not treat "most feature-rich" as the same thing as "best first stable example."

The best first promotions should be:

- small
- honest
- repeatable
- easy for a fresh AI session to drive
- strong enough to show immediate ConfigHub value
