# Demo Data ADT

This incubator example adapts the `demo-data-adt` flow from `cub-scout` into the official `examples` repo.

It is scan-first and no-cluster:

- one dev `eshop` fixture
- one prod `eshop` fixture
- one prod `website` fixture
- App-Deployment-Target-style labels and annotations
- immediate risk findings from static scan

## What This Example Is For

Use this example when you want to show two things at once:

- App-Deployment-Target-style labeling on realistic workloads
- immediate value from static risk scanning

This is a local evidence example. It does not mutate ConfigHub or live infrastructure.

## Source

This example is adapted from:

- [cub-scout demo-data-adt](https://github.com/confighub/cub-scout/tree/main/examples/demo-data-adt)

It pairs well with the stable [promotion-demo-data](/Users/alexis/Public/github-repos/examples/promotion-demo-data/README.md) example in this repo.

## What It Reads

It reads:

- the copied YAML fixtures under `fixtures/`
- the copied expected scan outputs under `expected-output/`
- the `cub-scout` binary

## What It Writes

It writes local files only:

- `sample-output/dev-eshop.scan.json`
- `sample-output/prod-eshop.scan.json`
- `sample-output/prod-website.scan.json`

It does not mutate ConfigHub state.
It does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/demo-data-adt
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

`./setup.sh` does not mutate ConfigHub or live infrastructure. It writes local scan output only.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It compares local output against the committed expected output.

`./cleanup.sh` removes local sample output only.

## What Success Looks Like

You should see:

- `dev-eshop` produces two warnings for missing resource limits
- `prod-eshop` produces no findings
- `prod-website` produces one warning for missing resource limits

The fixtures also show ADT-style labels and annotations such as:

- `confighub.com/Labels.App`
- `confighub.com/Labels.AppOwner`
- `confighub.com/Labels.TargetRole`
- `confighub.com/Labels.TargetRegion`

## Evidence To Check

```bash
jq '.static.findings' sample-output/dev-eshop.scan.json
jq '.static.findings' sample-output/prod-eshop.scan.json
jq '.static.findings' sample-output/prod-website.scan.json
```

## Why This Example Matters

This gives the incubator set one concrete example where a real issue is visible immediately:

- missing resource limits in dev
- no issue in prod `eshop`
- missing resource limits in prod `website`

That makes it useful for the current wedge because it answers the value question quickly without needing a cluster.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
