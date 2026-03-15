# Examples Incubator

This directory is for experimental examples before promotion to stable examples.

## Fast Entry

- AI-led landing page: [CLAUDE_LANDING.md](./CLAUDE_LANDING.md)
- Start guide: [START_HERE.md](./START_HERE.md)
- Persona quickstart: [PERSONA_QUICKSTART.md](./PERSONA_QUICKSTART.md)

## Current Experiments

- [`cub-up`](./cub-up/README.md): one-command app/platform bundles with assert + GUI checkpoints.

## Run Modes for `cub-up`

```bash
# Human-led
./scripts/cub-up-human-flow.sh app ./incubator/cub-up/global-app dev <existing-target>

# AI-led
./scripts/cub-up-ai-flow.sh app ./incubator/cub-up/global-app dev <existing-target>

# Human + AI pair
./scripts/cub-up-pair-flow.sh app ./incubator/cub-up/global-app dev <existing-target>
```

| Mode | Reuse existing? | Stale resource? | Interactive? |
|---|---|---|---|
| **Human** | prompt (TTY) / fresh | prompt (TTY) / fresh | Yes if TTY |
| **AI** | always fresh | always fresh | No |
| **Pair** | prompt the human | prompt the human | Yes |

Call chain: `pair → ai → human` (each sets env vars and delegates down).

## Current Experiments

- [`cub-up`](./cub-up/README.md): one-command app/platform bundles with assert + GUI checkpoints.
- [`global-app-layer`](./global-app-layer/README.md): recipes and layers package with specs plus four worked recipe examples, including a GPU-flavored chain.

## Purpose

- Keep stable examples easy to trust and review.
- Iterate quickly on UX and AI flow ideas.
- Promote only after clear validation.

## Rules

- Keep changes additive and easy to diff.
- Include verification commands.
- Do not break existing stable examples.
