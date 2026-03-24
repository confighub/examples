# Import From Live

This stable example adapts the `import-from-live` flow from `cub-scout` into the official `examples` repo.

It shows a simple brownfield path:

- one live cluster with mixed ownership signals
- one read-only `cub-scout import --dry-run --json` proposal
- one suggested App structure for ConfigHub
- one clear split between import evidence and actual ConfigHub mutation

## What This Example Is For

Use this example when you want to show how a developer or platform engineer can start from a running cluster and get a proposed ConfigHub structure without hand-mapping workloads first.

This example focuses on dry-run proposal generation. It does not write ConfigHub state by default.

## Source

This example is adapted from:

- [cub-scout import-from-live](https://github.com/confighub/cub-scout/tree/main/examples/import-from-live)

## What It Reads

It reads:

- the copied fixture manifests under `fixtures/`
- the current Kubernetes context created by `kind`
- the `cub-scout` binary

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- four namespaces:
  - `argocd`
  - `myapp-dev`
  - `myapp-staging`
  - `myapp-prod`
- one Argo CD `Application` CRD
- fixture `Application`, `Deployment`, `StatefulSet`, and `ConfigMap` resources

It writes local verification output under `sample-output/` during `./setup.sh` and `./verify.sh`.

It does not write ConfigHub state unless you run the optional manual follow-on import commands yourself.

## Read-Only First

```bash
cd import-from-live
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

`./setup.sh` mutates live infrastructure by creating a `kind` cluster and applying the copied fixtures. It also writes local evidence under `sample-output/`.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It reads the live cluster and compares generated local output with the committed expected output.

`./cleanup.sh` mutates live infrastructure by deleting the `kind` cluster and clears local sample output.

## What Success Looks Like

At the cluster level you should see:

- three Argo `Application` objects in `argocd`
- six Deployments across `myapp-dev`, `myapp-staging`, and `myapp-prod`
- three Helm-labeled `StatefulSet` resources
- one unmanaged `ConfigMap` in `myapp-prod`

At the import-proposal level you should see:

- one proposed App space named `myapp-team`
- three proposed units
- three apps: `api`, `worker`, and `redis`
- one aggregated `default` variant per app
- workload lists spanning `dev`, `staging`, and `prod`

## Evidence To Check

Direct cluster evidence:

```bash
kubectl get application -n argocd
kubectl get deployment -n myapp-dev
kubectl get statefulset -n myapp-prod
kubectl get configmap -n myapp-prod debug-config -o yaml
```

Import proposal evidence:

```bash
./verify.sh
jq '.proposal.appSpace' sample-output/suggestion.json
jq '.proposal.units | length' sample-output/suggestion.json
jq '.proposal.units[] | {slug, app, variant}' sample-output/suggestion.json
```

## Optional Manual Follow-On

If you want to turn the proposal into a real ConfigHub import after reviewing it, run the import command manually and make the mutation explicit:

```bash
cub-scout import --yes
```

Or, if you want immediate worker and target wiring too:

```bash
cub-scout import --yes --connect
```

Those commands mutate ConfigHub and are intentionally not part of the default scripted path here.

## Why This Example Matters

This example is a good bridge between `import-from-bundle` and the GitOps import examples.

It answers a different question:

- not “what would we import from an existing bundle?”
- not “what would we import from an already curated GitOps repo?”
- but “what can we learn directly from a running cluster right now?”

That makes it a strong single-player path and a good starting point for brownfield discovery.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
