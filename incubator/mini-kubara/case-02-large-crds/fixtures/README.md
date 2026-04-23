# Case 02 Fixtures

Deterministic fixtures for Mini-Kubara Case 02: Large CRDs.

Nothing here pulls real upstream CRDs. The risk is staged on purpose so
preflight tooling can detect it before any live apply.

## Files

- `applicationsets/mini-large-crds.yaml` — one Argo ApplicationSet that
  generates exactly one Application (`controlplane-mini-large-crds`). The
  Application points at this public workload path and sets
  `spec.syncPolicy.syncOptions: [CreateNamespace=true, ServerSideApply=true]`
  so Argo reconciles the CRDs with server-side apply.
- `workloads/crds/widgets.yaml` — ordinary CRD. Small body, safe under
  client-side apply.
- `workloads/crds/stores-large.yaml` — intentionally large CRD. The
  serialized body is well over the Kubernetes 262144-byte annotation limit
  so client-side apply would be rejected with
  `metadata.annotations: Too long`. Regenerate with
  `./generate-stores-large.sh` if needed.
- `workloads/controller.yaml` — placeholder controller-style Deployment and
  its namespace. Uses `registry.k8s.io/pause:3.9` so the pod stays
  Running/Ready without running reconciler logic. Enough to prove
  "controller pod is Running/Ready" without pulling a third-party image.
- `resources/widget-example.yaml` — dependent custom resource used at
  Gate B to prove the CRD exists and is accepted by the API server.
- `generate-stores-large.sh` — regenerator for `stores-large.yaml`. Run it
  if `FIELD_COUNT` is tuned or the template changes.

## Delivery layout

Argo reconciles `workloads/` recursively:

```text
workloads/
  controller.yaml
  crds/
    widgets.yaml
    stores-large.yaml
```

The dependent `Widget/example` is NOT part of the Argo-reconciled set. It is
held back for Gate B so the order is enforced: CRDs first, custom resource
after.

## Public runtime source

The ApplicationSet fixture is the source Argo fetches from. A fresh Argo
install in kind reads:

```text
repoURL: https://github.com/confighub/examples
path: incubator/mini-kubara/case-02-large-crds/fixtures/workloads
```

`stores-large.yaml` is committed here because Argo cannot run the generator.

## Stop conditions to honour

- A preflight cannot determine the CRD apply mode.
- The selected path still stamps the oversized annotation.
- The dependent Widget is attempted before the CRDs land.
- A direct `kubectl apply` of the large CRD is described as governed
  workload proof instead of a setup/recovery action.
