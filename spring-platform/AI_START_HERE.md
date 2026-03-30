# AI Start Here: Spring Platform Examples

Use this page when you want an AI assistant to guide a human through the Spring
platform examples from the new `spring-platform/` location.

## CRITICAL: Demo pacing

When walking a human through any Spring platform example, you MUST pause after
every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen. If it is long, keep the important section visible and do not replace it with a one-line summary
3. Explain what the output means in plain English
4. If there is a GUI URL or click path, print it
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

This is a demo, not a race to the end.

## Start With The Canonical Standards

Before driving one of these examples, read:

- [`../incubator/docs/ai-first-demo-standard.md`](../incubator/docs/ai-first-demo-standard.md)
- [`../incubator/standard-ai-demo-pacing.md`](../incubator/standard-ai-demo-pacing.md)

Those two docs define the pacing rules and the AI-first demo expectations for
the whole examples repo.

## Then Pick One Spring Entry Point

### 1. App-centric front door

Use:

- [`springboot-platform-app-centric/AI_START_HERE.md`](./springboot-platform-app-centric/AI_START_HERE.md)

Use this when the human wants:

- one app
- three deployments
- clear mutation outcomes
- the easiest story-first path

Suggested prompt:

```text
Read spring-platform/springboot-platform-app-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

### 2. Generator and authority story

Use:

- [`springboot-platform-app/AI_START_HERE.md`](./springboot-platform-app/AI_START_HERE.md)

Use this when the human wants:

- the generator model
- provenance and field lineage
- ConfigHub as the authority and governance layer

Suggested prompt:

```text
Read spring-platform/springboot-platform-app/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Explain how app inputs and platform policy become operational config.
Do not continue until I say continue.
```

### 3. Platform-centric view

Use:

- [`springboot-platform-platform-centric/AI_START_HERE.md`](./springboot-platform-platform-centric/AI_START_HERE.md)

Use this when the human wants:

- shared platform contracts
- multiple apps on one platform
- the platform -> apps -> deployments mental model

Suggested prompt:

```text
Read spring-platform/springboot-platform-platform-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Explain what is platform-owned versus app-owned.
Do not continue until I say continue.
```
