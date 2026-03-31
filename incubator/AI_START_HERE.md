# AI Start Here

AI-oriented entry point for the incubator.

For stricter protocol: [AGENTS.md](./AGENTS.md)
For concept explanation: [WHY_CONFIGHUB.md](./WHY_CONFIGHUB.md)
For pacing standard: [standard-ai-demo-pacing.md](./standard-ai-demo-pacing.md)

## Default Rules

1. Start in read-only mode
2. Prefer JSON output (`--json`, `--explain-json`)
3. Pause after every stage
4. Only mutate when the human asks

## Route By Reason

| Reason | First choice |
|--------|--------------|
| **Import** | [gitops-import-argo](./gitops-import-argo/AI_START_HERE.md) or [gitops-import-flux](./gitops-import-flux/AI_START_HERE.md) |
| **Mutate** | [platform-write-api](./platform-write-api/AI_START_HERE.md) |
| **Apply** | [global-app-layer/single-component](./global-app-layer/single-component/AI_START_HERE.md) |
| **Model** | [global-app-layer](./global-app-layer/AI_START_HERE.md) |

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
