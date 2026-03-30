# AI Start Here

Use this page as the single AI-oriented handoff page for the current incubator work.

If you need the stricter protocol first, read [AGENTS.md](./AGENTS.md).
If you need the fuller incubator AI guide, read [AI-README-FIRST.md](./AI-README-FIRST.md).
If you need to explain ConfigHub to a new user first, read [WHY_CONFIGHUB.md](./WHY_CONFIGHUB.md).
If you need the current pacing standard, read [standard-ai-demo-pacing.md](./standard-ai-demo-pacing.md).

Default rule:

- start in read-only mode
- prefer JSON output
- only mutate ConfigHub when the human asks for that next step

## Route By Reason First

Before choosing an example, identify the user's reason:

| Reason | What they want | First choice |
|--------|----------------|--------------|
| **Import** | See what exists in Git, clusters, or controllers | [gitops-import-argo](./gitops-import-argo/README.md) or [gitops-import-flux](./gitops-import-flux/README.md) |
| **Mutate** | Make controlled changes through ConfigHub | [platform-write-api](./platform-write-api/README.md) |
| **Apply** | Deploy real workloads to real targets | [springboot-platform-app](../spring-platform/springboot-platform-app/README.md) or [global-app-layer/single-component](./global-app-layer/single-component/README.md) |
| **Model** | Represent layered or governed config | [global-app-layer](./global-app-layer/README.md) |

## Delivery Matrix

Know which delivery mode is in scope before making claims:

| Delivery Mode | Status | When to use |
|---------------|--------|-------------|
| **Direct Kubernetes** | Fully working | Simplest real proof. No controller required. |
| **Flux OCI** | Current standard | Controller-oriented delivery. Flux manages workload lifecycle. |
| **Argo OCI** | Implemented | Use only where the example explicitly wires it and you can show controller plus live evidence. |
| **Renderer-only (ArgoCDRenderer)** | Working, limited scope | Renderer/hydration path. Not the same as OCI delivery. |

Critical distinctions:

- `FluxOCI` is the current standard controller-oriented delivery path
- `ArgoCDRenderer` is **not** Argo OCI delivery — it is a renderer path that expects Argo `Application` payloads
- Argo OCI is implemented, but it should only be claimed with controller and live evidence
- Do not conflate renderer paths with OCI bundle delivery

## Reality Rules

- Use "real end-to-end" only when ConfigHub stores the config, the mutation is real, apply uses a non-`Noop` target, a real app or controller receives the change, and live verification proves the result.
- Do not offer `Noop` targets unless the human explicitly asks for them or agrees they are needed for a narrow proof.
- If an example is import-only, evidence-only, controller-layout only, offline, or `Noop`-based, say that explicitly before you run any mutating step.
- If the human asks for real ConfigHub apply end-to-end, lead with [`global-app-layer`](./global-app-layer/README.md) or [`springboot-platform-app`](../spring-platform/springboot-platform-app/README.md) (with `--with-targets`), not [`platform-write-api`](./platform-write-api/README.md).

## Standard Stories

When the human says "show me Argo" or "show me Flux", do not fan out across the whole catalog first.

- `Standard Argo story`: [`gitops-import-argo`](./gitops-import-argo/README.md), centered on the healthy guestbook applications. Start with the healthy Argo path. Only add the brownfield contrast fixtures on a second pass.
- `Standard Flux story`: [`gitops-import-flux`](./gitops-import-flux/README.md), centered on the healthy `podinfo` path. Start with `podinfo`. Only add the D2 contrast fixtures on a second pass.
- `5-10 minute bar`: by minute 10, the human should either see a healthy controller-owned app and the exact next ConfigHub import step, or already see ConfigHub discover/import evidence. If not, the story is not ready as the standard front door.

## Argo And Flux Name Map

- Argo import from GitHub: [`gitops-import-argo`](./gitops-import-argo/README.md)
- Flux import from GitHub, including podinfo and D2 contrast: [`gitops-import-flux`](./gitops-import-flux/README.md)
- Argo ApplicationSet app-style example: [`apptique-argo-applicationset`](./apptique-argo-applicationset/README.md)
- Argo app-of-apps example: [`apptique-argo-app-of-apps`](./apptique-argo-app-of-apps/README.md)
- Flux monorepo app-style example: [`apptique-flux-monorepo`](./apptique-flux-monorepo/README.md)
- Flux multi-service fan-out example: [`flux-boutique`](./flux-boutique/README.md)

## Detailed Reason Routing

If the human describes a more specific reason:

- `why ConfigHub should exist as a write API`: [`platform-write-api`](./platform-write-api/README.md)
- `one real app with apply here vs lift upstream vs block or escalate`: [`springboot-platform-app`](../spring-platform/springboot-platform-app/README.md)
- `smallest layered recipe proof`: [`global-app-layer/single-component`](./global-app-layer/single-component/README.md)
- `small app-level layered recipe`: [`global-app-layer/frontend-postgres`](./global-app-layer/frontend-postgres/README.md)
- `realistic layered app`: [`global-app-layer/realistic-app`](./global-app-layer/realistic-app/README.md)
- `NVIDIA-shaped layered stack`: [`global-app-layer/gpu-eks-h100-training`](./global-app-layer/gpu-eks-h100-training/README.md)
- `bounded-procedure design work rather than a runnable front door`: [`cub-proc`](./cub-proc/README.md)

## CRITICAL: Demo Pacing

When walking a human through any incubator example, you MUST pause after every stage.

After each stage:

1. run only that stage's commands
2. show the output faithfully; if it is long, keep the important section visible and do not replace it with a one-line summary
3. explain what the output means in plain English
4. print the GUI link or GUI checkpoint if one exists
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if no issue exists yet, say that explicitly
8. tell the human to open the GUI and give them time to inspect it
9. if there is no GUI checkpoint, say so explicitly
10. stop and ask `Ready to continue?`
11. do not move on until the human says to continue

Do not treat these examples like a checklist to finish quickly. Treat the AI as the demo UX.

When a stage has GUI relevance, prefer this shape:

- `GUI now:` exact URL or click path and what is visible today
- `GUI gap:` what the GUI cannot show yet
- `GUI feature ask:` what the GUI should show next, with issue number if known
- `PAUSE:` tell the human to open the GUI and inspect it before continuing

## Suggested Prompt For Humans

```text
Read incubator/<example>/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show the output clearly.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
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
- GUI now: the spaces or targets you just listed in the CLI
- GUI gap: the general org pages may still require manual scanning instead of a purpose-built filtered view
- GUI feature ask: a tighter CLI-to-GUI handoff with direct filtered views and better grouping; cite the issue number if one exists for the example you are running

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

Then use one standard story and keep the first pass narrow.

If the human wants the Argo import path, start with the healthy guestbook story, not the contrast fixtures:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

If the human wants the matching Flux import path, start with the healthy `podinfo` story, not the D2 contrast fixtures:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
```

Only add `--with-contrast` after the human has already seen value from the standard story. If they want the ConfigHub part immediately and have auth ready, continue with `--with-worker`, then `cub gitops discover` and `cub gitops import`.

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
