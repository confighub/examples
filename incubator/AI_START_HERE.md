# AI Start Here

Use this page as the single AI-oriented handoff page for the current incubator work.

If you need the stricter protocol first, read [AGENTS.md](./AGENTS.md).
If you need the fuller incubator AI guide, read [AI-README-FIRST.md](./AI-README-FIRST.md).

Default rule:

- start in read-only mode
- prefer JSON output
- only mutate ConfigHub when the human asks for that next step

## 0) Prerequisites

```bash
cd <your-examples-checkout>
export CONFIGHUB_AGENT=1
```

## 1) Read-Only First

Start by inspecting the repo without mutating ConfigHub:

```bash
git rev-parse --show-toplevel
./scripts/verify.sh
rg --files incubator
```

If the human wants connected read-only inspection:

```bash
cub auth login
cub space list --json
cub target list --space "*" --json
cd incubator/global-app-layer
./find-runs.sh --json | jq
```

What these commands do not mutate:

- they do not create spaces or units
- they do not write config data
- they do not apply to a cluster

## 2) Stable machine-readable commands

Preferred contracts for AI use:

| Command | Output contract | Mutates anything? |
|---|---|---|
| `cub space list --json` | JSON array of spaces | no |
| `cub target list --space "*" --json` | JSON array of targets | no |
| `cub unit get --space <space> --json <unit>` | JSON object for one unit | no |
| `cub function do --dry-run --json ...` | JSON invocation response | no config write |
| `cub unit apply --dry-run --json ...` | JSON apply preview | no live apply |

If you want to run against a live target, have one ready in your active space.

Quick connected check:

```bash
cub target list --json
```

To discover currently active layered-example runs without knowing the prefix:

```bash
cd incubator/global-app-layer
./find-runs.sh
./find-runs.sh realistic-app --json | jq
```

## 3) Recommended first mutating path

Start with the realistic layered app example:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

If you want to wire a real target immediately:

```bash
./setup.sh <prefix> <space/target>
./verify.sh
```

## 4) Quick demo data (no cluster required)

For exploring ConfigHub's promotion UI without a live target:

```bash
cd promotion-demo-data
./setup.sh
./cleanup.sh
```

For CI/AI verification, see [promotion-demo-data-verify](./promotion-demo-data-verify/).

This creates 49 spaces and ~154 units using the **App-Deployment-Target** model:

- **App** → label + units (e.g., `aichat`, `eshop`, `platform`)
- **Target** → infra space with target object (e.g., `us-prod-1`)
- **Deployment** → `{target}-{app}` space (e.g., `us-prod-1-eshop`)

Uses the noop bridge, so no Kubernetes cluster is needed. This is the canonical multi-env model for ConfigHub.

## 5) Smaller and larger options

Smallest:

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh
```

GPU-flavored:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

## 6) What success looks like

You should be able to see:

- the setup plan before any mutation
- explicit spaces and units created
- variant-chain structure preserved
- recipe manifest materialized
- verification passing against the created ConfigHub objects

## 7) Tiny direct vs delegated fixtures

If you need the smallest possible direct and delegated apply inputs for design work around `cub run`, use:

- [cub-run-fixtures](./cub-run-fixtures/README.md)

These are preserved reference fixtures, not the main walkthrough.

## 8) Related Pages

- Repo-level AI path: [../AI_START_HERE.md](../AI_START_HERE.md)
- Incubator AI protocol: [AGENTS.md](./AGENTS.md)
- Incubator AI guide: [AI-README-FIRST.md](./AI-README-FIRST.md)
- Reusable example playbook: [ai-example-playbook.md](./ai-example-playbook.md)
- Reusable example template: [ai-example-template.md](./ai-example-template.md)
- Start guide: [global-app-layer/README.md](./global-app-layer/README.md)
- How it works: [global-app-layer/how-it-works.md](./global-app-layer/how-it-works.md)
