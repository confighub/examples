# Case 01: Clean Handoff

## Goal

Build the boring happy path that Kubara never got to show cleanly:

> ConfigHub applies one bootstrap ApplicationSet, Argo owns the generated
> Application, a one-shot sync converges, and the closeout has no mixed state.

This is the confidence-builder. It should be intentionally small, fast, and
boring.

Fresh-Claude handoff prompt: [`PROMPT.md`](./PROMPT.md).

## Scenario

One kind cluster. One namespace. One ApplicationSet. One generated Argo
Application. One tiny workload such as `echo-server`, `whoami`, or `nginx`.

The ApplicationSet should generate exactly one Application:

```text
ApplicationSet/mini-clean
  -> Application/controlplane-mini-clean
     -> Namespace/mini-clean
     -> Deployment/mini-clean
     -> Service/mini-clean
```

No CRDs. No admission trickery. No external dependencies.

## What This Trains

- Recognizing the controller object vs workload object split.
- Applying an ApplicationSet through ConfigHub without overclaiming workload
  delivery.
- Proving Argo adoption/generation.
- Performing one-shot sync while `syncPolicy.automated.enabled=false`.
- Closing with ConfigHub, Argo, Kubernetes, and `cub-scout` all agreeing.

## Route

```text
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE, then LIVE-WRITE for one-shot Argo sync
Scope: one bootstrap ApplicationSet and one generated Application
Wrong move: broad selectors, direct kubectl apply of workload manifests, cleanup
before closeout
```

ConfigHub should mutate only the bootstrap ApplicationSet unit. Argo should
create or manage the generated Application. The workload should install only
after a separate one-shot sync approval.

When the helper scripts are available, render the route with
`./scripts/confighub-banner route --force-color` and render setup/gate proof
with `./scripts/confighub-proof-rows --force-color --color-by both`. Color is
part of the trust surface. If the terminal flattens ANSI color, say that
plainly instead of silently falling back to uncolored bullets.

## GUI Moment

The ConfigHub GUI must appear before Gate A approval, not only at closeout.
After setup is complete and before asking for the first ConfigHub write, run:

```bash
./scripts/confighub-gui-urls --space mini-clean
```

Label the output:

```text
VIEW IN CONFIGHUB — OPEN NOW
```

Tell the user what the page should prove at that moment: worker Ready, target
bound, and unit panel empty. After Gate A lands, run the helper again with
`--unit mini-clean-appset` and point the user at the governed unit/revisions.

## Setup Helper

Use the repo-local setup helper before Gate A when the cluster, Argo CD,
worker, or target are missing:

```bash
incubator/mini-kubara/case-01-clean-handoff/setup.sh --explain
incubator/mini-kubara/case-01-clean-handoff/setup.sh
```

The helper stops before creating the `mini-clean-appset` unit. It intentionally
uses server-side apply for Argo CD because the ApplicationSet CRD can exceed the
client-side last-applied annotation limit, and it installs the ConfigHub worker
by exporting the worker manifest and applying it to the kind cluster.

## Proposed Fixture

Public fixture in this repo:

```text
incubator/mini-kubara/case-01-clean-handoff/fixtures/
  applicationsets/mini-clean.yaml
  workloads/mini-clean.yaml
```

Runtime source for Argo:

```text
https://github.com/confighub/examples
path: incubator/mini-kubara/case-01-clean-handoff/fixtures/workloads
```

The public source is required, not cosmetic. A fresh Argo install in kind must
be able to fetch this workload without private GitHub credentials.

## Prompt To Run

```text
Run Mini-Kubara Case 01: Clean Handoff.

Start read-only. Use cub-scout with the exact kubeconfig/context, then propose
one CH-WRITE gate to create/apply exactly ApplicationSet/mini-clean through
ConfigHub. Stop before one-shot Argo sync.

After approval, apply only that ApplicationSet. Prove Application generation or
adoption, ownerReferences, syncPolicy.automated.enabled=false, and no unrelated
changes.

Then ask for a separate LIVE-WRITE approval to one-shot sync the generated
Application. After sync, prove ConfigHub appset unit receipt, Argo
Synced/Healthy/Succeeded, pods Running/Ready, cub-scout convergence, and GUI or
mockup receipts.
```

## Expected Proof

- ConfigHub bootstrap unit reaches `HeadRev=LiveRev=LastApplied`.
- `ApplicationSet/mini-clean` exists.
- `Application/controlplane-mini-clean` exists and is owned by the
  ApplicationSet.
- Persistent sync policy stays `enabled=false`.
- One-shot sync succeeds.
- Pod is Running/Ready.
- `cub-scout doctor --presentation paired` with the exact kubeconfig shows no
  drift or blocking warnings for the namespace.

## Stop Conditions

- The ApplicationSet would generate more than one Application.
- Argo creates a different Application name than expected.
- The generated Application is auto-syncing unexpectedly.
- Any resource outside `mini-clean` changes.
- `cub-scout` is accidentally run against the wrong cluster context.

## What This Does Not Prove

- Large-CRD delivery.
- Admission-controller recovery.
- Dev/prod variant management.
- Promotion.

That is deliberate. This case exists to rebuild the clean baseline.
