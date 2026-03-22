# Claude Testing Handover: Incubator Examples

**Date:** 2026-03-22
**Goal:** give a fresh Claude session a clean testing brief for the current incubator set in `confighub/examples`

## Ground Rules

Use the canonical repo paths under `/Users/alexis/Public/github-repos/examples`.

Do not use `cub-scout` as the primary surface. Use it only where the incubator example explicitly depends on the `cub-scout` binary.

Do not mutate the dirty local checkout in `/Users/alexis/Public/github-repos/examples` if it is not clean. If a clean worktree is needed, create one from `origin/main` and report only canonical `examples` paths in user-facing notes.

Start read-only whenever possible.

## Current State

These are already merged on `main` and should be treated as the current canonical incubator set for this testing pass:

### No-cluster examples

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/fleet-import`
- `incubator/demo-data-adt`

### Mixed or live-evidence examples

- `incubator/combined-git-live`
- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`

### App-style examples still needing stronger live validation confidence

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

## Highest-Priority Testing Tasks

### 1. Re-run the no-cluster examples cleanly

These should be easy to verify first and should remain green.

For each of these:

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/fleet-import`
- `incubator/demo-data-adt`

Run:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
./cleanup.sh
```

What to record:

- whether the flow works exactly as documented
- whether `verify.sh` is strong enough
- whether the README claims match the actual output

### 2. Re-run `combined-git-live`

This example is live-optional and depends on a reachable cluster.

If a healthy disposable local cluster is available, run:

```bash
cd incubator/combined-git-live
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
./cleanup.sh
```

What to record:

- whether the fixture apply path behaves as documented
- whether the compare result still shows:
  - 4 aligned
  - 1 git-only
  - 1 cluster-only
- whether the README still matches the current `cub-scout combined` output

### 3. Resume live validation of the app-style examples

This is the main unfinished testing task.

Targets:

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

What happened last time:

- the examples were structurally validated
- fresh temporary kind clusters were created
- Docker or kind became sticky during follow-up probing and verification
- the work stopped rather than claiming a live proof that the environment did not support

What to do now:

- only proceed if Docker and kind are healthy
- prefer one clean cluster per controller family
- verify with direct evidence, not assumptions

Minimum evidence to collect:

For Flux:

```bash
kubectl get gitrepositories,kustomizations -n flux-system
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
flux get sources git -A
flux get kustomizations -A
```

For Argo ApplicationSet:

```bash
kubectl get applicationsets,applications -n argocd
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
```

For Argo app-of-apps:

```bash
kubectl get applications -n argocd
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
```

If `cub-scout` is available and the cluster is stable, also record ownership or provenance checks.

### 4. Check top-level routing only after example testing

Once the examples themselves are verified, sanity-check that these still point to the right places:

- `README.md`
- `AI_START_HERE.md`
- `START_HERE.md`
- `incubator/README.md`
- `incubator/AI_START_HERE.md`

## Known Upstream Caveats

Do not spend time trying to promote or validate a `drift` example in `examples` yet.

Two upstream issues were found and filed during this work:

- `confighub/cub-scout#331`
- `confighub/cub-scout#332`

Meaning:

- `scan --file --json` currently returns exit code `1` when findings are present, so examples need to tolerate that if they capture JSON output
- the `drift` example docs currently overclaim beyond the documented `cub-scout drift` contract

## Desired Output From Claude

The most useful testing report is not a narrative. It should be a short evidence log.

For each example tested, capture:

- pass or fail
- exact commands run
- what mutated, if anything
- what evidence was observed
- any mismatch between docs and runtime
- whether a code or doc fix is needed

## Good Stopping Point

A good stopping point for the next Claude session is:

- all no-cluster examples re-verified
- at least one clean live validation pass for the app-style set, if the runtime is healthy
- a short list of any doc or script fixes discovered during that testing
