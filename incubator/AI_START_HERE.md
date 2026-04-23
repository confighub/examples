# AI Start Here

AI-oriented entry point for the incubator.

For stricter protocol: [AGENTS.md](./AGENTS.md)
For concept explanation: [WHY_CONFIGHUB.md](./WHY_CONFIGHUB.md)
For pacing standard: [standard-ai-demo-pacing.md](./standard-ai-demo-pacing.md)
For enforced contract: [ai-guide-standard.md](./ai-guide-standard.md)
For shared read-only evaluation flow: [ai-machine-seams-first.md](./ai-machine-seams-first.md)
For reusable prompts: [ai-cold-eval-prompt-pack.md](./ai-cold-eval-prompt-pack.md)

## Default Rules

1. Start in read-only mode
2. Prefer JSON output (`--json`, `--explain-json`)
3. Pause after every stage
4. Only mutate when the human asks

For runnable examples, prefer the shared machine seams first:

- `./setup.sh --explain`
- `./setup.sh --explain-json`
- `./verify.sh --json`

## Route By Reason

| Reason | First choice |
|--------|--------------|
| **Import** | [gitops-import-argo](./gitops-import-argo/AI_START_HERE.md) or [gitops-import-flux](./gitops-import-flux/AI_START_HERE.md) |
| **Mutate** | [platform-write-api](./platform-write-api/AI_START_HERE.md) |
| **Apply** | [global-app-layer/single-component](./global-app-layer/single-component/AI_START_HERE.md) |
| **Model** | [global-app-layer](./global-app-layer/AI_START_HERE.md) |
| **Train handoff discipline** | [mini-kubara](./mini-kubara/AI_START_HERE.md) |

## CRITICAL: Demo Pacing

When walking a human through any incubator example, you MUST pause after every stage.

After each stage:

1. Run only that stage's commands
2. Show full output (do not summarize)
3. Explain in plain English
4. Print GUI checkpoint if one exists
5. Say what the GUI shows today
6. Say what the GUI does not show yet
7. Name the GUI feature ask with issue number (or say "No issue filed yet")
8. Stop and ask "Ready to continue?"
9. Wait for the human

## Suggested Prompt

```text
Read incubator/<example>/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Do not continue until I say continue.
```

## Stage 1: Read-Only Orientation

```bash
git rev-parse --show-toplevel
./scripts/verify.sh
cub version
cub context list --json | jq
```

GUI now: None yet; this stage is CLI-only.

GUI gap: No dashboard showing auth status and available targets at a glance.

GUI feature ask: Auth status widget on landing page. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Safe Commands

| Command | Mutates? |
|---------|----------|
| `./setup.sh --explain` | No |
| `./setup.sh --explain-json` | No |
| `cub space list --json` | No |
| `cub target list --space "*" --json` | No |
| `cub unit get --space <space> --json <unit>` | No |
| `./find-runs.sh --json` | No |

## Fast Preview For `global-app-layer`

For the reusable evaluation protocol, start with [ai-machine-seams-first.md](./ai-machine-seams-first.md).

For the shortest AI-safe preview path in the incubator:

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq
cd realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

Middle-sized layered app preview:

```bash
cd incubator/global-app-layer/frontend-postgres
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh --json
```

After a real `./setup.sh`, prefer:

```bash
./verify.sh
./verify.sh --json
```

## Delivery Matrix

| Mode | Status |
|------|--------|
| **Direct Kubernetes** | Fully working |
| **Flux OCI** | Current standard |
| **Argo OCI** | Implemented (needs live evidence) |
| **ArgoCDRenderer** | Renderer path only |

## CLI Gotchas

- Use `cub version`, not `cub --version`
- Use `cub context list`, not `cub context current`
- Check `cub ... --help` before assuming subcommands

## 2) Quick demo data (no cluster required)

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

## 3) Smaller and larger options

Smallest:

```bash
cd incubator/global-app-layer/single-component
./setup.sh --explain
./setup.sh --explain-json
./setup.sh
./verify.sh
./verify.sh --json
```

GPU-flavored:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

## 4) What success looks like

You should be able to see:

- explicit spaces and units created
- clone-chain structure preserved
- recipe manifest materialized
- verification passing against the created ConfigHub objects
- read-only explanation available before mutation

## 5) Tiny direct vs delegated fixtures

If you need the smallest possible direct and delegated apply inputs for design work around `cub run`, use:

- [cub-run-fixtures](./cub-run-fixtures/README.md)

These are preserved reference fixtures, not the main walkthrough.

## 6) Related Pages

- Start guide: [global-app-layer/README.md](./global-app-layer/README.md)
- How it works: [global-app-layer/how-it-works.md](./global-app-layer/how-it-works.md)
- Shared machine seams: [ai-machine-seams-first.md](./ai-machine-seams-first.md)
- Shared prompt pack: [ai-cold-eval-prompt-pack.md](./ai-cold-eval-prompt-pack.md)
