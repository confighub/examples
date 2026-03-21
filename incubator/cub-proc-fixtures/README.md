# `cub-proc-fixtures`

This directory preserves the only parts of the earlier `cub-up` exploration that still look useful:

- one tiny direct-apply fixture
- one tiny delegated-apply fixture

These are not the main incubator entrypoint.
They are small reference inputs for `cub-proc` design and testing.

## What is here

- [`global-app`](./global-app/up.yaml): minimal direct apply seed
- [`argocd-guestbook`](./argocd-guestbook/up.yaml): minimal delegated apply seed

## Why keep them

They still illustrate a few useful ideas clearly:

- `apply` mode changes what “done” means
- preflight should happen before mutation
- assertions should be explicit, not implied by command exit
- GUI links and stale/existing-resource policy matter in demos

## Current preferred language

Prefer explicit apply mode:

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

A separate `kind` field may still be useful for subject classification, but apply mode is the operationally important part.

## Relationship to current incubator work

For worked examples and live ConfigHub usage, start with:

- [global-app-layer](../global-app-layer/README.md)

Keep this directory only as seed material for:

- `cub-proc` design
- direct-vs-delegated tests
- tiny fixture-based experiments

For the public `cub-proc` design docs that now sit next to the runnable examples, see:

- [../cub-proc](../cub-proc/README.md)
