# Fresh Claude Prompt: Mini-Kubara Case 03

Use this as the complete prompt for a fresh Claude session. Run this case only
after earlier mini-Kubara work has stopped and been summarized. Do not run
mini-Kubaras in parallel.

```text
You are running Mini-Kubara Case 03: Policy Admission from the
confighub-ai-demo repo.

Goal:
Prove admission discipline: carry policy/admission risk as WATCH, promote it
to BLOCK when fail-closed admission rejects an in-scope resource, and recover
by fixing governed desired state or explicitly approving a dev-only policy
relaxation. Do not retry the same rejected manifest blindly.

Read first:
- AGENTS.md
- examples/mini-kubaras/README.md
- examples/mini-kubaras/case-03-policy-admission/GUIDE.md
- examples/mini-kubaras/case-03-policy-admission/fixtures/README.md
- docs/product/LOVABLE_UX_UVB_CONTRACT.md
- docs/product/TRUST_SURFACE_DENSITY_CONTRACT.md
- docs/templates/PROOF_CLOSEOUT_TEMPLATE.md

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
- Do not mutate public examples. Use them only as read-only references.
- The repo already ships deterministic Case 03 fixtures under
  examples/mini-kubaras/case-03-policy-admission/fixtures. Do not regenerate
  or overwrite them mid-run. Do not invent a fourth workload variant or a
  different policy. If a fixture appears missing, stop and report the blocker.

Initial preflight, read-only:
1. Print repo path and git status.
2. Read the case guide and the fixtures README and list the expected fixture
   and helper files.
3. Run the repo-local preflight helper FIRST, in --explain mode, then in the
   default read-only form:
     examples/mini-kubaras/case-03-policy-admission/preflight.sh --explain
     examples/mini-kubaras/case-03-policy-admission/preflight.sh
   The helper is read-only: it confirms the fixture layout and proves the bad
   workload violates the policy by construction and the fixed workload
   satisfies it by construction. Do not treat a successful preflight as
   Gate A approval.
4. Check cub auth/API reachability with cub space list or another
   authenticated read. Stop for cub auth login if needed.
5. Check kubectl current context and cluster reachability. If no reachable
   cluster exists, inspect local setup options and stop with a blocker or
   setup proposal; do not create a kind cluster silently.
6. Check cub-scout help and run cub-scout doctor with the exact kubeconfig or
   context if cluster access exists. Only use flags the current help output
   confirms.
7. Read admission controllers, ValidatingWebhookConfigurations, their
   failurePolicy and Service endpoints, Kyverno pods if any, and recent
   admission failure events. This is read-only.

Admission preflight (structural, from fixtures):
- Confirm the helper reports deployment-bad.yaml as BLOCK by construction
  (missing runAsNonRoot and/or allowPrivilegeEscalation on every container).
- Confirm the helper reports deployment-fixed.yaml as OK by construction.
- Confirm the policy has validationFailureAction=Enforce, failurePolicy=Fail,
  and match scope restricted to the mini-policy namespace.
- Confirm the ApplicationSet defaults to deployment-bad.yaml so the BLOCK
  is deterministic on first sync.
- If any of those are not reported by the helper, stop and report the
  specific blocker. Do not proceed.

Route card before any approval:
Use CONFIGHUB SAYS: ROUTE or ./scripts/confighub-banner route if available.
Include:
- Lane: CH-WRITE for governed desired-state change at Gate B; SETUP for the
  policy install at Gate A; LIVE-WRITE only for an explicitly approved
  dev-only policy relaxation.
- Scope: exactly one app and one policy in the mini-policy namespace.
- Wrong move: retrying the rejected manifest, disabling policy silently, or
  patching the live workload while ConfigHub desired state still violates
  policy.
- WATCH tripwire: admission unhealthy but not rejecting is WATCH;
  fail-closed rejection of in-scope resources promotes to BLOCK; webhook
  Service unreachable while failurePolicy=Fail is BLOCK before any intended
  mutation.

Setup complete stop before Gate A:
- Render a colored setup/preflight proof table with ./scripts/confighub-proof-rows
  --force-color --color-by both --title "CASE 03 PREFLIGHT READY" and rows
  for: policy shape (Enforce, Fail, namespaced), bad fixture BLOCK by
  construction, fixed fixture OK by construction, ApplicationSet default
  include, admission surface identified.
- Immediately after those rows, render a dedicated GUI block
  VIEW IN CONFIGHUB — OPEN NOW using ./scripts/confighub-gui-urls --space
  mini-policy if available. Say what the user should see in the GUI:
  worker Ready, target bound, and empty unit panel before Gate A.
- Then render the Gate A route banner and ask for Y/N.

Gate A: Policy and bad-app setup
Ask Y/N before any mutation. If approved:
- install or apply only the policy and the intentionally bad app path in the
  approved scope;
- after the ApplicationSet creates the Application, remember that this
  fixture deliberately omits syncPolicy.automated; ask before triggering the
  one-shot Argo sync that actually applies the workload;
- if admission rejects the pods, stop immediately and classify WATCH -> BLOCK;
- capture the exact admission error, policy name, webhook name,
  failurePolicy, and affected resource;
- do not retry the same manifest.

Gate B: Recovery
Propose the recovery gate. Prefer fixing governed desired state by changing
the ApplicationSet Unit's include from deployment-bad.yaml to
deployment-fixed.yaml through ConfigHub. Only offer dev-only policy
relaxation if the user explicitly wants to test that variant.

Ask Y/N before recovery mutation. If approved:
- apply only the fixed desired state or the explicitly approved policy
  relaxation;
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
