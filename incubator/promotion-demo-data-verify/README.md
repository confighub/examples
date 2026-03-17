# promotion-demo-data-verify

Verification script for the `promotion-demo-data` stable example.

## Who this is for

This is for **CI and AI workflows**, not human users.

If you're a human exploring the promotion UI, you don't need this — just run `setup.sh` and use the ConfigHub UI. The README in `promotion-demo-data/` has exploration commands if you want to query from the CLI.

This script exists because:
- **CI** needs explicit pass/fail to gate PRs
- **AI workflows** need structured assertions instead of "look at the output"

## Why this lives in incubator

The `promotion-demo-data/` example is a stable example outside the incubator. Adding verification scripts directly to it would mix stable content with incubator-style tooling.

This wrapper lets us verify the demo data without modifying the stable example.

## Usage

First, run the demo setup:

```bash
cd ../../promotion-demo-data
./setup.sh
```

Then verify:

```bash
cd ../incubator/promotion-demo-data-verify
./verify.sh
```

## What it checks

| Check | Description |
|-------|-------------|
| Space counts | 49+ demo spaces exist, including 8+ platform-owned and 12+ prod |
| Unit counts | 130+ units across app spaces |
| Label presence | Spaces queryable by App label |
| Version skew | us-prod-1-eshop has different image version than eu-prod-1-eshop |
| Targets | Key targets (us-dev-1, us-prod-1, eu-prod-1) exist |

## Cleanup

```bash
cd ../../promotion-demo-data
./cleanup.sh
```
