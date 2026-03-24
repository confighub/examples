# AI Start Here

Use this page as the single AI-oriented handoff page for the current incubator work.

If you need the stricter protocol first, read [AGENTS.md](./AGENTS.md).
If you need the fuller incubator AI guide, read [AI-README-FIRST.md](./AI-README-FIRST.md).

Default rule:

- start in read-only mode
- prefer JSON output
- only mutate ConfigHub when the human asks for that next step

## CRITICAL: Demo Pacing

When walking a human through any incubator example, you MUST pause after every stage.

After each stage:

1. run only that stage's commands
2. print the full output; do not replace it with a one-line summary
3. explain what the output means in plain English
4. print the GUI link or GUI checkpoint if one exists
5. if there is no GUI checkpoint, say so explicitly
6. stop and ask `Ready to continue?`
7. do not move on until the human says to continue

Do not treat these examples like a checklist to finish quickly. Treat the AI as the demo UX.

## Suggested Prompt For Humans

```text
Read incubator/<example>/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## 0) Prerequisites

```bash
cd <your-examples-checkout>
export CONFIGHUB_AGENT=1
```

## 1) Stage 1: Read-Only Orientation

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

GUI checkpoint:

- for connected ConfigHub state, open the relevant org in ConfigHub and compare the listed spaces or targets to the CLI output

Pause after this stage.

## 2) Stable Machine-Readable Commands

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

## 3) Recommended Starting Paths

Pick one stage-based example path and stay within it until the human is ready to continue.

### Path A: GitOps import wedge

Start with the published docs for the overall story:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)

Then use the runnable incubator examples here.

If the human wants the Argo import path, start here:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
```

If the human wants the matching Flux import path, start here:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
```

### Path B: No-cluster import and evidence

For offline import proposal generation:

```bash
cd incubator/import-from-bundle
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

For a live-cluster dry-run proposal before any ConfigHub mutation:

```bash
cd incubator/import-from-live
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

For multi-cluster aggregation from two existing imports:

```bash
cd incubator/fleet-import
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

### Path C: Scan, risk, and reporting

For a scan-first ADT-style example:

```bash
cd incubator/demo-data-adt
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

For Helm-to-Argo lifecycle hazards:

```bash
cd incubator/lifecycle-hazards
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

For stored summary reporting and dry-run Slack output:

```bash
cd incubator/connected-summary-storage
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

For offline bundle inspection and replay:

```bash
cd incubator/artifact-workflow
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```
