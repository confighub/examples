# PRD: `cub run`

## Status

Draft

## Summary

ConfigHub already supports operations that span more than one low-level CLI action.

ConfigHub should also act as the system of record for bounded operational procedures in connected environments.

This PRD proposes:

- a first-class `Operation` record in ConfigHub
- `cub run` as the public CLI for creating, updating, and reading those records

The point is not mainly a new command. The point is to stop treating multi-step procedures as shell output plus memory.

Repository context:

- runnable scenarios and worked examples live in this repo
- the public `cub` CLI lives in [`confighub`](https://github.com/confighubai/confighub)

## What `cub run` adds

ConfigHub could already store operational data in principle.

What is missing today is a standard bounded-procedure record:

- a standard shape
- a standard lifecycle
- a standard CLI for creating, updating, and reading it

Without `cub run`, ConfigHub can hold fragments of operational evidence, but each flow still invents its own way of producing and interpreting those fragments.

With `cub run`, ConfigHub gets one consistent operational record for a bounded procedure.

## User Problem

Some important ConfigHub tasks are already bounded procedures, not single commands.

Examples from this repo:

- preflight plus target selection plus apply
- GitOps discovery plus import plus renderer assertions
- upgrade plus wait plus assertions
- onboarding flows where worker, target, controller, and auth must be checked before continuing

In those cases, users need to know:

1. where am I
2. am I done
3. what failed, and what is still waiting
4. what has actually been proven

Without that, users stitch the procedure together from shell scripts, terminal output, GUI pages, controller state, and memory.

More importantly, the system itself has no shared record of the procedure as it progressed.

That is already visible in the runnable examples in this repo.

## How this helps users get apps up faster

`cub run` does not make Kubernetes, ArgoCD, Flux, or workers execute faster.

It reduces four specific delays:

### 1. Delay from not knowing what to do next

Users lose time when they cannot tell which step they are on or what the next waiting point is.

### 2. Delay from rerunning work unnecessarily

Users lose time when they cannot tell whether preflight, publish, import, or verification already happened.

### 3. Delay from checking multiple systems for status

Users lose time when they have to inspect CLI output, ConfigHub, controller state, and cluster state separately just to know whether the procedure is actually complete.

### 4. Delay from interruption or handoff

Users lose time when a later human or AI has to reconstruct what happened from terminal history instead of reading one operational record.

## Goal

Give ConfigHub one consistent operational record for each connected bounded procedure, and give `cub` one consistent CLI surface over that record.

For a user, success looks like this:

- they can start one named procedure from `cub`
- they can see the current step
- they can see which steps completed, failed, or are still pending
- they can see which assertions passed, failed, warned, or are still pending
- they can tell whether the procedure is done
- the same operational record is visible later in ConfigHub

## Canonical model

The center of gravity should be the stored object, not the CLI verb.

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
  - bundle refs
  - publish refs
  - delegated controller refs
- `Timestamps`
- `EvidenceRefs`
- `Actor`

### `Step`

A concrete stage inside the procedure.

Minimum fields:

- `Name`
- `State`
- `StartedAt`
- `FinishedAt`
- `Message`

### `Assertion`

A named check about current or resulting state.

Minimum fields:

- `Name`
- `State`
- `Required`
- `LastEvaluatedAt`
- `EvidenceRef`
- `Message`

### Separation from desired state

`Operation` records are operational data.

They should not appear as ordinary mutations on app or deployment units. They belong in ConfigHub, but distinct from desired-state unit data.

### Apply mode should be explicit

Execution semantics should not be inferred indirectly from subject type.

If an example, bundle, or procedure needs to describe how desired state reaches the target system, prefer explicit metadata such as:

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

`kind` can still be useful as subject classification, but `apply` is what determines waiting behavior, completion semantics, and default assertions.

## Lifecycle

Suggested operation states:

- `running`
- `waiting`
- `done`
- `failed`

Suggested step states:

- `pending`
- `running`
- `done`
- `failed`
- `skipped`

Suggested assertion states:

- `pending`
- `pass`
- `warn`
- `fail`

## Critical distinction: done vs asserted

This is the core behavioral distinction.

- `done` means the procedure reached its current endpoint
- `asserted` means the intended state was actually checked

Example:

- publish to ArgoCD can be `done`
- workload health can still be `pending`

So `cub run` must show both:

- procedure state
- assertion state

A plain `echo "done"` only covers step completion. It does not tell the user whether the result was proven.

## Procedure profiles

For MVP, procedure profiles should be hardcoded in `cub`.

That means:

- procedure names are hardcoded in the CLI
- ordered steps are hardcoded in the CLI
- default assertions are hardcoded in the CLI
- required inputs and defaults are hardcoded in the CLI

This keeps the first version understandable and shippable.

Later, ConfigHub may choose to externalize procedure profiles into declarative definitions. That is out of scope for MVP.

## Procedure profile contract

Each hardcoded procedure profile should define at least:

- subject resolver
- worker resolver
- target resolver
- apply mode resolver
- step evaluators
- assertion evaluators
- external refs to store for later `watch`

## Candidate procedures from current examples

The strongest current candidates are described in [procedure-candidates.md](./procedure-candidates.md).

The best first ones are:

- `demo-data/install`
- `gitops-import/flux`
- `gitops-import/argo`
- `global-app/install`
- `gpu-stack/install`

## Proposed CLI surface

### Main verb

`cub run` should be the primary CLI verb for bounded procedures.

### MVP commands

```bash
cub run <procedure> [subject]
cub run get <operation-id>
cub run list
cub run watch <operation-id>
```

### Initial procedures

```bash
cub run demo-data/install
cub run gitops-import/flux
cub run gitops-import/argo
cub run global-app/install
cub run gpu-stack/install
```

### Initial flags

```bash
--assert
--record none|summary|full
--target <space/target>
--worker <space/worker>
--apply direct|argo|flux
--verbose
--open-gui
```

## MVP boundary

For MVP:

- only `cub run` emits `Operation` records
- existing commands such as `cub unit apply`, `cub gitops import`, and `cub function do` remain unchanged
- no `rerun`, `abort`, or step resume

This keeps the first implementation contained and avoids silently changing the contract of existing commands.

## Persistence defaults

### Core rule

For connected mutating procedures, `Operation` records should persist by default.

Local ephemeral mode should be explicit opt-out, not the implied norm.

### Recording modes

```text
--record=none     local or explicit opt-out; do not persist the Operation
--record=summary  persist operation state, step state, assertion state, bindings, refs, timestamps
--record=full     persist summary plus additional evidence refs
```

## `get`, `watch`, and `list`

For MVP, `cub run` should not require a long-lived server-side run engine.

Instead:

- `cub run` executes locally in the CLI
- `get` reads the stored `Operation` from ConfigHub
- `watch` re-derives current state by polling underlying systems using the stored `Operation` plus the hardcoded procedure profile

`cub run list` should show persisted `Operation` records only.

## `--assert` semantics

`--assert` should mean:

- evaluate assertions when evidence is available
- print assertion outcomes in the CLI
- return non-zero if any evaluated required assertion fails

For MVP:

- `pass` and `warn` do not fail the command
- `fail` does fail the command
- `pending` does not fail the command by itself

This matters for delegated flows.

A procedure may return with state `waiting` while assertions are still `pending`. That should not be treated as a hard failure.

## Examples that support the design

The strongest current evidence is:

- [why-cub-run-example-promotions.md](./why-cub-run-example-promotions.md)
- [../gitops-import-argo](../gitops-import-argo/README.md)
- [../gitops-import-flux](../gitops-import-flux/README.md)
- [../global-app-layer](../global-app-layer/README.md)

## Current position

The examples should not wait on `cub run`.

The current wedge is already strong without it. The job of `cub run` is to give those worked procedures a consistent operational record later, not to become a prerequisite for using the examples now.
