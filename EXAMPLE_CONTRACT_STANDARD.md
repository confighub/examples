# Example Contract Standard

This doc explains the machine-readable contract for runnable ConfigHub examples.

## Why This Matters

Runnable examples need two entry points:

| Entry point | Audience | Purpose |
|-------------|----------|---------|
| Human-readable | Humans, AI assistants | Understand the example before running |
| Machine-readable | Scripts, CI, AI assistants | Inspect the plan programmatically |

The human-readable entry point is `README.md` + `AI_START_HERE.md`.

The machine-readable entry point is `setup.sh --explain-json` + `contracts.md`.

Both are required for full runnable examples. Together they support:
- Safe preview before mutation
- Automated verification
- AI-assisted walkthroughs with structured output

## Current Scope

The verifier (`scripts/verify.sh`) currently covers **30 examples**:

| Category | Count | Examples |
|----------|-------|----------|
| Stable | 4 | spring-platform (2), initiatives-demo, promotion-demo-data |
| Incubator | 26 | global-app-layer (5), gitops-import (2), apptique (3), and 16 others |

## What `./setup.sh --explain` Must Do

Print a human-readable plan. This is what humans and AI read before running.

Requirements:
- Must not mutate ConfigHub or live infrastructure
- Must describe what the example will create
- Should show the conceptual model (e.g., ASCII diagram)
- Should list mutation outcomes or proof types

Example output anchors:
- Section headers describing the model
- Bullet points listing objects to create
- Clear statement of what will be mutated

## What `./setup.sh --explain-json` Must Do

Print a machine-readable plan as JSON. This is what scripts and AI parse.

Requirements:
- Must not mutate ConfigHub or live infrastructure
- Must be valid JSON
- Must include stable fields that identify the example

Recommended stable fields:

```json
{
  "example_name": "example-slug",
  "mutates": false,
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "spaces": ["space-1", "space-2"],
  "units": ["unit-1", "unit-2"]
}
```

When an example supports both a preview path and a minimal real proof path, prefer adding explicit evaluation fields such as:

```json
{
  "evaluation_modes": {
    "fast_preview": {
      "mutates": false,
      "commands": ["./setup.sh --explain", "./setup.sh --explain-json | jq"]
    },
    "fast_operational_evaluation": {
      "mutates_confighub": true,
      "mutates_live_infra": false,
      "commands": ["./setup.sh", "./verify.sh"],
      "stop_before_cleanup": true
    }
  }
}
```

These fields help AI assistants distinguish "quick orientation" from "smallest real proof."

The exact schema varies by example, but the intent is always:
- Identify what will be created
- State what will be mutated
- Provide anchors for verification

## What `contracts.md` Must Document

Document stable command outputs that automation can rely on.

For each contract, state:

| Field | Purpose |
|-------|---------|
| `mutates` | Does this command write anything? |
| `output shape` | JSON, plain text, or specific format |
| `stable fields` or `stable text anchors` | What patterns are safe to match on |
| `proves` | What the output demonstrates |

### Example: Read-Only Contract

```markdown
### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `spaces`, `units`, `mutates`
- proves: the example plan before any mutation
```

### Example: Mutating Contract

```markdown
### `./setup.sh`

- mutates: yes (ConfigHub)
- creates: 4 spaces, 3 units, 3 targets
- cleanup: `./cleanup.sh`
```

### Example: Verification Contract

```markdown
### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All checks passed.`
```

### Example: Representative proof action

```markdown
### `cub function do ...`

- mutates: yes (ConfigHub)
- output shape: plain text success + persisted mutation history
- proves: at least one real path through the example works beyond setup alone
```

## What `verify.sh` Is For

Each example's `verify.sh` confirms that setup succeeded.

The repo-level `scripts/verify.sh` enforces standards across all examples:
- Shell script syntax
- Required files (README.md, AI_START_HERE.md, contracts.md)
- Required markers in AI guides
- Required flags in setup.sh (--explain, --explain-json)
- Required markers in contracts.md (`mutates:`, `proves:`)

Run it before committing:

```bash
./scripts/verify.sh
```

## Relationship to AI Guide Standard

This doc covers the **machine-readable** contract (`--explain-json`, `contracts.md`).

The **AI guide standard** covers the **human-readable** contract (`AI_START_HERE.md`).

Both are enforced by `scripts/verify.sh`. Both are required for full runnable examples.

| Standard | Doc | What it covers |
|----------|-----|----------------|
| AI guide | [incubator/ai-guide-standard.md](./incubator/ai-guide-standard.md) | Demo pacing, stages, GUI markers |
| Machine-readable | This doc | --explain-json, contracts.md, verify.sh |

## Exemptions

Some lighter stable examples are exempt from the stronger requirements.

### Current exemptions

| Example | Exempt from | Reason |
|---------|-------------|--------|
| `initiatives-demo` | `contracts.md`, `--explain` | Stable demo data |
| `promotion-demo-data` | `contracts.md`, `--explain` | Stable demo data |
| `incubator/watch-webhook` | `contracts.md` | Lightweight event example |

### When exemptions are appropriate

- Demo data examples that populate ConfigHub but don't need structured preview
- Lightweight examples where the README is sufficient
- Examples where --explain would add complexity without value

Keep exemptions intentionally narrow. The stronger contract is the default.

## Adding a New Example

For a full runnable example:

1. Create `setup.sh` with `--explain` and `--explain-json` support
2. Create `contracts.md` documenting stable outputs
3. Create `AI_START_HERE.md` following the AI guide standard
4. Add the example to `scripts/verify.sh`
5. Run `./scripts/verify.sh` to confirm

For a lighter example (with justification):

1. Add to the appropriate exemption array in `scripts/verify.sh`
2. Document why the exemption is appropriate

## Related

- [incubator/ai-guide-standard.md](./incubator/ai-guide-standard.md) — AI guide requirements
- [incubator/ai-example-template.md](./incubator/ai-example-template.md) — Full template to copy
- [scripts/verify.sh](./scripts/verify.sh) — Enforcement script
