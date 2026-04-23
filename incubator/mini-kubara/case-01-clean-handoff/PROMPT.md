# Fresh Claude Prompt: Mini-Kubara Case 01

Use this as the complete prompt for a fresh Claude session. Run this case by
itself, after any previous mini-Kubara has stopped and been summarized. Do not
run mini-Kubaras in parallel.

```text
You are running Mini-Kubara Case 01: Clean Handoff from the
confighub/examples repo at incubator/mini-kubara.

Goal:
Prove the boring happy path: ConfigHub applies one bootstrap ApplicationSet,
Argo owns one generated Application, a one-shot sync converges, and closeout is
clean. This is the green baseline for later harder cases.

Read first:
- AGENTS.md
- incubator/mini-kubara/README.md
- incubator/mini-kubara/case-01-clean-handoff/GUIDE.md
- incubator/AI_START_HERE.md
- incubator/ai-guide-standard.md
- incubator/standard-ai-demo-pacing.md

Operating rules:
- You are fresh. Do not assume kind, Argo, ConfigHub auth, workers, targets,
  kubeconfig, fixtures, or setup scripts are ready.
- The user will not manually touch kind clusters, GitOps controllers, or
  workers for you. You own read-only discovery, setup detection, and blocker
  reporting.
- Do not run auth login. If ConfigHub auth is expired and live ConfigHub work
  is needed, ask the user to run cub auth login and stop.
- Check current cub/cub-scout command surfaces with help before relying on a
  flag or verb.
- Use cub-scout with the exact kubeconfig/context used for kubectl. A
  default-context Scout read is not proof.
- One-shot the run if possible, but do not break approval scope. Stop for Y/N
  before ConfigHub writes or live cluster writes.
- Do not mutate other public examples. Only create or edit files under this
  case directory if fixture authoring is necessary; stop before live mutation
  and report the diff.
- If fixture manifests or setup/verify scripts are missing, create the missing
  repo-local files under this case directory first, then continue only if the
  local setup path is clear and bounded. Do not invent live ConfigHub or
  cluster state.
- Treat GUI as a first-class trust surface. The first meaningful ConfigHub GUI
  moment must appear before Gate A approval, not only in closeout.
- Use the repo visual helpers when present:
  - `./scripts/confighub-banner ... --force-color` for `CONFIGHUB SAYS`
    route/block/proof moments.
  - `./scripts/confighub-proof-rows --force-color --color-by both` for setup,
    gate, and closeout proof rows.
  - `./scripts/confighub-gui-urls --space <space> [--unit <unit>]` for GUI
    URLs derived from live ConfigHub IDs.
  If the terminal renderer flattens ANSI color, say "ANSI color was emitted but
  this renderer flattened it"; do not silently downgrade to plain bullets.

Initial preflight, read-only:
1. Print the current repo path and git status.
2. Read the case guide and list expected local fixture/setup files. Confirm
   the ApplicationSet workload source points at the public
   `https://github.com/confighub/examples` repo and
   `incubator/mini-kubara/case-01-clean-handoff/fixtures/workloads` path.
3. Check cub auth/API reachability with an authenticated read such as cub space
   list. If auth fails, stop and ask the user to run cub auth login.
4. Check kubectl current context and cluster reachability. If a repo-local
   state file or setup script exists, inspect it first rather than guessing.
5. Check cub-scout help and run cub-scout doctor with the exact kubeconfig or
   context if cluster access exists.
6. Check ConfigHub worker/target state read-only if auth exists.

If local fixtures are missing:
- Create a tiny local fixture for one ApplicationSet named mini-clean that
  generates one Application named controlplane-mini-clean and one simple
  workload in namespace mini-clean.
- Create or update a repo-local setup/verify note or script only if the pattern
  is obvious from nearby examples. Keep all new files under this case
  directory.
- Stop before live mutation and report the fixture diff if you cannot prove the
  setup path.

Route card before any approval:
Use `./scripts/confighub-banner route --force-color` if available; otherwise
render a visible `CONFIGHUB SAYS: ROUTE` block. Include:
- Lane: CH-WRITE for ApplicationSet delivery, then LIVE-WRITE for one-shot Argo
  sync.
- Scope: exactly ApplicationSet/mini-clean and generated
  Application/controlplane-mini-clean.
- Target/controller: ConfigHub target/worker if available; Argo ApplicationSet
  controller for generated app.
- Wrong move: broad selectors, direct kubectl apply of workload manifests,
  cleanup, or any other mini-Kubara case.
- WATCH tripwires: missing worker/target, wrong cluster context, auto-sync
  unexpectedly enabled, ApplicationSet fan-out > 1.

Setup complete stop before Gate A:
- Render a colored setup proof table with `./scripts/confighub-proof-rows
  --force-color --color-by both --title "CASE 01 SETUP READY"` if available; otherwise render a compact plain proof table. Include rows for:
  kind cluster, Argo availability, worker Ready, target bound, ConfigHub space,
  and 0 units before Gate A.
- Immediately after those rows, render a dedicated GUI block:
  `VIEW IN CONFIGHUB — OPEN NOW`.
  Use `./scripts/confighub-gui-urls --space mini-clean` if available; otherwise use the real ConfigHub space URL from `cub ... --web` or label the GUI URL as unavailable. Say what
  the user should see in the GUI: worker Ready, target bound, and empty unit
  panel before Gate A.
- Then render the Gate A route banner and ask for Y/N. Do not bury the GUI URL
  inside the route prose.

Gate A: ConfigHub ApplicationSet setup/apply
Ask one Y/N approval before any ConfigHub write. If approved:
- create/update/apply only the mini-clean bootstrap ApplicationSet unit through
  ConfigHub;
- prove the unit target/provider, revision receipt, and action/event status;
- prove ApplicationSet/mini-clean exists in Argo namespace;
- prove Application/controlplane-mini-clean exists and is owned by the
  ApplicationSet;
- prove syncPolicy.automated.enabled=false is preserved;
- re-surface GUI with `./scripts/confighub-gui-urls --space mini-clean --unit
  mini-clean-appset` so the user can inspect the governed unit and revisions;
- stop before one-shot sync.

Gate B: One-shot Argo sync
Ask a separate Y/N approval before patching/syncing the live Argo Application.
If approved:
- trigger exactly one one-shot sync for Application/controlplane-mini-clean;
- poll with a timeout and visible status updates;
- stop on Failed/Error/Degraded, wrong app, wrong namespace, or out-of-scope
  mutation;
- if converged, prove Argo Synced/Healthy/Succeeded, pod Running/Ready, and
  cub-scout convergence with the exact kubeconfig/context.

Closeout:
Use the proof closeout template. Include:
- ConfigHub receipt;
- Argo receipt;
- cluster receipt;
- cub-scout receipt;
- GUI URL or honest GUI mockup;
- verified directly;
- inferred;
- not proven;
- next truthful step.

Do not proceed to Case 02, Case 03, Case 04, cleanup, promotion, or public
example graduation. End with a concise report and git status.
```
