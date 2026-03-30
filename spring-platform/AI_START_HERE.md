# AI Start Here: Spring Platform Examples

This page helps AI assistants guide humans through the Spring platform examples.

Read the folder [`README.md`](./README.md) first. It explains the model. This page
explains how to demo it.

## Demo Pacing Rules

When walking a human through any Spring platform example:

1. Run only one stage's commands at a time
2. Print the full output (do not summarize)
3. Explain what the output means in plain English
4. If there is a GUI URL, print it
5. Ask "Ready to continue?" and wait for confirmation
6. Only proceed when the human says to continue

This is a demo, not a script execution.

## Choosing an Entry Point

| Human wants to understand | Use this example |
|---------------------------|------------------|
| Generator transformation and field lineage | [`springboot-platform-app`](./springboot-platform-app/AI_START_HERE.md) |
| One app across dev/stage/prod | [`springboot-platform-app-centric`](./springboot-platform-app-centric/AI_START_HERE.md) |
| One platform with multiple apps | [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/AI_START_HERE.md) |

## Suggested Prompts

### For generator and field lineage story

```text
Read spring-platform/springboot-platform-app/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Explain how app inputs and platform policy become operational config.
Do not continue until I say continue.
```

### For app/deployment/target story

```text
Read spring-platform/springboot-platform-app-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

### For platform-centric story

```text
Read spring-platform/springboot-platform-platform-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Explain what is platform-owned versus app-owned.
Do not continue until I say continue.
```

## What All Three Examples Share

All three examples demonstrate the same underlying model:

- Same generator story: app inputs + platform inputs → operational config
- Same mutation routes: apply-here, lift-upstream, block/escalate
- Same implementation status (see folder README for truth matrix)

The difference is the viewing angle, not the implementation.

## Reference

- Canonical pacing standard: [`../incubator/docs/ai-first-demo-standard.md`](../incubator/docs/ai-first-demo-standard.md)
- Longer demo-pacing guide: [`../incubator/standard-ai-demo-pacing.md`](../incubator/standard-ai-demo-pacing.md)
