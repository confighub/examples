# Incubator AI Example Template

Use this as a starter for a major incubator example.

## Recommended File Bundle

```text
README.md
AI_START_HERE.md
prompts.md
contracts.md
setup.sh
verify.sh
cleanup.sh
```

For runnable examples, `setup.sh` must support:
- `--explain` (human-readable plan)
- `--explain-json` (machine-readable plan)

---

## `README.md` Template

Suggested sections:

```md
# Example Name

## Stack And Scenario

## What This Proves

## Prerequisites

## What This Reads And Writes

## Read-Only Preview

## Run It

## Expected Output

## Verify It

## Inspect It In The GUI

## Troubleshooting

## Cleanup
```

---

## `AI_START_HERE.md` Template

This is the critical file. Copy and adapt this structure:

```md
# AI Start Here: <Example Name>

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen. If it is long, keep the important section visible and do not replace it with a one-line summary
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

This is a demo, not a script execution. The value is in understanding each step.

## Suggested Prompt

```text
Read incubator/<example>/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
`` `

## What This Example Teaches

<One paragraph: what the human will understand after the demo>

## Prerequisites

- `cub` in PATH
- `jq` for JSON preview
- Authenticated ConfigHub CLI context for mutating steps
- Optional: <other requirements>

---

## Stage 1: "<Title>" (read-only)

Run:

`` `bash
<command>
`` `

What to explain:

- <bullet points about what the output means>

GUI now: <exact URL or click path and what is visible today>

GUI gap: <what the GUI cannot show yet>

GUI feature ask: <what the GUI should show next, with issue number if known; if no issue exists, say "No issue filed yet.">

**PAUSE.** Wait for the human.

---

## Stage 2: "<Title>" (mutates ConfigHub)

Ask: "<Confirmation question about what will be created/changed>"

Run:

`` `bash
<command>
`` `

What to explain:

- <bullet points about what changed>

GUI now: <what to inspect>

GUI gap: <what's missing>

GUI feature ask: <feature request with issue number if known>

**PAUSE.** Wait for the human.

---

## Stage 3: "<Title>" (read-only)

<Repeat the pattern>

---

## Stage N: "Cleanup"

Run:

`` `bash
./cleanup.sh
`` `

This removes all objects created by the demo.

---

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh --explain-json` | Nothing |
| `./setup.sh` | <ConfigHub objects, local files> |
| `./verify.sh` | <local log files only> |
| `./cleanup.sh` | <deletes ConfigHub objects> |

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
```

**Important notes for authors:**

1. The `## CRITICAL: Demo Pacing` block must appear **exactly** at the top, before any stages
2. The `## Suggested Prompt` must appear early, immediately after the pacing block
3. Every stage must have:
   - A human-readable title ("Preview the plan" not "Step 1")
   - A read-only or mutates annotation in parentheses
   - `Run:` with exact commands
   - `What to explain:` with bullet points
   - `GUI now:` / `GUI gap:` / `GUI feature ask:` (or an explicit "No GUI checkpoint for this stage")
   - `**PAUSE.** Wait for the human.`
4. If there is no GUI checkpoint, say that explicitly: "GUI now: No GUI checkpoint for this stage — this is CLI-only."
5. If there is no issue number for a GUI feature ask, say: "No issue filed yet."

---

## `prompts.md` Template

Suggested prompts:

```md
# Prompts

## Orient Me First

Read this example, do not mutate anything yet, and explain:
- what stack it is for
- what it reads
- what it writes
- what I need installed
- what success should look like

## Safe Walkthrough

Guide me through this example step by step.
Pause after every stage. Show full output.
For each stage:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- tell me what success looks like
- tell me what to inspect next in the GUI
Do not continue until I say continue.

## Verify Everything

After running the example, verify:
- ConfigHub objects
- GUI state
- target/worker readiness
- live apply or GitOps state if available
```

---

## `contracts.md` Template

Suggested structure:

```md
# Contracts

## Read-only contracts

### `./setup.sh --explain-json`
- mutates: no
- stable fields: `spaces`, `units`, `willCreate`, `willDelete`
- proves: the example plan before any mutation

### `cub space list --where "Labels.ExampleName = '<example>'" --json`
- mutates: no
- stable fields: `Space.Slug`, `Space.Labels`
- proves: spaces created by this example

### `cub unit get --space <space> --json <unit>`
- mutates: no
- stable fields: `Unit.Slug`, `Unit.Data`
- proves: unit content after setup
```

---

## Review Checklist

Before merging a new example:

- [ ] Does the README answer the six key reader questions?
- [ ] Is there a read-only first path (`--explain`, `--explain-json`)?
- [ ] Is there a short AI guide (`AI_START_HERE.md`)?
- [ ] Does the AI guide have `## CRITICAL: Demo Pacing` at the top?
- [ ] Does the AI guide have `## Suggested Prompt`?
- [ ] Does the AI guide have numbered `## Stage N:` sections?
- [ ] Does every stage have `GUI now:`, `GUI gap:`, `GUI feature ask:` (or explicit "no checkpoint")?
- [ ] Does every stage end with `**PAUSE.** Wait for the human.`?
- [ ] Are copyable prompts documented?
- [ ] Are expected outputs documented?
- [ ] Are cleanup steps documented?
- [ ] Is there a stable JSON or text contract?
- [ ] Does `setup.sh` support `--explain` and `--explain-json`?
