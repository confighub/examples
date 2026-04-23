# Fresh Claude Prompt: Mini-Kubara Case 02

Use this as the complete prompt for a fresh Claude session. Run this case only
after Case 01 has stopped and been summarized. Do not run mini-Kubaras in
parallel.

```text
You are running Mini-Kubara Case 02: Large CRDs from the confighub/examples repo at incubator/mini-kubara.

Goal:
Prove the large-CRD delivery contract: detect annotation-size/apply-mode risk
before a workload install, choose an explicit delivery path, and prove CRDs,
controller readiness, and a dependent custom resource. Do not rediscover the
external-secrets failure halfway through a sync.

Read first:
- AGENTS.md
- incubator/mini-kubara/README.md
- incubator/mini-kubara/case-02-large-crds/GUIDE.md
- incubator/AI_START_HERE.md
- incubator/ai-guide-standard.md
- incubator/standard-ai-demo-pacing.md

Operating rules:
- You are fresh. Do not assume kind, Argo, ConfigHub auth, workers, targets,
  kubeconfig, fixtures, setup scripts, CRDs, or admission controllers are
  ready.
- The user will not manually touch kind clusters, GitOps controllers, workers,
  or CRDs for you. You own read-only discovery, setup detection, and blocker
  reporting.
- Do not run auth login. If ConfigHub auth is expired and live ConfigHub work
  is needed, ask the user to run cub auth login and stop.
- Check current cub/cub-scout command surfaces with help before relying on a
  flag or verb.
- Use cub-scout with the exact kubeconfig/context used for kubectl.
- One-shot the run if possible, but do not break approval scope. Stop for Y/N
  before ConfigHub writes, live cluster writes, or any CRD apply-mode bypass.
- Do not mutate other public examples. Only create or edit files under this
  case directory if fixture authoring is necessary; stop before live mutation
  and report the diff.
- If fixture manifests or setup/verify scripts are missing, create the missing
  repo-local files under this case directory first. Do not invent that a live
  cluster already has them.

Initial preflight, read-only:
1. Print repo path and git status.
2. Read the case guide and list expected fixture/setup files.
3. Check cub auth/API reachability with cub space list or another
   authenticated read. Stop for cub auth login if needed.
4. Check kubectl current context and cluster reachability. If no reachable
   cluster exists, inspect local setup options and stop with a blocker or setup
   proposal.
5. Check cub-scout help and run cub-scout doctor with the exact kubeconfig or
   context if cluster access exists.
6. Read current CRDs and admission webhook state. This is read-only.

CRD preflight:
- Count CRDs in the fixture.
- Identify any CRD likely to exceed the Kubernetes 262144 byte annotation
  limit if client-side last-applied configuration is stamped.
- State whether the intended delivery path is client-side apply, server-side
  apply, ConfigHub/Argo sync option, or a separately approved dev setup
  live-write.
- If the apply mode cannot be determined from current command surfaces, stop
  before mutation and report exactly what is unknown.

If local fixtures are missing:
- Create a tiny local fixture under this case directory with:
  - one ordinary CRD;
  - one intentionally large CRD risk fixture;
  - one harmless dependent custom resource;
  - one small controller-style Deployment or placeholder workload.
- Keep the fixture deterministic and local. Do not pull live upstream CRDs
  unless explicitly approved.
- Stop before live mutation if the setup path is not bounded.

Route card before any approval:
Use CONFIGHUB SAYS: ROUTE or ./scripts/confighub-banner route if available.
Include:
- Lane: CH-WRITE for governed ApplicationSet/controller delivery; RECOVERY or
  SETUP for any special CRD apply mode.
- Scope: exactly the mini-large-crds provider and dependent resource.
- Wrong move: retrying a known annotation-size failure without changing apply
  mode; calling a direct CRD workaround governed workload proof.
- WATCH tripwires: admission denial, CRD annotation-size failure, missing CRD
  after apply, dependent resource attempted before CRD exists.

Gate A: Large-CRD delivery decision
Ask Y/N before any mutation. The approval must name the selected apply mode.
If approved:
- perform only the selected CRD/provider delivery path;
- prove both CRDs exist;
- prove no annotation-size rejection occurred;
- prove ConfigHub action/event status if ConfigHub was used;
- classify clearly whether any direct live CRD action was setup, not governed
  workload proof.

Gate B: Dependent resource/controller proof
Ask a separate Y/N if a live sync or apply is needed after CRDs exist.
If approved:
- create/sync only the dependent custom resource and controller fixture;
- prove the API accepts the custom resource;
- prove controller pod Running/Ready if the fixture includes a controller;
- run cub-scout doctor with the exact kubeconfig/context.

Closeout:
Use the proof closeout template. Include:
- the CRD apply mode chosen;
- ConfigHub receipt if applicable;
- Argo/controller receipt if applicable;
- cluster CRD receipt;
- dependent custom resource receipt;
- whether this is governed proof or a separately approved dev setup path;
- not proven;
- next truthful step.

Do not proceed to Case 03, Case 04, promotion, public example graduation, or
full external-secrets. End with a concise report and git status.
```
