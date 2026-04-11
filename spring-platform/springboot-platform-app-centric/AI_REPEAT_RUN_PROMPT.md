# AI Repeat-Run Prompt

Use this for a fresh Claude or Codex session when you want a **repeat-until-boring** pass against the Spring Platform app-centric example.

## Operator Prep

Open a fresh AI session in the `examples` repo, then use this example directory as the working focus:

```bash
cd /Users/alexis/Public/github-repos/examples
./scripts/verify.sh
```

If auth is missing or expired:

```bash
cub auth login
```

Do not pre-run `./setup.sh`. Let the agent do the first real mutation path.

## Prompt

````markdown
You are running a repeat-until-boring evaluation of the Spring Platform app-centric example.

# Goal

Help me determine whether this example is becoming boringly reliable for AI-assisted use.

Use:

- `spring-platform/springboot-platform-app-centric/AI_START_HERE.md`
- `spring-platform/springboot-platform-app-centric/contracts.md`

Start with the fast preview, then continue into the fast operational evaluation unless I explicitly ask you to stay read-only.

# For each phase, report:

1. what you are testing
2. the command you ran
3. the important output
4. what this proves
5. what this does not prove
6. what you will do next

# Constraints

- Do not invent command surfaces. Use the example files and live help when needed.
- Do not stop after preview and call the example "working."
- Do not run cleanup unless I explicitly ask for it.
- If a command or contract is stale, say so clearly and recover using the live CLI/help surface.

# Closeout requirements

At the end, produce:

## A. Run summary
- short phase-by-phase summary

## B. Drift moments
- what drifted
- how you recovered
- whether the example contract or repo guidance helped

## C. Findings
- real product/doc/example gaps only

## D. Pattern status recommendation
- current status:
  - documented-only
  - preview-only
  - operational-once
  - repeatable-with-steering
  - repeatable-with-light-steering
  - boring
- whether this run should move the status
- why

## E. Time
- approximate wall-clock time

Begin.
````

## What A Good Run Looks Like

- the agent reads the example entry point
- the agent completes both:
  - fast preview
  - fast operational evaluation
- `./setup.sh` succeeds
- label isolation is shown
- `./verify.sh` is described honestly as fixture/contract verification
- one representative `apply-here` mutation succeeds
- mutation history is shown cleanly
- noop-target apply is explained honestly
- the closeout distinguishes:
  - what was proven
  - what was not proven

## What To Watch For

- stopping after preview and declaring success
- claiming `./verify.sh` proves live post-setup ConfigHub state
- unclear mutation audit reporting
- overstating what noop target mode proves
