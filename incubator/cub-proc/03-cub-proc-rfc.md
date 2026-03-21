# RFC: `cub-proc` and `Operation` records

## Summary

ConfigHub already stores desired state.

For connected procedures, it should also store structured operational state.

This RFC proposes two things:

- a first-class `Operation` record in ConfigHub for bounded procedures
- `cub-proc` as the CLI over that record

This is not mainly about a new command. It is about making multi-step operations first-class in ConfigHub instead of leaving them split across shell output, GUI pages, worker activity, controller state, and user memory.

Earlier drafts used `cub run` as the working name. This public incubator version uses `cub-proc` to avoid conflict with the existing `cub run` function surface in the `cub` CLI.

## Problem

Some important ConfigHub procedures are not one command.

Examples from the public examples set:

- GitOps discover, import, render, and verify
- resolve worker and target, then apply, then verify readiness
- push an upgrade through layered units, then confirm the resulting state
- create many spaces and units for a representative multi-app dataset, then verify the outcome

Today, users can often start these procedures, but after that they still have to reconstruct:

- what step was reached
- what system is now responsible
- whether the procedure is still running or merely waiting
- which checks have actually passed
- whether the visible state is fresh or stale

## Why existing ConfigHub support is not enough

ConfigHub could already store operational data in principle.

What is missing is a standard bounded-procedure record:

- one standard shape
- one standard lifecycle
- one standard way to create, update, and read it

Without that, ConfigHub can hold fragments of operational evidence, but each flow still invents its own way of producing and interpreting those fragments.

## Proposal

Add a first-class `Operation` record in ConfigHub.

`cub-proc` is the CLI for creating, updating, and reading `Operation` records.

## Canonical model

### `Operation`

An `Operation` is the structured record of one bounded procedure.

Minimum fields:

- `OperationID`
- `Procedure`
- `ProcedureVersion`
- `State`
- `SubjectBinding`
- `ApplyMode`
  - `direct`
  - `argo`
  - `flux`
- `ResolvedBindings`
  - worker
  - target
  - controller when delegated
- `Steps`
- `Assertions`
- `ExternalRefs`
  - bundle or publish refs
  - delegated controller refs when relevant
- `Timestamps`
- `EvidenceRefs`
- `Actor`

### Lifecycle

Minimum operation states:

- `running`
- `waiting`
- `done`
- `failed`

Minimum assertion states:

- `pending`
- `pass`
- `warn`
- `fail`

### Key distinction

- `done` means the procedure reached its current endpoint
- `asserted` means the intended state was actually checked

Example:

- publish to ArgoCD can be `done`
- workload health can still be `pending`

That distinction is the main reason this should exist as more than shell output.

### Apply mode should be explicit

Execution behavior should not be inferred indirectly from subject type.

If an example, bundle, or procedure needs to describe how state reaches the target system, prefer explicit metadata such as:

```yaml
apply: direct
```

or:

```yaml
apply: argo
```

or:

```yaml
apply: flux
```

`kind` may still be useful as a subject classification, but `apply` is what determines completion semantics, waiting behavior, and default assertions.

## CLI surface

Initial CLI shape:

```bash
cub-proc <procedure> [subject]
cub-proc get <operation-id>
cub-proc list
cub-proc watch <operation-id>
```

Initial procedure candidates:

```bash
cub-proc demo-data/install
cub-proc gitops-import/flux
cub-proc gitops-import/argo
cub-proc global-app/install
cub-proc gpu-stack/install
```

## Concrete failure modes without this

### 1. Delegated GitOps stays ambiguous

Current reality:

1. discover controller objects
2. import and render into ConfigHub
3. inspect controller and cluster state separately

Without an `Operation` record, users see command completion but still have to infer what is outstanding and what has actually been proven.

With an `Operation` record, ConfigHub can show:

- discover step done
- import step done
- render assertions pass or fail
- live controller assertions pending, pass, or fail

### 2. Worker setup failures are easy to misread

The local GitOps import examples require several setup phases. During validation we repeatedly saw situations where a cluster, controller, or worker looked partly healthy but the real problem only became clear after inspecting logs or controller state directly.

With an `Operation` record, this becomes one visible procedure instead of a successful shell command followed by hidden operational ambiguity.

### 3. Multi-phase install procedures stay hard to review

Examples such as [../../promotion-demo-data](../../promotion-demo-data/README.md), [../../global-app](../../global-app/README.md), and [../global-app-layer/gpu-eks-h100-training](../global-app-layer/gpu-eks-h100-training/README.md) already have clear procedure structure.

Without an `Operation` record, users have commands and scripts but no single operational record of the install procedure.

With an `Operation` record, ConfigHub can show:

- whether preflight passed
- whether config materialization finished
- whether binding and apply ran in the right order
- which assertions passed afterwards

## MVP

For MVP:

- procedure profiles are hardcoded in `cub`
- ordered steps are hardcoded in `cub`
- default assertions are hardcoded in `cub`
- only `cub-proc` emits `Operation` records
- existing commands stay unchanged

The current examples should remain runnable without `cub-proc`.

## `watch` and re-derivation

MVP does not require a long-lived server-side run engine.

Instead:

- `cub-proc` executes locally in the CLI
- `get` reads stored `Operation` state from ConfigHub
- `watch` re-derives current state by polling underlying systems using the stored `Operation` plus the hardcoded procedure profile

To make that work, the stored `Operation` must include enough context for re-derivation, at minimum:

- procedure
- subject binding
- apply mode
- resolved worker and target
- delegated controller ref when relevant
- assertion set or profile to re-check
- bundle or publish reference when needed
- any controller reference needed for status lookup

## Suggested first profile

The cleanest first profile is `demo-data/install`.

Reasons:

- it is a real runnable example
- it is clearly multi-phase
- it has obvious assertions
- it does not require a live cluster

After that, the next strongest profiles are the two GitOps import examples, then the richer live deployment examples.

## Open questions

- What is the right ConfigHub backing type for `Operation` records?
- What is the minimum assertion set for the first three procedure profiles?
- When should `cub-proc` stop waiting automatically versus return `waiting`?
- When, if ever, should procedure profiles become declarative rather than hardcoded?
