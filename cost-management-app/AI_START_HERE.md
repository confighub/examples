# AI Guide: cost-management-app

A generated ConfigHub operational app that finds config-derived waste across
an org, prices it against a declared rate card, and turns each finding into a
governed change: single-use approval, whitelisted mutation, revision
verification, receipt. Like the neighbouring
add-on manager example in this repository, it is a UI app with a
zero-dependency CLI sibling and no `setup.sh` — the early stages run on a
cold clone with no install and no ConfigHub account.

The neighbouring [`../cost-estimator`](../cost-estimator) is the enforcement
plane of the same problem: a price book and an apply gate that block
over-budget changes before they ship. This example is the reduction plane: it
finds money already leaking and claws it back through governed mutations. The
two are complementary, not competing.

## CRITICAL: Demo Pacing

Run ONE stage at a time. After each stage, STOP and wait for the human before
continuing. Do not run ahead, even if the next command is obvious. The human
may want to inspect the output or the GUI between stages.

## Suggested Prompt

> Walk me through the cost-management-app example stage by stage. Pause after
> each stage so I can look around. Start with the offline checks — I have not
> connected ConfigHub yet.

## Stage 1: Prove the package on a cold clone

No `npm install` needed — the tests, verifier, CLI, engine, and executor use
node builtins only.

```bash
npm test                         # engine honesty rules and executor refusals included
npm run verify                   # layout + content hygiene, then the tests
node cli.mjs preflight --json    # readiness before any risk
```

Expected: tests pass, and preflight returns `"verdict": "WATCH"` with
`"reason": "LIVE_BINDINGS_MISSING"` — the correct safe default before any live
binding exists. Two suites matter most here: `tests/cost-engine.test.mjs`
proves the pricing honesty rules (limits are exposure and never savings; an
unbound Unit gets no runtime cost claim; every figure carries its rate basis),
and `tests/executor.test.mjs` proves the write-path refusals (revision drift,
approval reuse, whitelist bypass, and the silent-skip guard: a mutation that
reports success without a new revision is a typed BLOCK).

**PAUSE.** Wait for the human.

## Stage 2: Read the findings surface with no data

```bash
node cli.mjs findings --json
```

Expected: a `COST_SWEEP_NOT_RUN` row telling you to run the sweep, plus the
binding blocker and stop rules. The app never renders invented numbers; no
sweep means no findings.

**PAUSE.** Wait for the human.

## Stage 3: Sweep a live org (read-only)

Needs a ConfigHub session (`cub auth status`). This stage reads; it changes
nothing.

```bash
npm run cost:sweep
node cli.mjs findings
```

Expected: `COST_SWEEP_COMPLETE` with totals — containers scanned, containers
missing requests, configured monthly request cost, and claimed monthly savings
computed only from Units actually bound to Targets. Findings arrive ranked,
each carrying a governed dry-run preview command. Adjust
`data/rate-card.json` to your blended rates; every figure names the basis it
was priced on.

**PAUSE.** Wait for the human.

## Stage 4: Walk the write path to its refusals

```bash
node cli.mjs approve --json
node cli.mjs commit --json
```

Expected: approve without `--grant` previews the scope model; commit without
an approval returns `BLOCK` / `APPROVAL_REQUIRED`. Granting against a made-up
Unit fails on the head-revision read. The whitelist is closed: only
`set-replicas` and `set-container-resources-defaults` can ever be granted,
because those are the functions proven by receipted live executions.

**PAUSE.** Wait for the human.

## Stage 5: Execute one approved change (mutates ConfigHub)

Only with the human's explicit go, against a finding they chose:

```bash
node cli.mjs approve --grant --space <space> --unit <unit> \
  --function set-replicas --args 1 --actor <who> --reason "<finding>" --json
node cli.mjs commit --approval <id> --json
```

Expected: `MUTATION_COMMITTED` with the revision pair, a receipt under
`data/receipts/`, and an honest `deliveryEvidence` field that stays
`blocked:` until a runtime controller is bound — a config revision is never
presented as a delivered change. Re-run the sweep and watch the totals
reconcile: the claimed savings drop by exactly the executed finding.

**PAUSE.** This is the end of the loop: found, priced, approved, executed,
verified, receipted, reconciled.

## GUI

`npm run oauth:register` creates a browser OAuth client for your org and
records it in the fleet record (`confighub/registry/fleet-record.json` ships
unregistered; registration is a recorded state transition). Then
`VITE_CONFIGHUB_BASE_URL=... VITE_OAUTH_CLIENT_ID=... npm run ui:dev` and open
the printed localhost address — serve on the exact origin you registered;
redirect URIs are matched exactly. The signed-in first panel is the findings
table, not governance chrome.
