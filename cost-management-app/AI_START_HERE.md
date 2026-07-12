# AI Guide: cost-management-app

A generated ConfigHub operational app that finds config-derived waste across
an org, prices it against a declared rate card, and turns each finding into a
governed change: finding-owned dry run, exact local review, explicit execution
confirmation, whitelisted mutation, revision verification, and receipt. The
local review is unsigned `WATCH` evidence, not approval or permission. Like the
neighbouring add-on manager example in this repository, it is a UI app with a
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
expired or reused reviews, identity or org mismatch, whitelist bypass, and
mutation-diff mismatch). A write is successful only when the Unit advances by
exactly one revision and the actual mutations equal the reviewed dry run.

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

## Stage 4: Inspect the exact-review refusals

```bash
node cli.mjs preview --json
node cli.mjs review --json
node cli.mjs commit --json
```

Expected: preview without a finding returns `BLOCK` / `FINDING_REQUIRED`;
review reports `WATCH` / `LOCAL_REVIEW_NOT_RECORDED` and says plainly that a
local review grants no permission; commit without live bindings returns
`BLOCK` / `LIVE_BINDINGS_REQUIRED`. The whitelist is closed: only
`set-replicas` and `set-container-resources-defaults` can be selected, and the
selection must come from an actionable finding rather than hand-entered scope.

**PAUSE.** Wait for the human.

## Stage 5: Review and execute one finding (mutates ConfigHub)

First create deployment-local `data/live-bindings.json` from
`data/live-bindings.example.json`, replacing every review-authority placeholder
with the verified ConfigHub server, org identities, object URL, and action
endpoint. Runtime/controller fields may remain explicit `blocked:` gaps; they
do not become universal claims and they do not erase the real governed
ConfigHub write path.

Choose one actionable finding, preview it, and record who inspected that exact
diff:

```bash
node cli.mjs preview --finding <finding-id> --json
node cli.mjs review --record --preview <preview-id> --reason "<why>" --json
node cli.mjs commit --review <review-id> --json
```

Expected: review returns `WATCH` / `LOCAL_REVIEW_RECORDED`; it derives the
reviewer from the authenticated Cub context, expires after 15 minutes, and
still grants no mutation permission. Commit without confirmation returns
`ASK` / `EXECUTION_CONFIRMATION_REQUIRED` and names the exact scope.

Only after the human explicitly approves that review id and scope, run:

```bash
node cli.mjs commit --review <review-id> --confirm-execute --json
```

Expected: `PASS` / `CONFIG_REVISION_COMMITTED` only after the finding-owned
function runs, the Unit advances by exactly one revision, and the actual
mutation equals the reviewed dry run. The receipt under `data/receipts/` keeps
provider atomicity at `WATCH` and delivery evidence explicit until separately
verified. Reloading this local unsigned receipt reports `WATCH` /
`LOCAL_UNSIGNED_RECEIPT_RECORDED`; it is not a signed approval, fresh server
attestation, controller result, or live runtime proof. Re-run the sweep and
confirm the claimed savings reconcile with the executed finding.

**PAUSE.** This is the end of the loop: found, priced, reviewed, executed,
verified, receipted, reconciled.

## GUI

`CONFIGHUB_ORG=<org> npm run oauth:register` shows a read-only registration
confirmation card. Repeat it with `-- --confirm` to create or reuse the browser
OAuth client and record it in the fleet record
(`confighub/registry/fleet-record.json` ships unregistered). Then
`VITE_CONFIGHUB_BASE_URL=... VITE_OAUTH_CLIENT_ID=... npm run ui:dev` and open
the printed localhost address — serve on the exact origin you registered;
redirect URIs are matched exactly. The signed-in first panel is the findings
table, not governance chrome.
