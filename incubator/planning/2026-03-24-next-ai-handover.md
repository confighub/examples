# Next AI Handover: Examples Mission, AI-First Rules, Testing, and Plan

**Date:** 2026-03-24  
**Repo:** `/Users/alexis/Public/github-repos/examples`  
**Current `origin/main`:** `a28ab7b680cb32a2b0465dcf663de821a97eed5d`

For the current OCI-standardization, AICR-bundle, and focused today sequence, also read:

- `incubator/planning/2026-03-28-oci-standard-aicr-bundles-and-today-plan.md`

## Mission

The current mission for `confighub/examples` is:

> Keep building AI-first examples in `examples/incubator` that give one person a fast reason to use ConfigHub, especially by adapting the best `cub-scout` flows into official, evidence-first examples.

Important boundary:

- keep examples in `incubator/` unless there is an explicit user decision to promote them
- do not infer promotion from validation quality alone
- improving an example and promoting an example are separate actions

## Non-Negotiable Working Rules

### 1. AI-first is mandatory

Follow the pacing and presentation rules from:

- `AI_START_HERE.md`
- `incubator/AI_START_HERE.md`
- `incubator/ai-example-playbook.md`
- `~/Desktop/advisory-ai-demo-pacing.md`

That means:

- use stage-based demo structure
- pause after every stage
- print full output, not summaries only
- explicitly say what mutates and what does not
- when a GUI checkpoint exists, include:
  - `GUI now`
  - `GUI gap`
  - `GUI feature ask`
- tell the human to stop and inspect before continuing

### 2. Keep a strict read-only-first habit

For any example, start with:

- `./setup.sh --explain`
- `./setup.sh --explain-json | jq`

Do not jump straight to the mutating path unless the user explicitly wants that.

### 3. Use clean worktrees from `origin/main`

Do not rely on a dirty local checkout.

Use:

```bash
git fetch origin
git worktree add -b codex/<topic> /private/tmp/<worktree> origin/main
```

### 4. Prefer dedicated kubeconfigs for live examples

If an example creates a local cluster, it should use a local kubeconfig under `var/`.

Do not rely on ambient `~/.kube/config`.

### 5. File issues instead of smoothing over uncertainty

If a `cub-scout` command contract is weaker than the example claims, keep the example honest and file an upstream issue instead of pretending it works cleanly.

## Current Example Map

### No-cluster evidence and offline paths

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/connected-summary-storage`
- `incubator/artifact-workflow`
- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/lifecycle-hazards`

### Live import, ownership, topology, and reporting paths

- `incubator/import-from-live`
- `incubator/graph-export`
- `incubator/combined-git-live`
- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`
- `incubator/custom-ownership-detectors`
- `incubator/orphans`
- `incubator/watch-webhook`
- `incubator/flux-boutique`
- `incubator/platform-example`

### App-style GitOps examples

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

### Structural and advanced examples

- `incubator/springboot-platform-app`
- `incubator/global-app-layer`
- `incubator/cub-proc`
- `incubator/vmcluster-from-scratch.md`
- `incubator/vmcluster-nginx-path.md`

## Testing Strategy

### First: no-cluster examples

Use these as the fastest signal:

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/connected-summary-storage`
- `incubator/artifact-workflow`
- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/lifecycle-hazards`

For each:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
./cleanup.sh
```

Record:

- pass or fail
- exact commands run
- what mutated, if anything
- evidence observed
- any mismatch between docs and runtime

### Second: smaller live examples

Use these before bigger controller-heavy stories:

- `incubator/import-from-live`
- `incubator/graph-export`
- `incubator/watch-webhook`
- `incubator/orphans`
- `incubator/custom-ownership-detectors`
- `incubator/flux-boutique`

Same command shape:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh
./verify.sh
./cleanup.sh
```

Special focus:

- dedicated kubeconfig behavior
- no hidden dependency on ambient kube state
- README and contracts still matching the runtime

### Third: broader live examples

- `incubator/combined-git-live`
- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`
- `incubator/platform-example`

These are valuable, but they have more moving parts. Test them after the smaller live set.

### Fourth: app-style GitOps examples

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

Use a clean branch-backed validation when repo paths matter:

```bash
EXAMPLES_GIT_REVISION=<branch> ./setup.sh
```

Minimum evidence to collect:

For Flux:

```bash
kubectl get gitrepositories,kustomizations -n flux-system
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
flux get sources git -A
flux get kustomizations -A
```

For Argo:

```bash
kubectl get applicationsets,applications -n argocd
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
```

## Known Upstream Caveats

- `confighub/cub-scout#331` for `scan --file --json` exit semantics
- `confighub/cub-scout#332` for `drift` docs and command contract mismatch
- `confighub/cub-scout#333` for custom ownership detectors not applying consistently across `map`, `explain`, and `trace`
- `confighub/cub-scout#334` for `trace` misclassifying a native Deployment as Flux in the `orphans` fixture
- `confighub/cub-scout#335` for `map orphans` not surfacing the fixture CronJob
- `confighub/cub-scout#336` for the `fleet-demo` README overclaiming current bundle differences
- `confighub/cub-scout#339` for the original `platform-example` Flux source depending on `ArtifactGenerator`
- `confighubai/confighub#4025` for dedicated kubeconfig use across live examples and incubator workflows

Do not build a new example around a known broken contract without linking the issue and keeping the claim narrow.

## Highest-Value Next Work

### 1. Keep improving AI-first behavior

Highest-priority AI-first follow-through:

- make sure the remaining `AI_START_HERE.md` files fully follow the newer pacing guidance
- especially:
  - `incubator/springboot-platform-app/AI_START_HERE.md`
  - `incubator/global-app-layer/AI_START_HERE.md`
  - any `global-app-layer` subexample AI guides

### 2. Continue selective `cub-scout` adaptation

Only keep adapting examples from `cub-scout` if they add a genuinely new operator or workflow story.

Good adaptations are:

- small
- evidence-first
- easy to validate
- clear about mutation boundaries

### 3. Keep the AICR bundle story getting more concrete

Current AICR bundle material now includes:

- `incubator/global-app-layer/04-bundles-attestation-and-todo.md`
- `incubator/global-app-layer/05-bundle-publication-walkthrough.md`
- `incubator/global-app-layer/06-bundle-evidence-gui-spec.md`
- `incubator/global-app-layer/bundle-evidence-sample/`

Next likely AICR steps:

- make the bundle sample more runnable against a real generated bundle
- make the GUI spec more product-shaped
- keep the distinction clear between:
  - bundle publication
  - integrity evidence
  - SBOMs and attestations
  - downstream handoff
- standardize the controller-oriented bundle path around OCI
- keep `FluxOCI` as the current standard controller path
- keep `ArgoCDRenderer` clearly separate from a future Argo OCI path

### 4. Keep `springboot-platform-app` in view

Important local handover notes existed outside git and have now been intentionally replaced by this tracked handover.

The key open `springboot-platform-app` themes to remember are:

- it is the authority-vs-provenance example
- it already has structural, ConfigHub-only, noop-target, and read-only lift/block proofs
- the next meaningful step is still a real cluster target proof
- field-level block/escalate and true lift-upstream PR automation remain product gaps

Use the existing example docs and `V2-LIVE-PLAN.md` in that example directory as the current tracked source of truth.

## Plan For The Next AI Session

### Phase 1

- execute the focused front-door and OCI planning sequence from `2026-03-28-oci-standard-aicr-bundles-and-today-plan.md`

### Phase 2

- if docs and contracts were updated, re-run only the smallest affected examples and collect a short evidence log

### Phase 3

- check the remaining AI guides for pacing quality, GUI honesty, and delivery-mode clarity

### Phase 4

- only after the OCI/controller path is clearer, pick one next implementation slice

### Phase 5

- revisit whether any example should ever be promoted, but only after an explicit user request

## Expected Output Style For The Next AI

The next AI should produce:

- concise evidence logs
- exact commands run
- explicit mutation notes
- specific doc/runtime mismatches
- issue links when contracts are weak

It should not:

- silently promote examples
- treat validation quality as permission to change repo structure
- skip the AI-first pacing rules for demos
