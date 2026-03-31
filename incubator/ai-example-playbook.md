# Incubator AI Example Playbook

Use this playbook when:
- adding a new incubator example
- improving an existing incubator example
- reviewing whether an example is understandable to humans and AI

For the file templates, see [ai-example-template.md](./ai-example-template.md).

## Goal

An important incubator example should be easy to:
- understand quickly
- preview safely
- run step by step
- verify afterwards
- hand off between humans and AI

## Non-Negotiable Reader Questions

Every important example should answer these directly:

1. what stack is this for?
2. what do I need installed?
3. what does this read?
4. what does this write?
5. what should I expect to see?
6. how would an AI assistant run this safely?

Do not hide the answers inside shell scripts.

## Required File Bundle

For runnable examples:

```text
README.md
AI_START_HERE.md
prompts.md
contracts.md
setup.sh      # must support --explain and --explain-json
verify.sh
cleanup.sh
```

## Demo Pacing Rules

Every `AI_START_HERE.md` must teach the AI how to present the demo, not only which commands to run.

### Rule 1: Explicit pacing block at the top

Every AI guide must start with this exact section header:

```md
## CRITICAL: Demo Pacing
```

This block tells the AI to pause after every stage, show full output, and wait for the human.

### Rule 2: Suggested prompt early

Every AI guide must include:

```md
## Suggested Prompt
```

This gives humans a copyable prompt to start the demo correctly.

### Rule 3: Stage-based structure

Use numbered stages with human-readable titles and annotations:

- `## Stage 1: "Preview the plan" (read-only)`
- `## Stage 2: "Materialize in ConfigHub" (mutates ConfigHub)`
- `## Stage 3: "Verify the structure" (read-only)`
- `## Stage N: "Cleanup"`

Do not present the example as an uninterrupted list of shell commands.

### Rule 4: GUI now, GUI gap, and GUI feature ask

Every stage must include these three markers (or explicitly state there is no GUI checkpoint):

- `GUI now:` exact URL or click path and what is visible today
- `GUI gap:` what the GUI cannot show yet
- `GUI feature ask:` what the GUI should show next, with issue number if known

If there is no issue number, say: "No issue filed yet."

If there is no GUI checkpoint for a stage, say: "GUI now: No GUI checkpoint for this stage — this is CLI-only."

### Rule 5: Pause markers

Every stage must end with:

```md
**PAUSE.** Wait for the human.
```

## Narrative Arc Rules

For any example that demonstrates mutation or routing, each scenario should cover all five:

1. the pain
2. the fix
3. what you see
4. what this proves
5. what this does not prove

Without all five, the demo either overclaims or undersells.

Every mutating step should also include a concrete `what you see after` section with exact visible evidence.

If the step has a GUI equivalent, `what you see after` should include both:

- CLI-visible evidence
- GUI-visible evidence

## Required Doc Shape

### 1. Stack And Scenario

Say:
- what stack or platform this is for
- what user problem it demonstrates

### 2. What This Proves

Say:
- the main point of the example
- whether it is structural proof, ConfigHub-only proof, or live deployment proof

### 3. Prerequisites

Say:
- what must be installed locally
- whether `cub auth login` is required
- whether a worker, target, cluster, or GitOps controller is required

### 4. What This Reads And Writes

Say clearly:
- what files it reads
- what ConfigHub objects it writes
- what live infrastructure it writes, if any

### 5. Read-Only Preview

Start with a non-mutating path when possible:
- `--help`
- `--json`
- `--dry-run`
- `--explain`
- `--explain-json`

Always say what the command does not mutate.

### 6. Exact CLI Sequence

Provide:
- exact commands
- placeholder meanings
- whether each command mutates ConfigHub only or also live infrastructure

### 7. Expected Output

After each major stage, say what success looks like.

Examples:
- spaces created with a known prefix
- units visible in a known space
- specific JSON fields present
- GUI page opens to the expected object
- target becomes visible
- apply completes successfully

### 8. Verification

Document exact verification commands for:
- ConfigHub state
- GUI state
- target or worker readiness
- live apply or GitOps state, if used

### 9. Troubleshooting

Cover at least:
- no `cub`
- no auth
- wrong context
- no worker
- no target
- wrong repo path
- example uses stubs rather than real software

### 10. Cleanup

Say:
- exact cleanup steps
- what cleanup removes
- what cleanup does not remove

## Copyable Prompt Guidance

Major examples should include 2-4 prompts such as:

- orient me before we run anything
- run the read-only preview and explain each step
- run this example safely and verify it after each stage
- show me the GUI checkpoints while we go

## Stable Contract Guidance

Document at least one stable machine-readable inspection path.

Good examples:

- `cub space list --json`
- `cub unit get --space <space> --json <unit>`
- `./setup.sh --explain-json`

For each contract, say:
- whether it mutates
- which fields or text patterns are stable
- what the output proves

## Minimal Review Checklist

Before calling an incubator example `AI-friendly`, verify:

- it answers the six reader questions
- it has a read-only first step
- it has stage-based pacing guidance
- it has a short AI guide
- it has copyable prompts
- it documents expected output
- it has cleanup steps
- it documents at least one stable contract
