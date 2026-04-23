# Fresh Claude Prompt: Mini-Kubara Case 04

Use this as the complete prompt for a fresh Claude session. Run this case only
after the earlier mini-Kubara cases have stopped and been summarized. Do not
run mini-Kubaras in parallel.

```text
You are running Mini-Kubara Case 04: Dev/Prod Variants from the
confighub/examples repo at incubator/mini-kubara.

Goal:
Prove ConfigHub's dev/prod variant value: dev-only settings must be visible and
must not leak into prod during promotion. The case can run on one laptop, but
it must not claim real production readiness.

Read first:
- AGENTS.md
- incubator/mini-kubara/README.md
- incubator/mini-kubara/case-04-dev-prod-variants/GUIDE.md
- incubator/AI_START_HERE.md
- incubator/ai-guide-standard.md
- incubator/standard-ai-demo-pacing.md

Operating rules:
- You are fresh. Do not assume kind, Argo, ConfigHub auth, workers, targets,
  kubeconfig, fixtures, setup scripts, spaces, promotion objects, or
  environments are ready.
- The user will not manually touch kind clusters, GitOps controllers, workers,
  spaces, or promotion setup for you. You own read-only discovery, setup
  detection, and blocker reporting.
- Do not run auth login. If ConfigHub auth is expired and live ConfigHub work
  is needed, ask the user to run cub auth login and stop.
- Check current cub/cub-scout command surfaces with help before relying on a
  flag or verb.
- Use cub-scout with the exact kubeconfig/context used for kubectl.
- One-shot the run if possible, but do not break approval scope. Stop for Y/N
  before ConfigHub writes, live cluster writes, promotion setup, or promotion.
- Do not mutate other public examples. Only create or edit files under this
  case directory if fixture authoring is necessary; stop before live mutation
  and report the diff.
- If fixture manifests or setup/verify scripts are missing, create the missing
  repo-local files under this case directory first. Keep dev/prod simulated
  clearly if only one kind cluster is available.

Initial preflight, read-only:
1. Print repo path and git status.
2. Read the case guide and list expected fixture/setup files.
3. Check cub auth/API reachability with cub space list or another
   authenticated read. Stop for cub auth login if needed.
4. Check kubectl current context and cluster reachability.
5. Check cub-scout help and run cub-scout doctor with the exact kubeconfig or
   context if cluster access exists.
6. Inspect whether dev/prod spaces, units, targets, workers, promotion
   surfaces, or namespace variants already exist. Read-only only.

If local fixtures are missing:
- Create a tiny local fixture under this case directory with:
  - dev values;
  - prod values;
  - one app manifest or ApplicationSet for each variant;
  - a promotion-variant checklist that names allowed and forbidden
    differences.
- The fixture must include at least one dev-only value that must not promote
  to prod, such as fake ACME email, local secret backend, or dev-only admission
  relaxation.
- Stop before live mutation if the setup path is not bounded.

Variant preflight:
- Name the dev and prod variants.
- Print the fields allowed to differ.
- Print the fields forbidden from promotion.
- If dev and prod values are not distinguishable, stop and fix the fixture or
  report the blocker.

Route card before any approval:
Use CONFIGHUB SAYS: ROUTE or ./scripts/confighub-banner route if available.
Include:
- Lane: CH-WRITE for variant data and appset delivery; PROMOTION for dev ->
  prod.
- Scope: one app across dev and prod variants.
- Wrong move: copying dev rendered YAML to prod, promoting fake credentials,
  or claiming production readiness from one local kind cluster.
- WATCH tripwires: dev-only value in prod, promotion without diff proof,
  missing prod target, wrong cluster context, or failed action/event history.

Gate A: Dev variant delivery
Ask Y/N before any mutation. If approved:
- create/apply only the dev variant path;
- prove ConfigHub receipts, Argo/controller receipts, cluster receipts, and
  cub-scout convergence;
- label dev-only WATCH items clearly.

Gate B: Prod variant/promotion preflight
Before promotion, run read-only comparison:
- ConfigHub diff or equivalent current command surface;
- dev values vs prod values;
- allowed vs forbidden fields;
- promotion target proof.

If any forbidden dev value would reach prod, stop and classify BLOCK. Do not
promote.

Gate C: Promotion or prod apply
Ask Y/N before promotion or prod mutation. If approved:
- promote or apply only the approved fields;
- prove prod received the right variant values;
- prove dev-only data stayed out;
- prove Argo/controller/cluster receipts if prod is live;
- classify any simulated proof honestly.

Closeout:
Use the session report and proof closeout templates. Include:
- dev/prod variant map;
- allowed and forbidden differences;
- ConfigHub revision/diff/promotion receipt;
- controller/cluster receipts if live;
- what was simulated because this is one laptop;
- not proven;
- next truthful step toward real multi-cluster promotion.

Do not proceed to full Kubara, real cloud, retired-unit cleanup, or public
example graduation. End with a concise report and git status.
```
