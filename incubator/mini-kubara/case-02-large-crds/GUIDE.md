# Case 02: Large CRDs

## Goal

Productize the external-secrets lesson without dragging the full Kubara stack
behind it:

> ConfigHub and the agent detect large-CRD delivery risk before a workload
> install, choose an explicit apply mode, and prove the CRDs plus dependent
> controller are operational.

This case should make the CRD decision visible instead of discovering a 256 KiB
annotation failure halfway through a sync.

Fresh-Claude handoff prompt: [`PROMPT.md`](./PROMPT.md).

## Scenario

One kind cluster. One CRD provider. One dependent resource.

The fixture should include:

- a small controller-style app with two CRDs;
- one ordinary CRD that applies normally;
- one intentionally large CRD that would fail if a client-side
  `last-applied-configuration` annotation is stamped;
- one dependent custom resource that proves the CRD exists and is usable.

The names can be fake and harmless:

```text
ApplicationSet/mini-large-crds
  -> Application/controlplane-mini-large-crds
     -> CustomResourceDefinition/widgets.platform.example.com
     -> CustomResourceDefinition/stores.platform.example.com
     -> Deployment/mini-crd-controller
     -> Widget/example
```

## What This Trains

- CRD preflight before sync.
- Distinguishing "retry later" from "delivery mode is wrong."
- Routing large CRDs to server-side apply or a CRD-specific sync option.
- Proving a dependent custom resource only after the CRD exists.
- Avoiding hidden `kubectl` workarounds.

## Route

```text
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for ApplicationSet, then RECOVERY/SETUP lane only if CRD apply
mode requires a special path
Scope: CRD provider app only
Wrong move: retrying a known annotation-size failure without changing apply mode
```

## Proposed Fixture

Local fixture in this repo:

```text
incubator/mini-kubara/case-02-large-crds/fixtures/
  applicationsets/mini-large-crds.yaml
  crds/widgets.yaml
  crds/stores-large.yaml
  resources/widget-example.yaml
```

Public graduation targets:

- external-secrets-style CRDs;
- cert-manager-style CRDs;
- any CRD-heavy platform app from a public example repo.

Do not start with the real public CRDs. First make a local large-CRD fixture so
the failure mode is predictable.

## Prompt To Run

```text
Run Mini-Kubara Case 02: Large CRDs.

Start with read-only CRD preflight. Count CRDs, inspect annotation-size risk,
and state the apply mode before any mutation.

If the large CRD would exceed the Kubernetes annotation limit under
client-side apply, stop and propose the explicit delivery mode: server-side
apply, CRD-specific sync option, or labeled dev setup live-write. Do not retry
blindly.

After approval, apply the provider path, prove both CRDs exist, prove the
controller is Running/Ready, then create or sync the dependent custom resource.
Close with cub-scout, ConfigHub receipts, Argo status, and exact CRD proof.
```

## Expected Proof

- Preflight names the large CRD risk before sync.
- The selected apply mode is visible in the route card.
- Both CRDs exist after the mutation.
- No `metadata.annotations: Too long` error appears.
- Controller pod is Running/Ready.
- Dependent custom resource is accepted by the API.
- The final report says whether the large-CRD path was ConfigHub-governed or a
  separately approved dev setup write.

## Stop Conditions

- CRD preflight cannot determine apply mode.
- The selected path still stamps an oversized annotation.
- The dependent custom resource is attempted before the CRD exists.
- Admission denial appears and was not part of the case scope.
- The agent describes a direct live CRD workaround as governed workload proof.

## What This Does Not Prove

- Admission-controller reliability.
- Promotion.
- Values-file governance.

This case is only about making CRD delivery semantics first-class.
