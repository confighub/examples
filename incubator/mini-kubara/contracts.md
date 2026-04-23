# Mini-Kubara Contracts

## Scope

This contract covers the public Mini-Kubara training pack under `incubator/mini-kubara`.

## Case 01: Clean Handoff

mutates:
- ConfigHub space/unit state only after Gate A approval;
- live Argo Application operation state only after Gate B approval;
- namespace `mini-clean` workloads only through Argo sync.

proves:
- a bootstrap ApplicationSet can be delivered through ConfigHub;
- Argo generates exactly one Application;
- one-shot sync converges without private GitHub credentials;
- ConfigHub, Argo, Kubernetes, cub-scout, and GUI receipts are all surfaced.

## Case 02: Large CRDs

mutates:
- only after the run names the CRD apply-mode risk and receives approval.

proves:
- large-CRD delivery risk is detected before apply;
- recovery or server-side apply mode is explicit, not accidental.

## Case 03: Policy Admission

mutates:
- only the approved policy/app scope;
- no silent policy disablement.

proves:
- admission WATCH becomes BLOCK when fail-closed policy rejects the requested resource;
- recovery is explicit and governed.

## Case 04: Dev/Prod Variants

mutates:
- only approved dev/prod variant data and promotion paths.

proves:
- dev-only settings are visible;
- promotion does not leak forbidden dev values into prod.
