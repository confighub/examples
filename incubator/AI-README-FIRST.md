# Incubator AI README First

This is the fuller AI onboarding guide for the `incubator/` area.

If you only need the shortest protocol, start with [AGENTS.md](./AGENTS.md).

For the shared read-only evaluation flow, see [ai-machine-seams-first.md](./ai-machine-seams-first.md).

## 1. What This Is

The incubator contains experimental ConfigHub examples before promotion to stable examples.

Use it for:
- trying new example structures
- testing AI and human walkthroughs
- proving out layered recipe or delivery ideas

Do not assume every incubator example is production-shaped.

## 2. What You Need To Know First

### Trust order

When example behavior matters, trust in this order:

1. the example README
2. the example's AI guide, if it has one
3. the example scripts, especially `--explain` paths
4. current `cub --help`

### The key split

Most incubator examples have at least two possible modes:

1. ConfigHub database only
2. live target delivery

Do not blur them together.

Say clearly whether a step:
- only creates or verifies ConfigHub objects
- or also reaches a live target or GitOps controller

## 3. Cold-Start Checklist

Start here:

```bash
cd <your-examples-checkout>
git rev-parse --show-toplevel
./scripts/verify.sh
rg --files incubator
```

If connected read-only inspection is wanted:

```bash
cub version
cub context list
cub space list --json
cub target list --space "*" --json
```

Avoid guessing unsupported commands. In particular:
- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`

## 4. Capability Branching

### A. Repo-only mode

Use this if the user wants explanation, doc review, or script walkthrough only.

Safe actions:
- read docs
- use `--help`
- use `--explain`
- use `--explain-json`

### B. ConfigHub-only mode

Use this if auth works but no worker/target is available.

Safe actions:
- create or inspect spaces and units
- verify example state in ConfigHub
- stop before apply

### C. Live target mode

Use this only when a real target and worker are available.

Safe actions:
- bind target
- apply units
- verify live state

## 5. Auth Gate For ConfigHub Mutation

Treat ConfigHub auth as a hard gate before any ConfigHub-mutating step.

Run a read-only auth check first:

```bash
cub info
```

If auth is missing or expired:

- stop before worker install, discover, import, apply, or other ConfigHub mutation
- tell the human to run `cub auth login`
- rerun only the blocked step after auth is fixed

Do not:

- keep retrying the same failing step in the background
- rerun the full example from the beginning unless another step actually needs to be repeated
- confuse an auth block with a cluster or controller failure

## 6. How To Walk A User Through An Incubator Example

For each stage:

1. explain what the example proves
2. explain the next step
3. say what it reads
4. say what it writes
5. say what success should look like
6. verify immediately after the step

Good phrasing:

- “This only reads the example and ConfigHub state; it does not mutate anything.”
- “This creates spaces and units in ConfigHub, but it does not apply to a cluster yet.”
- “This step binds the deployment units to a real target, so from here on we are touching live delivery.”

## 7. Important Review Questions

When you review or improve an incubator example, make sure the docs answer:

1. what stack is this for?
2. what do I need installed?
3. what does this read?
4. what does this write?
5. what should I expect to see?
6. how would an AI assistant run this safely?

## 8. Reusable Standard

For reusable incubator best practices, read:

- [ai-machine-seams-first.md](./ai-machine-seams-first.md)
- [ai-cold-eval-prompt-pack.md](./ai-cold-eval-prompt-pack.md)
- [ai-example-playbook.md](./ai-example-playbook.md)
- [ai-example-template.md](./ai-example-template.md)
