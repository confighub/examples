# AI Start Here: redis-platform-with-rbac-guardrails

## CRITICAL: Demo Pacing

When walking a human through this example, pause after every stage.

After each stage:

1. Run only that stage's commands.
2. Show the output faithfully.
3. Explain what the output means in plain English.
4. Stop and ask `Ready to continue?`
5. Continue only after the human says to continue.

## Suggested Prompt

```text
Read redis-platform-with-rbac-guardrails/AI_START_HERE.md and walk me through
the demo. Pause after every stage. Show full output. Do not continue until I say
continue.
```

## What This Example Teaches

This example shows how Redis can be one piece of a larger `payments-platform`
product, while a small Go app manages RBAC across the whole product.

It answers the earlier design question:

- We are not adding the RBAC manager to Redis.
- We are not making a new Redis chart.
- We are modeling a larger payments product that includes Redis, a custom API,
  and RBAC guardrails.

This example is offline. It runs `cmd/payments-rbac` over a local fixture. It
does not mutate ConfigHub and it does not require a Kubernetes cluster.

## Stage 1: Preview The Plan

```bash
cd redis-platform-with-rbac-guardrails
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview.
- GUI gap: no public UI currently shows this app-plus-RBAC plan before setup.
- GUI feature ask: app example preview for component, variants, pieces, and
  RBAC findings.

Pause after this stage.

## Stage 2: Generate The Local RBAC View

```bash
./setup.sh
```

This writes local files under `sample-output/`.

What to explain:

- `cmd/payments-rbac` and `internal/platform` are the app code.
- `component-map.json` shows `payments-platform` variants and pieces.
- `snapshot.json` shows Redis, `payments-api`, and RBAC guardrail Units.
- `who-can-get-secrets-prod-us.json` answers the cross-piece RBAC question.
- `findings.json` names the issue.
- `proposed-edit.json` shows the dry-run hardening change.

GUI checkpoint:

- GUI now: none; this is a local fixture-backed example.
- GUI gap: no chart/app page yet renders these RBAC-manager outputs.
- GUI feature ask: ConfigHub app page card for RBAC findings and proposed
  guardrail edits.

Pause after this stage.

## Stage 3: Verify The Contract

```bash
./verify.sh
```

What to explain:

- The verifier checks the component name, variants, unit counts, who-can output,
  finding, and dry-run edit.
- A passing result means the example is internally consistent.
- It does not prove a live ConfigHub promotion or a live Kubernetes deployment.

GUI checkpoint:

- GUI now: none; verification is CLI-only.
- GUI gap: no green/red UI surface for this offline companion example.
- GUI feature ask: example health card for local proofs and known limits.

Pause after this stage.
