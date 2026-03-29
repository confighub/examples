# promotion-demo-data-verify

Verification wrapper for the stable `promotion-demo-data` example.

## What This Example Is For

This wrapper is for **CI and AI workflows** that need structured verification after the stable demo data has been created.

If you're a human exploring the promotion UI, you usually start in [`../../promotion-demo-data`](../../promotion-demo-data/README.md) and use the ConfigHub UI directly. This incubator wrapper exists so AI and CI can make explicit assertions instead of relying on “look at the output.”

## Stack And Scenario

The stable example creates a multi-app, multi-environment ConfigHub dataset:

- 6 apps
- 7 targets
- 40+ deployment spaces
- 100+ units

This wrapper does not create that data. It verifies that the stable example already populated ConfigHub with the expected shape.

## What This Proves

1. The stable demo setup created the expected number of spaces and units.
2. The label model is queryable across app, role, and region.
3. The intended version skew in `eshop` still exists.
4. Key targets are present, so the promotion UI story still has its anchor objects.

## Prerequisites

- `cub` CLI installed and authenticated (`cub auth login`)
- the stable setup already run from [`../../promotion-demo-data`](../../promotion-demo-data/README.md)
- `jq` only if you want to pretty-print `--json` or `--explain-json`

This script exists because:
- **CI** needs explicit pass/fail to gate PRs
- **AI workflows** need structured assertions instead of "look at the output"

## What This Reads And Writes

| Command | ConfigHub | Git | Cluster |
|--------|-----------|-----|---------|
| `./verify.sh --explain` | reads nothing live | - | - |
| `./verify.sh --explain-json` | reads nothing live | - | - |
| `./verify.sh` | reads spaces, units, targets, and images | - | - |
| `./verify.sh --json` | reads spaces, units, targets, and images | - | - |

This wrapper never mutates ConfigHub, Git, or a cluster.

## Why This Lives In Incubator

The `promotion-demo-data/` example is a stable example outside the incubator. Adding verification scripts directly to it would mix stable content with incubator-style tooling.

This wrapper lets us verify the demo data without modifying the stable example.

## Read-Only Preview

```bash
./verify.sh --explain
./verify.sh --explain-json | jq
```

## Run It

First, run the demo setup:

```bash
cd ../../promotion-demo-data
./setup.sh
```

Then verify:

```bash
cd ../incubator/promotion-demo-data-verify
./verify.sh
./verify.sh --json | jq
```

## What it checks

| Check | Description |
|-------|-------------|
| Space counts | 49+ demo spaces exist, including 8+ platform-owned and 12+ prod |
| Unit counts | 130+ units across app spaces |
| Label presence | Spaces queryable by App label |
| Version skew | us-prod-1-eshop has different image version than eu-prod-1-eshop |
| Targets | Key targets (us-dev-1, us-prod-1, eu-prod-1) exist |

## AI Handoff

- AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Copyable prompts: [`prompts.md`](./prompts.md)
- Stable contracts: [`contracts.md`](./contracts.md)

## Cleanup

```bash
cd ../../promotion-demo-data
./cleanup.sh
```
