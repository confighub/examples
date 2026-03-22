# Custom Ownership Detectors

This incubator example adapts the `custom-ownership-detectors` flow from `cub-scout` into the official `examples` repo.

It shows a small live-inspection path:

- one local `kind` cluster
- one copied custom detector config
- three native Deployments with different ownership signals
- one read-only inspection flow centered on `cub-scout map`, with current `explain` and `trace` behavior captured explicitly

## What This Example Is For

Use this example when you want to show how a platform team can extend ownership detection without writing Go code.

This example does not write ConfigHub state. It focuses on live ownership inspection.

## Source

This example is adapted from:

- [cub-scout custom-ownership-detectors](https://github.com/confighub/cub-scout/tree/main/examples/custom-ownership-detectors)

## What It Reads

It reads:

- the copied `detectors.yaml`
- the copied workload fixtures under `fixtures/`
- the current Kubernetes context created by `kind`
- the `cub-scout` binary

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- one namespace: `detectors-demo`
- three native `Deployment` resources

It writes local verification output under `sample-output/` during `./setup.sh` and `./verify.sh`.

It does not write ConfigHub state.

## Read-Only First

```bash
cd incubator/custom-ownership-detectors
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

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster and applying the fixture Deployments. It also writes local sample output.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It inspects the live cluster and checks that custom owners appear where they should.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and clears local sample output.

## What Success Looks Like

At the cluster level you should see:

- `payments-api` in `detectors-demo` with the custom owner `Internal Platform`
- `infra-ui` in `detectors-demo` with the custom owner `Pulumi`
- `debug-tool` in `detectors-demo` still classified as `Native`

At the inspection level you should see:

- `cub-scout map list --json` reporting the custom owners
- `cub-scout explain --format json` still falling back to the current built-in partial-trace summary
- `cub-scout trace --json` still falling back to the current built-in native summary

## Evidence To Check

Direct cluster evidence:

```bash
kubectl get deployment -n detectors-demo
kubectl get deployment -n detectors-demo payments-api -o yaml
kubectl get deployment -n detectors-demo infra-ui -o yaml
```

Ownership evidence:

```bash
./verify.sh
jq '.[] | {name, owner, ownerDetails}' sample-output/map.json
jq '.owner' sample-output/payments-api.explain.json
jq '.owner' sample-output/infra-ui.explain.json
jq '.summary.ownerType' sample-output/payments-api.trace.json
```

## Why This Example Matters

This gives the incubator set a small platform-team example that is not about import.

It answers one practical question right away:

- how do we teach `cub-scout` about our own platform controller or tool in inventory views?

It also captures a current gap honestly:

- `map` honors custom detectors
- `explain` and `trace` do not yet show the same custom owner names in this build

That makes it a good companion to `import-from-live` and the GitOps import examples, while also giving us a tight regression case for ownership parity.

The current parity gap is tracked in [cub-scout issue #333](https://github.com/confighub/cub-scout/issues/333).

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
