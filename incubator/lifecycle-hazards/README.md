# Lifecycle Hazards

This incubator example adapts the `lifecycle-hazards` flow from `cub-scout` into the official `examples` repo.

It shows a small no-cluster migration-risk path:

- one copied manifest file with Helm and Argo hook patterns
- one hook inventory from `cub-scout map hooks --file`
- one lifecycle-hazard scan from `cub-scout scan --file --lifecycle-hazards --json`
- one explicit split between hook ambiguity and general static scan findings

## What This Example Is For

Use this example when you want to show how a platform team can detect GitOps lifecycle hazards before moving a Helm chart under Argo CD.

This example is file-based. It does not mutate ConfigHub or live infrastructure.

## Source

This example is adapted from:

- [cub-scout lifecycle-hazards](https://github.com/confighub/cub-scout/tree/main/examples/lifecycle-hazards)

## What It Reads

It reads:

- the copied `fixtures/helm-hooks.yaml`
- the `cub-scout` binary

## What It Writes

It writes local verification output under `sample-output/`:

- `hooks.json`
- `lifecycle-scan.json`
- normalized copies for comparison

It does not write ConfigHub state.
It does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/lifecycle-hazards
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` does not mutate ConfigHub or live infrastructure. It writes local JSON output only.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It compares local output against the committed expected output.

`./cleanup.sh` removes local sample output only.

## What Success Looks Like

At the hook inventory level you should see:

- four hook resources
- `db-migrate` mapped to `PostSync`
- `notify-deploy` mapped to `PostSync`
- two explicit Argo hooks with `PreSync` and `PostSync`

At the lifecycle-hazard level you should see:

- one `helm-hook-ambiguity` finding for `Job/db-migrate`
- one `postsync-idempotency-risk` finding for `Job/db-migrate`
- one static finding for missing resource limits on `Deployment/api`

## Evidence To Check

```bash
./verify.sh
jq '.hooks[] | {name, mappedPhase, helmHooks, argoHook}' sample-output/hooks.json
jq '.lifecycleHazards.findings[] | {rule, resource, mappedPhase}' sample-output/lifecycle-scan.json
jq '.static.findings[] | {name, resource_name, severity}' sample-output/lifecycle-scan.json
```

## Why This Example Matters

This gives the incubator set a strong no-cluster migration example.

It answers a practical question quickly:

- what breaks or becomes ambiguous when we move Helm hooks under Argo CD?

That makes it a good companion to `demo-data-adt`, while adding a different kind of static evidence: migration and lifecycle risk instead of application config risk.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
