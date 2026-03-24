# Claude Testing Handover: Incubator Examples

**Date:** 2026-03-22
**Goal:** give a fresh Claude session a clean testing brief for the current incubator set in `confighub/examples`

## Ground Rules

Use the canonical repo paths under `/Users/alexis/Public/github-repos/examples`.

Do not use `cub-scout` as the primary surface. Use it only where the incubator example explicitly depends on the `cub-scout` binary.

Do not mutate the dirty local checkout in `/Users/alexis/Public/github-repos/examples` if it is not clean. If a clean worktree is needed, create one from `origin/main` and report only canonical `examples` paths in user-facing notes.

Start read-only whenever possible.

For live examples that create local clusters, prefer examples that already use dedicated kubeconfig files under a local `var/` directory. Do not rely on ambient `~/.kube/config` state.

## Current State

These are already merged on `main` and should be treated as the current canonical incubator set for this testing pass.

### No-cluster examples

- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/lifecycle-hazards`
- `incubator/artifact-workflow`

Stable no-cluster examples already promoted:

- `connect-and-compare`
- `import-from-bundle`
- `connected-summary-storage`

### Mixed or live-evidence examples

- `incubator/combined-git-live`
- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`
- `incubator/custom-ownership-detectors`
- `incubator/orphans`
- `incubator/watch-webhook`
- `incubator/flux-boutique`

Stable live-evidence examples already promoted:

- `import-from-live`
- `graph-export`

### App-style examples now validated live

- `incubator/apptique-argo-app-of-apps`

Stable app-style example already promoted:

- `apptique-flux-monorepo`
- `apptique-argo-applicationset`

## Highest-Priority Testing Tasks

### 1. Re-run the no-cluster examples cleanly

These should stay green and are the fastest signal.

For each of these:

- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/lifecycle-hazards`
- `incubator/artifact-workflow`

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

### 2. Re-run the smaller live examples first

These now form the best live smoke-test set before the bigger controller stories.

Targets:

- `incubator/watch-webhook`
- `incubator/orphans`
- `incubator/custom-ownership-detectors`
- `incubator/flux-boutique`

For each:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
./cleanup.sh
```

What to record:

- whether the dedicated-kubeconfig pattern still works cleanly
- whether the evidence captured by the example still matches the current command output
- whether any README or contract text drifts from runtime behavior

### 3. Re-run `combined-git-live`

This remains valuable, but it is broader than the smaller live examples above.

Targets:

- `incubator/combined-git-live`

What to record:

- whether the fixture apply path behaves as documented
- whether the dry-run or compare result still matches the expected output
- whether any live cluster dependencies have become flaky

### 4. Re-check the app-style examples after the self-contained pass

The app-style set has now had one clean self-contained live validation pass. The next testing pass should confirm that it stays stable on fresh machines.

Targets:

- `incubator/apptique-argo-app-of-apps`

What to do:

- prefer one clean cluster per controller family
- verify with direct evidence, not assumptions
- confirm the dedicated-kubeconfig and self-install flow still behaves exactly as documented

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

### 5. Check top-level routing only after example testing

Once the examples themselves are verified, sanity-check that these still point to the right places:

- `README.md`
- `AI_START_HERE.md`
- `START_HERE.md`
- `incubator/README.md`
- `incubator/AI_START_HERE.md`

## Known Upstream Caveats

The following upstream issues were discovered during the current adaptation work.

- `confighub/cub-scout#331` for `scan --file --json` exit semantics
- `confighub/cub-scout#332` for `drift` docs and command contract mismatch
- `confighub/cub-scout#333` for custom ownership detectors applying in `map list` but not consistently in `explain` or `trace`
- `confighub/cub-scout#334` for `trace` misclassifying a native Deployment as Flux in the `orphans` fixture
- `confighub/cub-scout#335` for `map orphans` not surfacing the fixture CronJob
- `confighub/cub-scout#336` for the `workflows/fleet-demo` README overclaiming differences in the current prebuilt bundles
- `confighubai/confighub#4025` for adopting dedicated kubeconfig files across live examples and incubator workflows

Meaning:

- examples that capture JSON from `scan --file --json` must tolerate exit code `1` when findings are present
- do not promote a `drift` example into `examples` yet
- treat the current `custom-ownership-detectors`, `orphans`, and `fleet-demo` stories as evidence-based and issue-linked rather than perfect happy paths

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
- the smaller live examples re-verified with dedicated kubeconfigs
- at least one clean re-validation pass for the app-style set on a fresh runtime
- a short list of any doc or script fixes discovered during that testing
