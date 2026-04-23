# Case 03: Policy Admission

## Goal

Turn the Kyverno lesson into a focused drill:

> The agent carries admission risk as WATCH, promotes it to BLOCK when
> fail-closed admission rejects an in-scope resource, and opens a recovery gate
> instead of retrying blindly.

This case is not about making policy disappear. It is about making admission
behavior legible and safe: the BLOCK is proven by construction before any
cluster mutation, and the recovery path prefers fixing governed desired state
over relaxing policy.

Fresh-Claude handoff prompt: [`PROMPT.md`](./PROMPT.md).

## Scenario

One kind cluster. One policy controller (Kyverno). One app that starts blocked
by policy.

The fixture ships with:

- a single Kyverno `ClusterPolicy` requiring `securityContext.runAsNonRoot=true`
  and `securityContext.allowPrivilegeEscalation=false` on every container in
  the `mini-policy` namespace, with `validationFailureAction: Enforce` and
  `failurePolicy: Fail`;
- one intentionally bad Deployment that violates the policy;
- one corrected Deployment that satisfies the policy;
- an Argo ApplicationSet that defaults to the bad workload so the admission
  BLOCK is deterministic on first sync.

Names are fake and harmless:

```text
ApplicationSet/mini-policy
  -> Application/controlplane-mini-policy
     -> ClusterPolicy/require-safe-container
     -> Deployment/mini-policy-api                 (bad by default)
     -> Deployment/mini-policy-api (fixed variant, held for Gate B)
```

## What This Trains

- Admission-controller preflight.
- WATCH tripwire wording and the WATCH -> BLOCK promotion rule.
- Stopping on fail-closed rejection instead of retrying.
- Choosing between fixing governed desired state and a dev-only policy
  relaxation.
- Showing policy as part of ConfigHub governance, not as random cluster noise.

## Route

```text
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for governed desired-state fix at Gate B;
      SETUP for installing Kyverno + policy at Gate A;
      LIVE-WRITE only if the user explicitly approves a dev-only admission
      relaxation
Scope: one app and one policy in the mini-policy namespace
Wrong move: retrying the same rejected manifest, disabling policy silently,
  or live-patching the workload while ConfigHub desired state still violates
  the policy
```

When the helper scripts are available, render the route with
`./scripts/confighub-banner route --force-color` and render preflight/gate proof
with `./scripts/confighub-proof-rows --force-color --color-by both`. Color is
part of the trust surface. If the terminal flattens ANSI color, say that
plainly instead of silently falling back to uncolored bullets.

## GUI Moment

The ConfigHub GUI must appear before Gate A approval, not only at closeout.
After the preflight runs and before asking for the first ConfigHub write, run:

```bash
./scripts/confighub-gui-urls --space mini-policy
```

Label the output:

```text
VIEW IN CONFIGHUB — OPEN NOW
```

Tell the user what the page should prove at that moment: worker Ready, target
bound, and unit panel empty. After Gate A lands, run the helper again with
`--unit mini-policy-appset` and point the user at the governed unit/revisions.

## Preflight Helper

Use the repo-local preflight helper before Gate A. It is read-only and does
not create clusters, install Kyverno, apply policies, or sync Argo.

```bash
examples/mini-kubaras/case-03-policy-admission/preflight.sh --explain
examples/mini-kubaras/case-03-policy-admission/preflight.sh
```

The helper:

- confirms the fixture layout (policy, bad deployment, fixed deployment,
  ApplicationSet) is present;
- proves the bad Deployment lacks the two required `securityContext` fields on
  every container, so Pod admission by the policy would be rejected;
- proves the fixed Deployment sets both fields on every container, so
  admission would succeed;
- checks the policy is scoped (`Enforce`, `Fail`, match restricted to the
  `mini-policy` namespace) and names the admission webhook surface to inspect
  at Gate A;
- checks the ApplicationSet defaults to the bad workload so the BLOCK is
  deterministic on first sync;
- states the WATCH -> BLOCK tripwire and the recovery preference before any
  mutation.

A fresh Claude session must run `--explain` and then the default form before
proposing Gate A. It must not create a kind cluster, install Kyverno, stand
up a worker/target, `kubectl apply` the policy, or sync the ApplicationSet
on the strength of this helper.

## Fixture Layout

Authoring fixture in this repo:

```text
examples/mini-kubaras/case-03-policy-admission/fixtures/
  README.md
  policies/require-safe-container.yaml
  workloads/deployment-bad.yaml
  workloads/deployment-fixed.yaml
  applicationsets/mini-policy.yaml
```

Runtime source for Argo:

```text
https://github.com/confighub/examples
path: incubator/mini-kubara/case-03-policy-admission/fixtures/workloads
```

The ApplicationSet uses `directory.include: deployment-bad.yaml` so Argo only
materialises one Deployment at a time. The recovery flow changes that
`include` to `deployment-fixed.yaml` through the governed ConfigHub path.

The Application generated by the ApplicationSet deliberately has no
`syncPolicy.automated` block. That keeps the approval boundary sharp, but it
also means a fresh run must trigger a one-shot Argo sync after Gate A applies
the ApplicationSet. Do not poll for auto-sync; prove the Application exists,
then ask for the one-shot sync approval.

Do not create a fourth variant or a different policy mid-run. The Case 03
fixture is deterministic on purpose so admission discipline is the only
variable.

Public graduation targets:

- `examples/confighub-authority-scan/` for scan/advisory framing;
- a future policy-heavy live cluster example once the local case is boring.

## Prompt To Run

```text
Run Mini-Kubara Case 03: Policy Admission.

Start with the read-only preflight.
  examples/mini-kubaras/case-03-policy-admission/preflight.sh --explain
  examples/mini-kubaras/case-03-policy-admission/preflight.sh

Do not create a cluster, install Kyverno, apply the policy, or sync the
ApplicationSet on the strength of the preflight alone.

Use cub-scout and kubectl to prove admission-controller state, webhook
failurePolicy, webhook endpoints, and the exact policy in scope (read-only).

Propose Gate A to install the policy and apply the intentionally bad app
path in scope. If admission rejects the pods, stop immediately and classify
WATCH -> BLOCK. Do not retry.

Propose Gate B for recovery. Prefer fixing governed desired state by
changing the ApplicationSet Unit's include from deployment-bad.yaml to
deployment-fixed.yaml through ConfigHub. Only offer dev-only policy
relaxation if the user explicitly wants that variant.

After recovery, prove the policy remains active, the app is admitted, pods
are Running/Ready, and no unrelated resources changed. Close with
cub-scout, ConfigHub receipts, Argo status, and exact admission proof.
```

## Expected Proof

- The route card names the admission WATCH and its BLOCK threshold.
- The first rejected apply is classified from the admission error, not from
  generic pod status.
- Recovery changes the governed desired state or the dev policy explicitly.
- The fixed app passes admission.
- The policy controller stays healthy.
- `cub-scout doctor --presentation paired` confirms convergence after the fix.

## Stop Conditions

- Fixture is missing or altered and the preflight cannot pass by construction.
- Policy controller is unhealthy before the case starts.
- Webhook is fail-closed and unreachable before the intended mutation.
- The rejection reason does not match the policy under test.
- The proposed recovery disables or weakens policy globally without explicit
  approval.
- The app is made healthy by direct live patch while ConfigHub desired state
  still violates policy.

## What This Does Not Prove

- Large-CRD delivery.
- ApplicationSet adoption.
- Promotion.

This case exists to train admission literacy and recovery boundaries.
