# Platform Example

This incubator example adapts the `platform-example` flow from `cub-scout` into the official `examples` repo.

It shows a mixed-ownership live cluster:

- one local `kind` cluster
- Flux controllers in `flux-system`
- one working Flux podinfo path
- one copied orphan fixture set
- one ownership inventory from `cub-scout map list --json`
- one orphan inventory from `cub-scout map orphans --json`
- one representative trace from a Flux-managed workload

## What This Example Is For

Use this example when you want to show a realistic mixed cluster where both GitOps-managed resources and unmanaged resources coexist.

This example does not write ConfigHub state.

## Source

This example is adapted from:

- [cub-scout platform-example](https://github.com/confighub/cub-scout/tree/main/examples/platform-example)

## What It Reads

It reads:

- the copied `fixtures/orphans.yaml`
- the copied `fixtures/flux/podinfo-kustomizations.yaml`
- the current Kubernetes context created by `kind`
- the `flux` CLI
- the `cub-scout` binary

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- Flux controllers in `flux-system`
- the copied podinfo GitRepository and Kustomization
- orphan demo resources in `default` and `kube-system`

It writes local verification output under `sample-output/`:

- `map-list.json`
- `orphans.json`
- `trace-podinfo.json`
- `flux-status.txt`

It does not write ConfigHub state.

## Read-Only First

```bash
cd incubator/platform-example
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

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster, installing Flux, applying the copied podinfo GitRepository and Kustomization, applying the orphan fixtures, and writing local evidence under `sample-output/`.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It checks that Flux resources exist, that the orphan fixtures exist, that `map list` sees both `Flux` and `Native` ownership, and that `trace` can attribute `Deployment/podinfo` back to a Flux source.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and local kubeconfig, and clears local sample output.

## What Success Looks Like

At the cluster level you should see:

- namespace `flux-system`
- namespace `podinfo`
- a Flux-managed `Deployment/podinfo`
- orphan fixtures such as `Deployment/debug-nginx` and `ConfigMap/manual-config`

At the ownership level you should see:

- `map list --json` entries with both `owner == "Flux"` and `owner == "Native"`
- `map orphans --json` entries for the orphan fixtures
- a `trace` result for `Deployment/podinfo` that identifies Flux ownership and the podinfo source

## Evidence To Check

```bash
./verify.sh
cat sample-output/flux-status.txt
jq '.[] | select(.owner == "Flux") | {namespace, kind, name, owner}' sample-output/map-list.json
jq '.[] | select(.owner == "Native") | {namespace, kind, name, owner}' sample-output/orphans.json
jq '{target, summary}' sample-output/trace-podinfo.json
```

## Why This Example Matters

This gives the incubator set a flagship mixed-ownership story.

It answers a practical question quickly:

- what does a cluster look like when GitOps is present, but real unmanaged resources still exist beside it?

That makes it a strong bridge between the import, ownership, and app-style examples.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
