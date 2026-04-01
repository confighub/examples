# Planning And Milestones

Use this page as the shortest current map of the incubator planning docs.

## Read This First

If you only read three planning docs, read these:

1. [2026-03-24-next-ai-handover.md](./2026-03-24-next-ai-handover.md)
2. [2026-03-28-oci-standard-aicr-bundles-and-today-plan.md](./2026-03-28-oci-standard-aicr-bundles-and-today-plan.md)
3. [2026-03-27-new-user-sense-check-and-plan.md](./2026-03-27-new-user-sense-check-and-plan.md)

For AI-first pacing and rollout:

- [../standard-ai-demo-pacing.md](../standard-ai-demo-pacing.md)
- [2026-03-29-ai-first-demo-pacing-rollout-issue-draft.md](./2026-03-29-ai-first-demo-pacing-rollout-issue-draft.md)

## Operating Principles

These rules apply across all examples work:

1. **Real end-to-end means every step is real.** ConfigHub mutation, apply, target delivery, running app, and live verification must all be genuine.
2. **Noop targets are opt-in only.** They must never be presented as real e2e proof.
3. **If an example is not freshly proven, the docs must say so plainly.** Do not assume docs equal proof.
4. **Keep incubator work in incubator.** Do not promote examples to root unless explicitly asked.
5. **Prefer short, honest, example-local docs** over repo-wide taxonomy sprawl.

## Gold Standard For The Next Incubator Pass

After the Spring Boot flagship work, use this as the quality bar for the next incubator cleanup pass:

1. **Examples should be real, useful, and end-to-end by default.** If an example intentionally stops short of full delivery, the docs must name the business reason plainly.
2. **A human should be able to land on the example in GitHub and understand it without separate docs.** The example README should explain the purpose, the lesson, the safest first step, and the honest proof boundary.
3. **AI should be able to drive the demo as well as a human.** `AI_START_HERE.md`, `--explain`, `--explain-json`, and `contracts.md` should make the example workable as a one-prompt, stage-based walkthrough.
4. **The examples should be lovable, not just correct.** They should create value quickly, answer “why does this matter?” early, and leave both users and the team feeling that the example taught something real.

## Current Milestones

### Milestone 1: Re-prove the Recent Real Examples

**Status (2026-03-30):** Direct Kubernetes and ArgoCD import proven. FluxOCI blocked by OCI endpoint (404).

Goal:

- capture fresh evidence for the examples we now describe as real or nearly real

Main examples:

- `incubator/springboot-platform-app` — ✅ proven (full mutation chain: ConfigHub → deployment → HTTP)
- `incubator/gitops-import-argo` — ✅ proven (ArgoCD healthy, both guestbook apps Synced+Healthy)
- `incubator/gitops-import-flux` — ✅ proven (Flux works, FluxOCI worker installs with updated cub v0.1.24)
- `incubator/global-app-layer/single-component` with `FluxOCI` — ❌ blocked (OCI endpoint 404, see [#4088](https://github.com/confighubai/confighub/issues/4088))
- `incubator/global-app-layer/single-component` with `ArgoCDOCI` — ❌ blocked (same OCI endpoint issue)
- `incubator/global-app-layer/gpu-eks-h100-training` with `FluxOCI` — ❌ blocked (same OCI endpoint issue)
- `incubator/global-app-layer/gpu-eks-h100-training` with `ArgoCDOCI` — ❌ blocked (same OCI endpoint issue)

Key blocker: OCI Distribution API at `oci.hub.confighub.com` returns 404. Worker creates Flux resources correctly but Flux cannot pull artifacts.

Known issue: Worker apply bug discovered - worker reports success but doesn't apply changes. Manual kubectl apply works.

Supporting docs:

- [2026-03-29-milestone1-proof-pass-results.md](./2026-03-29-milestone1-proof-pass-results.md) — **latest results and lessons learned**
- [2026-03-24-next-ai-handover.md](./2026-03-24-next-ai-handover.md)
- [2026-03-22-claude-testing-status-and-todo.md](./2026-03-22-claude-testing-status-and-todo.md)
- [2026-03-28-oci-standard-aicr-bundles-and-today-plan.md](./2026-03-28-oci-standard-aicr-bundles-and-today-plan.md)

### Milestone 2: Apply the Spring Lessons Across the Repo

Goal:

- take what worked in the successful Spring Boot examples and apply it to other incubator and `cub-gen`-style examples

That means:

- one clear reason for existing
- read-only first
- real versus simulated paths separated cleanly
- explicit proof boundaries
- better AI-first pacing
- verification that matches the claims

Supporting docs:

- [../../spring-platform/springboot-platform-app/README.md](../../spring-platform/springboot-platform-app/README.md)
- [../../spring-platform/springboot-platform-app-centric/README.md](../../spring-platform/springboot-platform-app-centric/README.md)
- [2026-03-27-new-user-sense-check-and-plan.md](./2026-03-27-new-user-sense-check-and-plan.md)

### Milestone 3: Make AI-First the Default Story

Goal:

- make every important example work as a one-prompt, stage-based, pause-and-show walkthrough

Scope:

- all 31 `AI_START_HERE.md` files
- incubator examples first
- root and `cub-gen`-style examples after that

Supporting docs:

- [../standard-ai-demo-pacing.md](../standard-ai-demo-pacing.md)
- [2026-03-29-ai-first-demo-pacing-rollout-issue-draft.md](./2026-03-29-ai-first-demo-pacing-rollout-issue-draft.md)

### Milestone 4: Standardize the OCI Controller Story

**Status (2026-03-29):** Documentation complete. All delivery matrices standardized. Live proof pending Milestone 1 blockers.

Goal:

- keep direct Kubernetes as the simplest proof
- keep Flux OCI as the current standard controller path
- keep Argo OCI honest and evidence-backed where it exists
- tighten the bundle language around target-specific output

Completed:

- Delivery matrix standardized across all layered docs
- Argo OCI status changed from "in selected examples" to "Implemented"
- ConfigHub-native OCI origin language applied consistently
- ArgoCDRenderer clearly distinguished from Argo OCI delivery

Supporting docs:

- [2026-03-28-oci-standard-aicr-bundles-and-today-plan.md](./2026-03-28-oci-standard-aicr-bundles-and-today-plan.md)
- [../proposal-oci-api-confighub.md](../proposal-oci-api-confighub.md)
- [../global-app-layer/07-argo-oci-spec.md](../global-app-layer/07-argo-oci-spec.md)

### Milestone 5: Make NVIDIA and AICR Real, Not Just Well-Explained

Goal:

- keep the current layered recipe story
- add a real GPU-capable proof path
- tighten the bundle publication and downstream consumption story around real evidence

Supporting docs:

- [2026-03-28-oci-standard-aicr-bundles-and-today-plan.md](./2026-03-28-oci-standard-aicr-bundles-and-today-plan.md)
- [2026-03-27-new-user-sense-check-and-plan.md](./2026-03-27-new-user-sense-check-and-plan.md)
- [../global-app-layer/04-bundles-attestation-and-todo.md](../global-app-layer/04-bundles-attestation-and-todo.md)
- [../global-app-layer/05-bundle-publication-walkthrough.md](../global-app-layer/05-bundle-publication-walkthrough.md)

### Milestone 6: Decide the Bounded-Procedure Story

Goal:

- clarify whether the future operational record story is `cub-proc`, `cub run`, or some merged shape

Important boundary:

- do not make the current examples depend on `cub-proc`

Supporting docs:

- [../cub-proc/README.md](../cub-proc/README.md)
- [../cub-proc/03-cub-proc-prd.md](../cub-proc/03-cub-proc-prd.md)
- [../cub-proc/03-cub-proc-rfc.md](../cub-proc/03-cub-proc-rfc.md)
- [../cub-proc/procedure-candidates.md](../cub-proc/procedure-candidates.md)

## Secondary And Historical Docs

These are still useful, but they are not the first place to start:

- [2026-03-22-incubator-examples-roadmap.md](./2026-03-22-incubator-examples-roadmap.md)
- [2026-03-23-incubator-promotion-shortlist.md](./2026-03-23-incubator-promotion-shortlist.md)
- [2026-03-20-ai-handover-mainstream-runnable-examples.md](./2026-03-20-ai-handover-mainstream-runnable-examples.md)
- [2026-03-20-mainstream-runnable-examples-from-cub-scout.md](./2026-03-20-mainstream-runnable-examples-from-cub-scout.md)
- [2026-03-17-label-mapping-convention.md](./2026-03-17-label-mapping-convention.md)
- [2026-03-18-cli-label-query-gap.md](./2026-03-18-cli-label-query-gap.md)
