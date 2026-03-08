# ConfigHub Examples: Claude Landing Page

Use this page as the single entrypoint for an AI-led end-to-end demo.

## Goal

Run one `cub-up` scenario with:

- full CLI output
- `--assert` state transitions
- `--open-ui` GUI checkpoints
- stale-safe demo defaults

## 0) Prerequisites

```bash
cd <your-examples-checkout>   # e.g. ~/src/examples
cub auth login
./scripts/verify.sh
```

You need one ready target in your active space.

Quick check:

```bash
cub target list --no-header
```

## 1) Run AI-led flow (recommended)

Arguments: `<kind>` `<bundle-path>` `<env>` `<target>`

```bash
CUB_UP_ON_EXISTS=fresh CUB_UP_STALE_ACTION=fresh \
./scripts/cub-up-ai-flow.sh \
  app \                              # kind
  ./incubator/cub-up/global-app \    # bundle path
  dev \                              # environment
  my-target                          # target slug from `cub target list`
```

## 2) Success Criteria

You should see:

- `PREFLIGHT` lines showing `unit -> target -> worker`
- `ASSERT RESULT: PASS` after each major step
- `GUI (...)` URLs for space, targets, and units
- final line: `Completed: cub up app`

## 3) Pair Mode (optional)

Use pair mode when you want interactive prompts for reuse/fresh decisions
instead of the fully automated AI-led flow.

```bash
./scripts/cub-up-pair-flow.sh \
  app \
  ./incubator/cub-up/global-app \
  dev \
  my-target
```

## 4) Troubleshooting

- Not authenticated: run `cub auth login`
- Target missing: pass a valid target slug
- Worker not ready: preflight should fail fast with worker condition
- Stale confusion: keep `CUB_UP_ON_EXISTS=fresh CUB_UP_STALE_ACTION=fresh`

## 5) Related Pages

- Start guide: [START_HERE.md](./START_HERE.md)
- Personas: [PERSONA_QUICKSTART.md](./PERSONA_QUICKSTART.md)
- Bundles: [cub-up/README.md](./incubator/cub-up/README.md)
