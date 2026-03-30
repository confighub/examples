# AI-First Demo Pacing Standard

This document defines the standard for all `AI_START_HERE.md` files in the examples repo.

## The Standard

Every `AI_START_HERE.md` file should include:

### 1. CRITICAL pause block (required at top)

```markdown
## CRITICAL: Demo pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Print the FULL output on screen — do not summarize or abbreviate
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue
```

### 2. Stage-based structure (required)

- Each stage is a numbered section: `### Stage N: "What does this do?" (read-only|mutates X)`
- Each stage ends with `**PAUSE.** Wait for the human.`
- Stages progress from read-only → mutating

### 3. GUI annotations (required where applicable)

- `GUI now:` — what to see in the GUI at this point
- `GUI gap:` — what the GUI doesn't show yet
- `GUI ask:` — feature request for the GUI

## Files to Update

The following 31 files need to be reviewed and updated:

### Root level (2)
- [ ] `./AI_START_HERE.md`
- [ ] `./incubator/AI_START_HERE.md`

### Incubator examples (29)
- [ ] `./incubator/apptique-argo-app-of-apps/AI_START_HERE.md`
- [ ] `./incubator/apptique-argo-applicationset/AI_START_HERE.md`
- [ ] `./incubator/apptique-flux-monorepo/AI_START_HERE.md`
- [ ] `./incubator/artifact-workflow/AI_START_HERE.md`
- [ ] `./incubator/combined-git-live/AI_START_HERE.md`
- [ ] `./incubator/connect-and-compare/AI_START_HERE.md`
- [ ] `./incubator/connected-summary-storage/AI_START_HERE.md`
- [ ] `./incubator/custom-ownership-detectors/AI_START_HERE.md`
- [ ] `./incubator/demo-data-adt/AI_START_HERE.md`
- [ ] `./incubator/fleet-import/AI_START_HERE.md`
- [ ] `./incubator/flux-boutique/AI_START_HERE.md`
- [ ] `./incubator/gitops-import-argo/AI_START_HERE.md`
- [ ] `./incubator/gitops-import-flux/AI_START_HERE.md`
- [ ] `./incubator/global-app-layer/AI_START_HERE.md`
- [ ] `./incubator/global-app-layer/bundle-evidence-sample/AI_START_HERE.md`
- [ ] `./incubator/global-app-layer/frontend-postgres/AI_START_HERE.md`
- [ ] `./incubator/global-app-layer/gpu-eks-h100-training/AI_START_HERE.md`
- [ ] `./incubator/global-app-layer/realistic-app/AI_START_HERE.md`
- [ ] `./incubator/global-app-layer/single-component/AI_START_HERE.md`
- [ ] `./incubator/graph-export/AI_START_HERE.md`
- [ ] `./incubator/import-from-bundle/AI_START_HERE.md`
- [ ] `./incubator/import-from-live/AI_START_HERE.md`
- [ ] `./incubator/lifecycle-hazards/AI_START_HERE.md`
- [ ] `./incubator/orphans/AI_START_HERE.md`
- [ ] `./incubator/platform-example/AI_START_HERE.md`
- [ ] `./incubator/platform-write-api/AI_START_HERE.md`
- [x] `./spring-platform/springboot-platform-app-centric/AI_START_HERE.md` ✅
- [x] `./spring-platform/springboot-platform-app/AI_START_HERE.md` ✅
- [ ] `./incubator/watch-webhook/AI_START_HERE.md`

## Reference Implementation

See these examples that already follow the standard:

- `spring-platform/springboot-platform-app-centric/AI_START_HERE.md`
- `spring-platform/springboot-platform-app/AI_START_HERE.md`

## Why This Matters

AI-assisted demos are becoming a primary interaction mode for ConfigHub examples. The pacing standard ensures:

- Humans stay in control and can explore at their own pace
- AI doesn't race ahead and overwhelm the user
- GUI checkpoints are documented so users can verify in the web UI
- The experience is consistent across all examples

## Template

Use this template when creating new `AI_START_HERE.md` files:

```markdown
# AI Start Here: [Example Name]

## CRITICAL: Demo pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Print the FULL output on screen — do not summarize or abbreviate
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

This is a demo, not a script execution. The value is in understanding each step.

## Suggested prompt

\`\`\`
Read incubator/[example-name]/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Don't move on until I say continue.
\`\`\`

## What this example teaches

[Brief description of what the user will learn]

## Stages

### Stage 1: "[Question this stage answers]" (read-only)

[Commands to run]

[What to explain]

GUI now: [What to see in GUI]

**PAUSE.** Wait for the human.

---

### Stage 2: "[Next question]" (mutates X)

...

### Stage N: "Cleanup"

[Cleanup commands]

---

## Key files

[Table of important files]
```
