# AI README FIRST

This is the canonical repo-level guide for AI assistants working in `confighub/examples`.

If you want the shortest strict protocol first, read [AGENTS.md](./AGENTS.md) before this file.

## 1. What This File Is For

Use this file for:

- repo-level AI operating rules
- CLI and mutation safety guidance
- choosing the right example family

Do not treat this file as the walkthrough script. When the user wants pause-heavy demo mode, use the selected example's `AI_START_HERE.md`, or [incubator/AI_START_HERE.md](./incubator/AI_START_HERE.md) for incubator-wide walkthrough conventions.

## 2. Resolve The Repo Root First

Do not hardcode the checkout path. This repo may be checked out as `examples`, `confighub-examples`, or another folder name.

Resolve the repo root first:

```bash
git rev-parse --show-toplevel
```

## 3. Access Live ConfigHub Through `cub`

If you need live ConfigHub state, use the `cub` CLI.

Do not assume that fetching `https://hub.confighub.com` directly will give you meaningful data. It is a browser application. For machine use, `cub` is the right interface.

Use:

```bash
cub auth login
cub context list --json
cub space list --json
cub target list --space "*" --json
```

## 4. Important CLI Gotchas

Avoid these common mistakes:

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub auth status`
- use `--json` and `--jq` when you want machine-readable output
- use `--where "Labels.Key = 'value'"` for label filtering on list commands, not guessed `--label` filters
- use `--dry-run` before `cub function do` or `cub unit apply` if you want a non-mutating preview

Good discovery commands:

```bash
export CONFIGHUB_AGENT=1
cub --help-overview
cub info
cub context list --json
```

## 5. Default Operating Mode

Start in read-only mode.

Recommended order:

1. inspect the repo
2. run repo verification
3. inspect live ConfigHub state in JSON
4. only then run mutating example flows if the human wants that

## 6. Safe First Commands

These commands do not mutate ConfigHub or any cluster:

```bash
cd <your-examples-checkout>
./scripts/verify.sh
rg --files .
cub context list --json
cub space list --json
cub target list --space "*" --json
```

If you already know a specific unit:

```bash
cub unit get --space <space> --json <unit>
```

## 7. Choose The Right Example Family

Pick the family that matches the user's goal:

- Stable no-cluster intro: [promotion-demo-data](./promotion-demo-data/README.md) and [campaigns-demo](./campaigns-demo/README.md)
- GitOps import or brownfield discovery: [gitops-import](./gitops-import/README.md), [import-from-live](./incubator/import-from-live/README.md), [gitops-import-argo](./incubator/gitops-import-argo/README.md), [gitops-import-flux](./incubator/gitops-import-flux/README.md)
- App mutation and platform flow: [springboot-platform-app-centric](./spring-platform/springboot-platform-app-centric/README.md)
- Worker extensibility: [custom-workers](./custom-workers/)
- Layered and advanced model: [incubator/global-app-layer](./incubator/global-app-layer/README.md)
- Full experimental catalog: [incubator/README.md](./incubator/README.md)

If the user wants a human-oriented overview of the repo, [README.md](./README.md) is the public front door, but it is not the primary AI entry point.

## 8. Shared Meanings For “Evaluate Quickly”

Use these meanings consistently across examples:

- **preview** = read-only orientation
- **fast preview** = read-only example-specific path such as `./setup.sh --explain`, `./setup.sh --explain-json`, or other non-mutating preview/report commands
- **fast operational evaluation** = preview plus the smallest real setup and proof sequence that demonstrates the example actually works

If a user says "help me evaluate it quickly" and does not explicitly ask for read-only mode, the default should be:

1. run the fast preview
2. run the smallest real operational proof path for that example
3. stop before cleanup unless asked

Do not conclude that an example is ready based only on preview output if the example provides a real setup path and proof path.

## 9. Machine-Readable Contracts

Treat these as stable repo-wide contracts:

| Command | Output contract | Mutates anything? |
|---|---|---|
| `./scripts/verify.sh` | exit code + plain text verifier output | no |
| `cub context list --json` | JSON array of contexts | no |
| `cub space list --json` | JSON array of spaces | no |
| `cub target list --space "*" --json` | JSON array of targets | no |
| `cub unit get --space <space> --json <unit>` | JSON object for one unit | no |
| `cub function do --dry-run --json ...` | JSON invocation response | no config write |
| `cub unit apply --dry-run --json ...` | JSON apply preview | no live apply |

Example-specific seams vary. Do not assume `./verify.sh --json`, `./setup.sh --explain-json`, `./find-runs.sh --json`, or similar flags exist unless the example README, AI guide, script help, or source confirms them.

## 10. Mutating Commands: Use Care

Examples often use:

- `./setup.sh`
- `./set-target.sh`
- `cub unit apply`
- `cub function do`

Those do create or mutate ConfigHub objects, and in some cases can affect a live cluster.

Before you run them, say clearly:

- what space(s) will be created or changed
- what unit(s) will be created or changed
- whether any target or cluster will be touched
- what read-only or dry-run command could be used first

## 11. Walkthrough Mode

Use the selected example's `AI_START_HERE.md` when the user wants a guided demo with pauses after each stage.

Use [incubator/AI_START_HERE.md](./incubator/AI_START_HERE.md) when the work is specifically in the incubator and you need incubator-specific walkthrough conventions.

## 12. Next Step

If you are starting fresh, the best single next step is:

```bash
cd <your-examples-checkout>
./scripts/verify.sh
```

Then choose the example family that matches the user's goal.
