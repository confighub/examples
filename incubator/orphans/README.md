# Orphans

This incubator example adapts the `orphans` flow from `cub-scout` into the official `examples` repo.

It shows a small live orphan-discovery path:

- one local `kind` cluster
- one copied fixture set of unmanaged resources
- one orphan inventory from `cub-scout map orphans --json`
- one captured native `trace` result for a representative orphan

## What This Example Is For

Use this example when you want to show how a platform team can find unmanaged resources that GitOps does not know about.

This example does not write ConfigHub state. It focuses on live orphan discovery.

## Source

This example is adapted from:

- [cub-scout orphans](https://github.com/confighub/cub-scout/tree/main/examples/orphans)

## What It Reads

It reads:

- the copied `fixtures/realistic-orphans.yaml`
- the current Kubernetes context created by `kind`
- the `cub-scout` binary

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- fixture resources in:
  - `legacy-apps`
  - `temp-testing`
  - `default`

It writes local verification output under `sample-output/`:

- `orphans.json`
- `debug-nginx.trace.json`
- `debug-nginx.trace.stderr.txt`
- `debug-nginx.trace.exitcode`

It does not write ConfigHub state.

## Read-Only First

```bash
cd incubator/orphans
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

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster and applying the orphan fixture set. It also writes local inspection output.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It checks that key fixture resources appear as `Native` in the orphan inventory.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and clears local sample output.

## What Success Looks Like

At the cluster level you should see orphan-like resources in three namespaces:

- `legacy-apps`
- `temp-testing`
- `default`

At the orphan inventory level you should see these fixture resources classified as `Native`:

- `Deployment/legacy-prometheus`
- `Deployment/debug-nginx`
- `Deployment/debug-busybox`
- `Deployment/hotfix-worker`
- `ConfigMap/manual-override`
- `Secret/manual-api-key`

The fixture also creates `CronJob/manual-cleanup`, but the current `map orphans` output does not surface that object yet. This example keeps the CronJob in the cluster-level checks and treats the inventory gap as a current tool limitation rather than pretending it is covered. See [cub-scout#335](https://github.com/confighub/cub-scout/issues/335).

## Evidence To Check

Direct cluster evidence:

```bash
kubectl get deployment -n legacy-apps
kubectl get deployment -n temp-testing
kubectl get configmap -n default manual-override -o yaml
kubectl get secret -n default manual-api-key -o yaml
```

Orphan evidence:

```bash
./verify.sh
jq '.[] | select(.owner == "Native") | {namespace, kind, name, owner}' sample-output/orphans.json
jq '{target, summary}' sample-output/debug-nginx.trace.json
cat sample-output/debug-nginx.trace.exitcode
```

Current `trace` behavior for the representative orphan may still be surprising. During validation, `trace` returned a `Flux` owner type for the native `debug-nginx` Deployment and exited non-zero even though `map orphans` correctly classified it as `Native`. This example captures that output as evidence instead of overclaiming a clean native trace. See [cub-scout#334](https://github.com/confighub/cub-scout/issues/334).

## Why This Example Matters

This gives the incubator set a practical operator story about unmanaged live state.

It answers a common question quickly:

- what is running in this cluster outside GitOps and outside our intended control plane?

That makes it a good companion to `import-from-live`, because it highlights the part of a brownfield cluster that still needs explicit attention.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
