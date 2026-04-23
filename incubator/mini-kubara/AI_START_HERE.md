# AI Start Here: Mini-Kubara

Use this page when you want to drive the four Mini-Kubara cases safely from the public `confighub/examples` repo.

## CRITICAL: Demo Pacing

Pause at every gate. These cases are training drills for safe ConfigHub + GitOps operation, so the pauses are part of the product experience.

After each stage:

1. show the command output faithfully;
2. explain what changed or what was only read;
3. print the ConfigHub GUI URL when one exists;
4. say what the GUI proves and what it does not prove;
5. stop for Y/N before ConfigHub writes or live cluster writes.

## Suggested Prompt

```text
Read incubator/mini-kubara/AI_START_HERE.md and incubator/mini-kubara/README.md.
Then run Mini-Kubara Case 01 only, using incubator/mini-kubara/case-01-clean-handoff/PROMPT.md.
Start read-only, verify auth/cluster/worker/target state before writes, show the ConfigHub GUI URL before Gate A, and stop before every ConfigHub or live-cluster mutation for Y/N.
Do not run Cases 02-04 until Case 01 is summarized.
```

## What This Example Teaches

Mini-Kubara is a small, public training pack for ConfigHub + Argo handoff patterns. It exists so fresh AI sessions can practice the same lessons from the larger Kubara run without depending on private repo access.

## Public Source Requirement

Case 01's generated Argo Application reads its workload from this public repo:

```text
repoURL: https://github.com/confighub/examples
path: incubator/mini-kubara/case-01-clean-handoff/fixtures/workloads
```

That public source is intentional. Do not point the fixture back at a private repo, because a fresh Argo install in a kind cluster will not have private GitHub credentials.

## Stage 1: Read-Only Orientation

Read:

```bash
git rev-parse --show-toplevel
sed -n '1,180p' incubator/mini-kubara/README.md
sed -n '1,220p' incubator/mini-kubara/case-01-clean-handoff/PROMPT.md
```

What to explain:

- which case is being run;
- which repo/path Argo will read;
- whether the current checkout already has the fixture files.

GUI now: none yet; this is repo-only orientation.

GUI gap: no ConfigHub objects exist until the case setup creates or finds a space.

Pause before moving into live preflight.

## Stage 2: Live Preflight

Follow the selected case prompt. For Case 01, verify ConfigHub auth, cluster context, Argo, worker, target, and `cub-scout` with the exact kubeconfig/context before asking for Gate A.

This stage is read-only.

GUI now: once the ConfigHub space is identified or created by setup, print the space URL before Gate A approval.

Pause before Gate A.

## Stage 3: Gate A

Ask Y/N before the first ConfigHub write. Case 01 Gate A creates or updates exactly one governed ApplicationSet unit, then applies it through ConfigHub.

Prove:

- ConfigHub unit receipt;
- ApplicationSet exists;
- generated Application exists and is owned by the ApplicationSet;
- persistent auto-sync remains off;
- ConfigHub GUI URL is available for the unit.

Pause before Gate B.

## Stage 4: Gate B

Ask Y/N before the one-shot Argo sync. Case 01 Gate B syncs exactly `Application/controlplane-mini-clean`.

Prove:

- Argo `Synced` / `Healthy` / operation succeeded;
- workload pod is Running/Ready in `mini-clean`;
- `cub-scout` agrees using the exact kubeconfig/context.

Stop on `Failed`, `Error`, `Degraded`, wrong app, wrong namespace, or out-of-scope mutation.

## Stage 5: Closeout

Use the selected case prompt closeout section. Include what was verified directly, what was inferred, what was not proven, and the next truthful step.

Do not continue into another Mini-Kubara case without a new prompt.

## Run Order

1. Case 01: Clean Handoff - green baseline.
2. Case 03: Policy Admission - WATCH to BLOCK discipline.
3. Case 02: Large CRDs - CRD-heavy delivery mechanics.
4. Case 04: Dev/Prod Variants - explicit variant and promotion thinking.

Run them in sequence, not in parallel.

## Cases

- [Case 01: Clean Handoff](./case-01-clean-handoff/GUIDE.md)
- [Case 02: Large CRDs](./case-02-large-crds/GUIDE.md)
- [Case 03: Policy Admission](./case-03-policy-admission/GUIDE.md)
- [Case 04: Dev/Prod Variants](./case-04-dev-prod-variants/GUIDE.md)
- [Contracts](./contracts.md)
