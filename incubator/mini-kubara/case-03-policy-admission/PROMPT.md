# Fresh Claude Prompt: Mini-Kubara Case 03

Use this as the complete prompt for a fresh Claude session. Run this case only
after earlier mini-Kubara work has stopped and been summarized. Do not run
mini-Kubaras in parallel.

```text
You are running Mini-Kubara Case 03: Policy Admission from the
confighub/examples repo at incubator/mini-kubara.

Goal:
Prove admission discipline: carry policy/admission risk as WATCH, promote it
to BLOCK when fail-closed admission rejects an in-scope resource, and recover
by fixing governed desired state or explicitly approving a dev-only policy
relaxation. Do not retry the same rejected manifest blindly.

Read first:
- AGENTS.md
- incubator/mini-kubara/README.md
- incubator/mini-kubara/case-03-policy-admission/GUIDE.md
- incubator/AI_START_HERE.md
- incubator/ai-guide-standard.md
- incubator/standard-ai-demo-pacing.md

Operating rules:
- You are fresh. Do not assume kind, Argo, ConfigHub auth, workers, targets,
  kubeconfig, fixtures, setup scripts, Kyverno, policies, or webhooks are
  ready.
- The user will not manually touch kind clusters, GitOps controllers, workers,
  Kyverno, or policies for you. You own read-only discovery, setup detection,
  and blocker reporting.
- Do not run auth login. If ConfigHub auth is expired and live ConfigHub work
  is needed, ask the user to run cub auth login and stop.
- Check current cub/cub-scout command surfaces with help before relying on a
  flag or verb.
- Use cub-scout with the exact kubeconfig/context used for kubectl.
- One-shot the run if possible, but do not break approval scope. Stop for Y/N
  before ConfigHub writes, live cluster writes, policy installs, policy
  relaxations, or app syncs.
- Do not mutate other public examples. Only create or edit files under this
  case directory if fixture authoring is necessary; stop before live mutation
  and report the diff.
- If fixture manifests or setup/verify scripts are missing, create the missing
  repo-local files under this case directory first. Keep them small and
  deterministic.

Initial preflight, read-only:
1. Print repo path and git status.
2. Read the case guide and list expected fixture/setup files.
3. Check cub auth/API reachability with cub space list or another
   authenticated read. Stop for cub auth login if needed.
4. Check kubectl current context and cluster reachability.
5. Check cub-scout help and run cub-scout doctor with the exact kubeconfig or
   context if cluster access exists.
6. Inspect admission controllers, validating/mutating webhook configurations,
   webhook failurePolicy, Service endpoints, pods, and recent admission
   failures. This is read-only.

If local fixtures are missing:
- Create a tiny local fixture under this case directory with:
  - one policy requiring safe container fields;
  - one intentionally bad Deployment;
  - one fixed Deployment;
  - one ApplicationSet or ConfigHub-unit input that can deliver the app.
- Do not install or relax policy until a gate is approved.
- Stop before live mutation if the setup path is not bounded.

Route card before any approval:
Use CONFIGHUB SAYS: ROUTE or ./scripts/confighub-banner route if available.
Include:
- Lane: CH-WRITE for governed desired-state changes; LIVE-WRITE only for an
  explicitly approved dev-only policy relaxation.
- Scope: exactly one app and one policy.
- Wrong move: retrying the rejected manifest, disabling policy silently, or
  patching live workload while ConfigHub desired state still violates policy.
- WATCH tripwire: admission unhealthy but not rejecting is WATCH; fail-closed
  rejection of in-scope resources is BLOCK.

Gate A: Policy and bad-app setup
Ask Y/N before any mutation. If approved:
- install or apply only the policy and intentionally bad app path in the
  approved scope;
- if admission rejects the app, stop immediately and classify WATCH -> BLOCK;
- capture the exact admission error, policy name, webhook name, failurePolicy,
  and affected resource;
- do not retry the same manifest.

Gate B: Recovery
Propose the recovery gate. Prefer fixing desired state through ConfigHub.
Only offer dev-only policy relaxation if the user explicitly wants to test that
variant.

Ask Y/N before recovery mutation. If approved:
- apply only the fixed desired state or explicitly approved policy relaxation;
- prove the policy remains active if fixing desired state;
- prove the app is admitted and pods are Running/Ready;
- prove ConfigHub/Argo/cluster receipts and cub-scout convergence.

Closeout:
Use the proof closeout template. Include:
- original WATCH and tripwire;
- admission error and classification;
- recovery path chosen;
- ConfigHub receipt;
- policy/admission receipt;
- cluster receipt;
- not proven;
- next truthful step.

Do not proceed to Case 04, large CRDs, promotion, or unrelated Kyverno harden.
End with a concise report and git status.
```
