# AI Start Here

This is the AI-assistant entry point for the `confighub/examples` repo.

Default rule:

- start in read-only mode
- prefer machine-readable output
- do not mutate ConfigHub or a cluster until the human explicitly wants that next step

The exact commands below are chosen so an assistant can inspect the repo and, when authenticated, inspect ConfigHub safely.

## 0. Agent Mode

For `cub` help and discovery:

```bash
export CONFIGHUB_AGENT=1
cub --help-overview
```

## 1. Repo-Local Read-Only Path

These commands do not mutate ConfigHub, the repo, or a cluster.

```bash
cd <your-examples-checkout>
./scripts/verify.sh
rg --files .
```

What they do not mutate:

- ConfigHub spaces, units, workers, targets, bundles
- Kubernetes clusters
- Git history

## 2. Connected Read-Only Path

If the human wants live inspection, authenticate first:

```bash
cub auth login
```

Then prefer JSON output:

```bash
cub space list --json
cub target list --space "*" --json
```

If you already know a specific space and unit:

```bash
cub unit get --space <space> --json <unit>
```

What these commands do not mutate:

- they do not create or change spaces
- they do not change unit data
- they do not apply manifests
- they do not touch cluster live state

## 3. Machine-Readable Contracts

Use these commands as the default contracts for AI inspection:

| Command | Output contract | Mutates anything? |
|---|---|---|
| `./scripts/verify.sh` | exit code + plain text verifier output | no |
| `cub space list --json` | JSON array of space objects | no |
| `cub target list --space "*" --json` | JSON array of target objects | no |
| `cub unit get --space <space> --json <unit>` | JSON object for one unit | no |
| `cub unit apply --dry-run --json ...` | JSON preview response for apply | no live apply |
| `cub function do --dry-run --json ...` | JSON invocation response without writing config data | no config write |

Notes:

- prefer `--json` first
- use `--jq` only when you need a narrower machine-readable projection
- use `--dry-run` before any mutating apply/function path

## 4. Recommended First Inspection Targets

For stable, no-cluster review:

```bash
cd promotion-demo-data
```

For layered recipes and NVIDIA AICR mapping:

```bash
cd incubator/global-app-layer
```

Read in this order:

1. `00-config-hub-hello-world.md`
2. `README.md`
3. `confighub-aicr-value-add.md`
4. `how-it-works.md`

## 5. What To Say Before Mutating

Before running any mutating command, state clearly:

- what will be created or changed
- which space or unit will be touched
- whether the cluster or target will be touched
- what can be inspected first in dry-run or read-only mode

## 6. Next Step Into Connected Mode

Once the human wants a real flow, the best next steps are:

1. `promotion-demo-data` for a stable multi-environment ConfigHub walkthrough
2. `incubator/global-app-layer/00-config-hub-hello-world.md` for one-space, one-unit connected basics
3. `incubator/global-app-layer/realistic-app` for a fuller layered recipe flow

For the incubator-specific AI path, use:

- [`incubator/AI_START_HERE.md`](./incubator/AI_START_HERE.md)
