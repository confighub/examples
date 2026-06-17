# AI Start Here: ctrlplane-on-confighub

This example maps a [Ctrlplane](https://ctrlplane.dev) **System** bundle onto a
ConfigHub governed-app plan. The default path is **read-only** — it creates no
ConfigHub or live state.

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen. If it is long, keep the important
   section visible and do not replace it with a one-line summary
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?"
6. Only proceed when the human says to continue

This is a demo, not a script execution. The value is in understanding the
mapping between the two products.

## Suggested Prompt

```text
Read ctrlplane/AI_START_HERE.md and walk me through it.
Pause after every stage. Show full output. This is read-only — do not create any
ConfigHub objects. For each stage, tell me what maps cleanly and where the two
models do not line up.
```

## What This Example Teaches

After this demo a human understands how a Ctrlplane "app" (System / Deployment /
Environment / Resource / JobAgent / Policy) lines up with a ConfigHub governed
app (app / Unit / Space / Target / delivery strategy / approval gate), and
exactly where the two models diverge — config data, verification, and rollout
timing.

## Prerequisites

- `python3` with `PyYAML` (`pip install pyyaml`)
- `jq` for JSON preview
- `cub` in PATH only if the human later runs the generated commands by hand

---

## Stage 1: "See the mapping" (read-only)

Run:

```bash
cd ctrlplane
./setup.sh --explain
```

What to explain:

- The conceptual model table: System→app, Deployment→base Unit (upstream),
  Environment→Space + downstream variant, Resource→Target,
  JobAgent→delivery strategy, Policy→gate
- 3 spaces (a base/upstream space + staging + production), 3 units (one base
  `checkout-api`, plus a staging and a production variant linked upstream),
  3 targets
- The production space is tagged `[requires approval]` from the Ctrlplane Policy
- `confighub-oci-argo` is chosen because the JobAgent type is `argocd`
- Promotion is shown as `cub unit update <variant> --upgrade` (pull from base)

GUI now: nothing to show yet — this stage creates no ConfigHub objects.

GUI gap: the GUI cannot preview "here is what this external Ctrlplane System
would become as ConfigHub spaces/units" before anything is created.

GUI feature ask: a read-only "import preview" panel that renders a proposed
space/unit/target tree from an uploaded System bundle. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Read the machine plan" (read-only)

Run:

```bash
./setup.sh --explain-json | jq '{mutates, spaces, units, targets, delivery_strategy, mapping_notes}'
```

What to explain:

- `"mutates": false` — this is a plan, not an action
- `release_targets_preview` is Deployment × Environment × Resource, exactly how
  Ctrlplane fans out
- `mapping_notes` lists the structural seams (next stage)

GUI now: not applicable (read-only plan).

GUI gap: there is no GUI surface that ingests this View-Packet-shaped JSON and
offers a one-click governed-app creation.

GUI feature ask: accept a mapping View Packet and render an approve-to-create
flow. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Understand the seams" (read-only)

Run:

```bash
./setup.sh --cub-commands
```

What to explain:

- The base Unit takes a supplied manifest (**Ctrlplane carries only an image
  reference**); the environment variants are created with `--upstream-unit` and
  inherit it, carrying only env-local overrides
- Promotion appears as `cub unit update <variant> --upgrade` — and the comment
  warns to **diff** afterward, because `--upgrade` can under-propagate
  list/nested fields
- Verification policies (Datadog/Prometheus) map to post-apply external checks,
  not ConfigHub primitives
- Rollout timing stays in Ctrlplane; ConfigHub governs and proves each step

GUI now: not applicable.

GUI gap: the GUI does not show "this Unit still needs config data before it can
be created" as a first-class precondition.

GUI feature ask: surface unmet preconditions (missing manifest, unbound target)
on a proposed-unit card. No issue filed yet.

**PAUSE.** This is the end of the read-only walkthrough. The live create/apply
path is the next proof step and is not yet verified end-to-end.
