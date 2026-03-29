# Standard: AI-first demo pacing for ConfigHub examples

This is the standard for all `AI_START_HERE.md` files in ConfigHub examples and
related demo cases. Use this to teach the team how to do AI-first demos.

---

## Why this matters

AI-first means the AI is the developer experience. When Ilya or David watches
a demo, the thing they're evaluating is: "Could I paste this prompt and get
this experience myself?" If the demo works, they try it. If it doesn't, they
don't.

The demo prompt is the product. The AI_START_HERE.md file is the spec.

---

## The problem with naive AI demos

When Claude drives a demo without explicit pacing rules, it races through:

1. Runs a command
2. Shows a one-line summary
3. Moves to the next command
4. Finishes in 30 seconds
5. Human learned nothing

The human never gets to study the output, click a GUI link, or ask a question.
The demo feels like watching a script execute, not understanding a product.

**Root cause:** AI_START_HERE.md files tell Claude *what commands to run* but
not *how to present the results*. Claude optimizes for task completion, not
demo pacing.

---

## The three rules

Every AI_START_HERE.md must follow these rules.

### Rule 1: Explicit pause-and-show block at the top

Add this exact block at the top of every AI_START_HERE.md:

```markdown
## CRITICAL: Demo pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen. If it is long, keep the important section visible and do not replace it with a one-line summary
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

This is a demo, not a script execution. The value is in understanding each step.
```

Without this block, Claude will not pause. It must be explicit and at the top.

### Rule 2: Stage-based structure with pause markers

**Bad** (Claude races through):

```markdown
## Commands
./setup.sh
./compare.sh
./mutate.sh
```

**Good** (Claude pauses at each):

```markdown
### Stage 1: "What will this create?" (read-only)

Run: `./setup.sh --explain`

Show the output clearly. Explain what it means.

GUI: Open https://hub.confighub.com → Units → this is where the objects will appear.

**PAUSE.** Wait for the human to say "continue."

---

### Stage 2: "Create the config" (mutates ConfigHub)

Ask: "This will create 3 spaces and 3 units. OK?"

Run: `./setup.sh`

Show the output. Then verify with:
`cub space list --where "Labels.App = 'inventory-api'" --json`

GUI: Open https://hub.confighub.com → Units → filter by "inventory-api" →
you should see three units in a data grid.

**PAUSE.** Wait for the human.
```

Each stage must have:
- A human-readable title ("What will this create?" not "Step 1")
- A read-only or mutates annotation
- What command(s) to run
- What to show from the output
- What to explain
- A GUI link with exact click path
- **PAUSE.** marker

### Rule 3: Three-part GUI annotations

Every stage with a GUI link must include THREE things:

1. **What the GUI shows today** — specific URL, click path, what they'll see
2. **What the GUI cannot show yet** — explicit gap statement
3. **The feature ask** — what it should show, with issue number

Example:

```markdown
GUI now: Open https://hub.confighub.com → Units → click inventory-api-prod →
see the rendered ConfigMap YAML → find FEATURE_INVENTORY_RESERVATIONMODE.
The value should say "optimistic".

GUI gap: You can see field values, but there are no route badges showing
which fields are mutable vs platform-owned. Every field looks equally editable.

GUI ask: Colored route badges (green=mutable, yellow=lift-upstream,
red=generator-owned) next to each field. Issue #209.
```

This teaches the audience that GUI and CLI show the same data, makes gaps
concrete, and gives the product team experience-grounded feature asks.

---

## The suggested prompt

Every AI_START_HERE.md should include this prompt that humans can paste:

```
Read <path>/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show the output clearly. Give GUI links where possible.
Don't move on until I say continue.
```

The key phrases are "pause after every stage" and "don't move on until I say."

---

## Mutation scenario requirements

For examples that demonstrate mutation routing (apply-here, lift-upstream,
block-escalate), each scenario needs five parts:

1. **The pain** — what does this problem look like today, in the user's words
2. **The fix** — what ConfigHub gives you (exact commands)
3. **What you see** — full expected output, with annotations
4. **What this proves** — explicit claim
5. **What this does NOT prove** — honest boundary

Without all five, the demo either overclaims or undersells.

---

## Checklist: examples that need this standard

Verified on 2026-03-29: the repo currently has 31 `AI_START_HERE.md` files.

Root:

- [ ] `AI_START_HERE.md`

Incubator:

- [ ] `incubator/AI_START_HERE.md`
- [ ] `incubator/apptique-argo-app-of-apps/AI_START_HERE.md`
- [ ] `incubator/apptique-argo-applicationset/AI_START_HERE.md`
- [ ] `incubator/apptique-flux-monorepo/AI_START_HERE.md`
- [ ] `incubator/artifact-workflow/AI_START_HERE.md`
- [ ] `incubator/combined-git-live/AI_START_HERE.md`
- [ ] `incubator/connect-and-compare/AI_START_HERE.md`
- [ ] `incubator/connected-summary-storage/AI_START_HERE.md`
- [ ] `incubator/custom-ownership-detectors/AI_START_HERE.md`
- [ ] `incubator/demo-data-adt/AI_START_HERE.md`
- [ ] `incubator/fleet-import/AI_START_HERE.md`
- [ ] `incubator/flux-boutique/AI_START_HERE.md`
- [x] `incubator/gitops-import-argo/AI_START_HERE.md`
- [x] `incubator/gitops-import-flux/AI_START_HERE.md`
- [ ] `incubator/graph-export/AI_START_HERE.md`
- [ ] `incubator/import-from-bundle/AI_START_HERE.md`
- [ ] `incubator/import-from-live/AI_START_HERE.md`
- [ ] `incubator/lifecycle-hazards/AI_START_HERE.md`
- [ ] `incubator/orphans/AI_START_HERE.md`
- [ ] `incubator/platform-example/AI_START_HERE.md`
- [x] `incubator/platform-write-api/AI_START_HERE.md`
- [ ] `incubator/springboot-platform-app-centric/AI_START_HERE.md`
- [x] `incubator/springboot-platform-app/AI_START_HERE.md`
- [ ] `incubator/watch-webhook/AI_START_HERE.md`

Global app layer:

- [x] `incubator/global-app-layer/AI_START_HERE.md`
- [ ] `incubator/global-app-layer/bundle-evidence-sample/AI_START_HERE.md`
- [ ] `incubator/global-app-layer/frontend-postgres/AI_START_HERE.md`
- [ ] `incubator/global-app-layer/gpu-eks-h100-training/AI_START_HERE.md`
- [ ] `incubator/global-app-layer/realistic-app/AI_START_HERE.md`
- [x] `incubator/global-app-layer/single-component/AI_START_HERE.md`

---

## How to use this to teach the team

1. **Share this doc** as the standard for AI-first demos
2. **Pick one example** and update its AI_START_HERE.md together
3. **Run the demo** with the suggested prompt — see the pauses work
4. **File issues** for GUI gaps discovered during the demo
5. **Repeat** for each example

The goal: every example in the incubator can be demoed by pasting one prompt,
and the demo pauses at each stage, shows full output, and gives GUI context.

---

## Template

Copy this template when creating a new AI_START_HERE.md:

```markdown
# AI Start Here: <Example Name>

## CRITICAL: Demo pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen. If it is long, keep the important section visible and do not replace it with a one-line summary
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

## Suggested prompt

```
Read <path>/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show the output clearly. Give GUI links where possible.
Don't move on until I say continue.
```

## What this example teaches

<One paragraph: what the human will understand after the demo>

## Stages

### Stage 1: "<Title>" (read-only)

Run: `<command>`

<What to explain>

GUI now: <URL and click path>
GUI gap: <What's missing>
GUI ask: <Feature request with issue number>

**PAUSE.** Wait for the human.

---

### Stage 2: "<Title>" (mutates)

Ask: "<Confirmation question>"

Run: `<command>`

<What to explain>

GUI now: <URL and click path>
GUI gap: <What's missing>
GUI ask: <Feature request with issue number>

**PAUSE.** Wait for the human.

---

<Repeat for each stage>

## Cleanup

Run: `./cleanup.sh`

This removes all objects created by the demo.
```

---

## Origin

Learned from building `platform-write-api` and `springboot-platform-app` demos.
Validated against Ilya (Coreweave), David Flanagan, and Camille personas.
