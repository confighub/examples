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
- incubator/mini-kubara/case-02-large-crds/fixtures/README.md
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
- Do not mutate other public examples. Only create or edit files under this case directory if fixture authoring is necessary; stop before live mutation and report the diff.
- The repo already ships deterministic Case 02 fixtures under
  incubator/mini-kubara/case-02-large-crds/fixtures. Do not regenerate or
  overwrite them mid-run. If a fixture appears missing, stop and report the
  blocker; do not invent a replacement.

Initial preflight, read-only:
1. Print repo path and git status.
2. Read the case guide and the fixtures README and list the expected fixture
   and helper files.
3. Run the repo-local preflight helper FIRST, in --explain mode, then in the
   default read-only form:
     incubator/mini-kubara/case-02-large-crds/preflight.sh --explain
     incubator/mini-kubara/case-02-large-crds/preflight.sh
   The helper is read-only: it counts fixture CRDs, measures annotation-size
   risk, and states the intended apply mode. Do not treat a successful
   preflight as Gate A approval.
4. Check cub auth/API reachability with cub space list or another
   authenticated read. Stop for cub auth login if needed.
5. Check kubectl current context and cluster reachability. If no reachable
   cluster exists, inspect local setup options and stop with a blocker or
   setup proposal; do not create a kind cluster silently.
6. Check cub-scout help and run cub-scout doctor with the exact kubeconfig or
   context if cluster access exists. Only use flags the current help output
   confirms.
7. Read current CRDs and admission webhook state on the target cluster. This
   is read-only.

CRD preflight:
- Count CRDs in the fixture (the helper does this).
- Identify any CRD likely to exceed the Kubernetes 262144 byte annotation
  limit if client-side last-applied-configuration is stamped (the helper
  reports this too).
- Confirm the hardened Argo path is present before Gate A:
  Application `ServerSideApply=true`, Application
  `ClientSideApplyMigration=false`, and per-resource
  `argocd.argoproj.io/sync-options: ServerSideApply=true` on the oversized
  CRD. If any are missing, stop and classify the fixture as not hardened.
- State whether the intended delivery path is client-side apply, server-side
  apply, an Argo/ConfigHub sync option with ServerSideApply=true, or a
  separately approved dev setup live-write.
- If the apply mode cannot be determined from current command surfaces, stop
  before mutation and report exactly what is unknown.

Route card before any approval:
Use CONFIGHUB SAYS: ROUTE or ./scripts/confighub-banner route if available.
Include:
- Lane: CH-WRITE for governed ApplicationSet/controller delivery; RECOVERY or
  SETUP for any special CRD apply mode.
- Scope: exactly the mini-large-crds provider and the held-back widget-example
  custom resource.
- Wrong move: retrying a known annotation-size failure without changing apply
  mode; calling a direct CRD workaround governed workload proof.
- WATCH tripwires: admission denial, CRD annotation-size failure, missing CRD
  after apply, dependent resource attempted before CRD exists.

Setup complete stop before Gate A:
- Render a colored setup/preflight proof table with ./scripts/confighub-proof-rows
  --force-color --color-by both --title "CASE 02 PREFLIGHT READY" and rows for:
  fixture CRD count, largest CRD bytes, annotation-limit outcome, ApplicationSet
  ServerSideApply setting, controller fixture present, dependent CR held back.
- Immediately after those rows, render a dedicated GUI block
  VIEW IN CONFIGHUB — OPEN NOW using ./scripts/confighub-gui-urls --space
  mini-large-crds if available. Say what the user should see in the GUI:
  worker Ready, target bound, and empty unit panel before Gate A.
- Then render the Gate A route banner and ask for Y/N.

Gate A: Large-CRD delivery decision
Ask Y/N before any mutation. The approval must name the selected apply mode.
If approved:
- perform only the selected CRD/provider delivery path (server-side apply
  through the ApplicationSet unit is the default safe path);
- after the ApplicationSet creates the Application, remember that this fixture
  deliberately omits `syncPolicy.automated`; ask before triggering the
  one-shot Argo sync that actually applies the CRDs;
- prove both CRDs exist;
- prove no annotation-size rejection occurred;
- prove ConfigHub action/event status if ConfigHub was used;
- classify clearly whether any direct live CRD action was setup, not governed
  workload proof;
- re-surface GUI with ./scripts/confighub-gui-urls --space mini-large-crds
  --unit mini-large-crds-appset so the user can inspect the governed unit and
  revisions.

Gate B: Dependent resource/controller proof
Ask a separate Y/N if a live sync or apply is needed after CRDs exist.
If approved:
- create/sync only the dependent custom resource (fixtures/resources/
  widget-example.yaml) and ensure the controller fixture is Running/Ready;
- prove the API accepts the custom resource;
- prove controller pod Running/Ready;
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
