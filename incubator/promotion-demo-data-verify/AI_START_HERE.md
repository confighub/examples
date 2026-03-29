# AI Start Here

## CRITICAL: Verification pacing

Treat this as a verification walkthrough, not a one-line CI check.

After each stage:

1. say what the next command reads
2. say whether it mutates anything
3. show the result clearly
4. explain what the result proves
5. point at the matching GUI checkpoint when one exists
6. stop and ask `Ready to continue?`

## Suggested prompt

```text
Read incubator/promotion-demo-data-verify/AI_START_HERE.md and walk me through the verification.
Pause after every stage. Show the important output clearly.
Tell me what the result proves, what it does not prove, and what I can inspect in the GUI.
Do not continue until I say continue.
```

## What this wrapper is for

This wrapper verifies the stable `promotion-demo-data` example after setup. It is for AI and CI workflows that need explicit checks, not just “open the UI and look around.”

## Stage 1: Preview the verification plan (read-only)

Run:

```bash
./verify.sh --explain
./verify.sh --explain-json | jq
```

Explain:

- this wrapper only reads ConfigHub state
- it depends on `../../promotion-demo-data`
- it will check counts, labels, version skew, and key targets

GUI now: nothing special yet; this stage is only about the verification plan.

GUI gap: there is no built-in checklist view that tells you exactly what the stable demo should contain.

GUI feature ask: example-level verification checklist linked from the stable example. No issue filed yet.

PAUSE.

## Stage 2: Create the stable demo data (mutates ConfigHub only)

Ask:

`This will create the stable promotion demo data in ConfigHub. OK?`

Run:

```bash
cd ../../promotion-demo-data
./setup.sh
```

Explain:

- this mutates ConfigHub state only
- it does not write Git
- it uses Noop targets, so it does not deploy to a live cluster

GUI now: open ConfigHub and inspect the generated spaces, especially `demo-infra`, `us-prod-1-eshop`, and `eu-prod-1-eshop`.

GUI gap: there is no one-click “show me only this example's spaces” landing page.

GUI feature ask: example-scoped filtered landing page for demo datasets. No issue filed yet.

PAUSE.

## Stage 3: Run the verification wrapper (read-only)

Run:

```bash
cd ../incubator/promotion-demo-data-verify
./verify.sh
./verify.sh --json | jq
```

Explain:

- the text mode is good for humans
- the JSON mode is the stable AI/CI contract
- passing checks mean the stable demo still has its expected shape

GUI now: inspect the spaces list and compare it to the verification counts.

GUI gap: the GUI does not summarize “49+ spaces, 130+ units, 7 targets” as one health card for the example.

GUI feature ask: example health summary card with counts and known invariants. No issue filed yet.

PAUSE.

## Stage 4: Spot-check the high-signal invariants (read-only)

Show the human the most important checks from the output:

- total demo spaces
- app label query for `aichat`
- version skew between `us-prod-1-eshop` and `eu-prod-1-eshop`
- existence of the key targets

Explain:

- these checks prove the dataset still supports the promotion UI story
- they do not prove every single unit is semantically correct
- this is a structural verification layer, not a full e2e product test

GUI now: open `us-prod-1-eshop` and `eu-prod-1-eshop` in the UI and compare their `api` units.

GUI gap: there is no built-in invariant view that explains that the image skew is intentional demo data.

GUI feature ask: attach “expected divergence” notes to example data. No issue filed yet.

PAUSE.

## Stage 5: Cleanup

Run:

```bash
cd ../../promotion-demo-data
./cleanup.sh
```

Explain:

- cleanup belongs to the stable example, not the wrapper
- this removes the demo dataset from ConfigHub
- there is no extra incubator-local cleanup step
