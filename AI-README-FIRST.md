# AI README FIRST

If you are an AI assistant, read this file before exploring the rest of the repo.

This repo is meant to be understandable and usable by both humans and AI assistants, but you will avoid a lot of confusion if you start here.

## 1. What This Repo Is

This is the public `confighub/examples` repo.

It contains:

- stable examples such as `promotion-demo-data`, `global-app`, `helm-platform-components`, and `vm-fleet`
- incubator work in [`incubator/`](./incubator/README.md), especially the layered-recipe package in [`incubator/global-app-layer/`](./incubator/global-app-layer/README.md)
- verifier scripts in `./scripts/`

## 2. How To Access Live ConfigHub

If you need live ConfigHub state, use the `cub` CLI.

Do **not** assume that fetching `https://hub.confighub.com` directly will give you meaningful data. It is a browser application. For machine use, `cub` is the right interface.

Use:

```bash
cub auth login
cub context list --json
cub space list --json
cub target list --space "*" --json
```

## 3. Important CLI Gotchas

Avoid these common mistakes:

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub auth status`
- use `--json` and `--jq` when you want machine-readable output
- use `--dry-run` before `cub function do` or `cub unit apply` if you want a non-mutating preview

Good discovery commands:

```bash
export CONFIGHUB_AGENT=1
cub --help-overview
cub info
cub context list --json
```

## 4. Default Operating Mode

Start in read-only mode.

Recommended order:

1. inspect the repo
2. run repo verification
3. inspect live ConfigHub state in JSON
4. only then run mutating example flows if the human wants that

## 5. Safe First Commands

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

What these commands do **not** mutate:

- spaces
- units
- workers
- targets
- clusters
- Git history

## 6. Mutating Commands: Use Care

Examples often use:

- `./setup.sh`
- `./set-target.sh`
- `cub unit apply`
- `cub function do`

Those **do** create or mutate ConfigHub objects, and in some cases can affect a live cluster.

Before you run them, say clearly:

- what space(s) will be created or changed
- what unit(s) will be created or changed
- whether any target or cluster will be touched
- what read-only or dry-run command could be used first

## 7. Machine-Readable Contracts

Use these as your default contracts:

| Command | Output contract | Mutates anything? |
|---|---|---|
| `./scripts/verify.sh` | exit code + plain text verifier output | no |
| `cub context list --json` | JSON array of contexts | no |
| `cub space list --json` | JSON array of spaces | no |
| `cub target list --space "*" --json` | JSON array of targets | no |
| `cub unit get --space <space> --json <unit>` | JSON object for one unit | no |
| `cub function do --dry-run --json ...` | JSON invocation response | no config write |
| `cub unit apply --dry-run --json ...` | JSON apply preview | no live apply |

## 8. Best First Reading Order

If the user is asking about ConfigHub generally:

1. [START_HERE.md](./START_HERE.md)
2. [`promotion-demo-data/README.md`](./promotion-demo-data/README.md)
3. [`incubator/global-app-layer/00-config-hub-hello-world.md`](./incubator/global-app-layer/00-config-hub-hello-world.md)

If the user is asking about NVIDIA AICR, recipes, or layered variants:

1. [`incubator/global-app-layer/README.md`](./incubator/global-app-layer/README.md)
2. [`incubator/global-app-layer/confighub-aicr-value-add.md`](./incubator/global-app-layer/confighub-aicr-value-add.md)
3. [`incubator/global-app-layer/how-it-works.md`](./incubator/global-app-layer/how-it-works.md)
4. one worked example under `incubator/global-app-layer/`

If the user is asking about incubator-only work:

1. [`incubator/README.md`](./incubator/README.md)
2. [`incubator/AI_START_HERE.md`](./incubator/AI_START_HERE.md)

## 9. Repo Layout You Will Actually Need

Most assistants only need these locations:

- stable repo landing: [README.md](./README.md)
- human entry path: [START_HERE.md](./START_HERE.md)
- AI entry path: [AI_START_HERE.md](./AI_START_HERE.md)
- incubator landing: [`incubator/README.md`](./incubator/README.md)
- layered recipes package: [`incubator/global-app-layer/README.md`](./incubator/global-app-layer/README.md)
- package mechanics: [`incubator/global-app-layer/how-it-works.md`](./incubator/global-app-layer/how-it-works.md)
- package value-add story: [`incubator/global-app-layer/confighub-aicr-value-add.md`](./incubator/global-app-layer/confighub-aicr-value-add.md)

## 10. Placeholder Conventions

When you see placeholders like:

- `<space>`
- `<unit>`
- `<space/target>`
- `<target-ref>`

they mean:

- these commands are real
- but the specific connected object name depends on the user’s environment

To discover the actual value, prefer:

```bash
cub space list --json
cub target list --space "*" --json
```

## 11. What To Do If A User Says “Can You Access ConfigHub?”

Do not answer by trying to fetch the web app.

Instead:

1. say that live ConfigHub access is through `cub`
2. run a read-only CLI check such as:

```bash
cub context list --json
cub space list --json
```

3. report what you found
4. only then ask whether they want you to inspect or mutate a specific example

## 12. Skills / Special Instructions

There are no repo-local “skills” files in this repo that you need to load first.

Your main tools here are:

- the example READMEs
- the shell scripts in each example
- the `cub` CLI
- JSON output from `cub`

## 13. Next Step

If you are starting fresh, the best single next step is:

```bash
cd <your-examples-checkout>
./scripts/verify.sh
```

Then choose one path:

- human-friendly overview: [START_HERE.md](./START_HERE.md)
- incubator AI path: [incubator/AI_START_HERE.md](./incubator/AI_START_HERE.md)
- layered recipes and AICR mapping: [incubator/global-app-layer/README.md](./incubator/global-app-layer/README.md)
