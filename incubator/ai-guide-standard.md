# AI Guide Standard

This doc explains the enforced contract for AI guides. The verifier (`scripts/verify.sh`) currently covers 30 examples: 26 incubator + 4 stable.

For the machine-readable contract (`--explain-json`, `contracts.md`), see [EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md).

## Required Files

Every runnable example must have:

| File | Purpose |
|------|---------|
| `README.md` | Human-first orientation |
| `AI_START_HERE.md` | AI assistant entry point |
| `contracts.md` | Stable inspection paths |
| `setup.sh` | Must support `--explain` and `--explain-json` |

## Required AI Guide Sections

`AI_START_HERE.md` must contain these markers (enforced by verifier):

| Marker | Purpose |
|--------|---------|
| `## CRITICAL: Demo Pacing` | Tells AI to pause after every stage |
| `## Suggested Prompt` | Copyable prompt for users |
| `## Stage N:` or `### Stage N:` | At least one numbered stage |
| `GUI gap:` | What the GUI cannot show yet |
| `GUI feature ask:` or `GUI ask:` | What the GUI should add |

Every stage must end with `**PAUSE.** Wait for the human.`

## Why This Matters

1. **Safety** — Explicit pauses prevent runaway automation
2. **Clarity** — Staged structure makes output understandable
3. **Feedback** — GUI gap markers capture product requests

## Verifier

The verifier runs automatically and checks all examples listed in `scripts/verify.sh`:

```bash
./scripts/verify.sh
```

To add a new example, add its path to the `ai_guide_examples` array.

## Exemptions

Some examples are exempt from `contracts.md` or `--explain` requirements:

```bash
exempt_from_contracts=(
  "${repo_root}/incubator/watch-webhook"
  "${repo_root}/initiatives-demo"
  "${repo_root}/promotion-demo-data"
)

exempt_from_explain=(
  "${repo_root}/initiatives-demo"
  "${repo_root}/promotion-demo-data"
)
```

Keep exemptions small. See [EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for exemption criteria.

## Related

- [ai-example-template.md](./ai-example-template.md) — Full template to copy
- [ai-example-playbook.md](./ai-example-playbook.md) — 5 rules for demo pacing
- [scripts/verify.sh](../scripts/verify.sh) — Enforcement script
