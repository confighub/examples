# Case 01 Fixtures

Minimal fixtures for the Mini-Kubara Case 01: Clean Handoff.

## Files

- `applicationsets/mini-clean.yaml` — one Argo ApplicationSet named `mini-clean`
  in the `argocd` namespace. Uses a `list` generator with a single element so
  the fan-out is exactly one. Generates `Application/controlplane-mini-clean`.
  No `syncPolicy.automated` block — one-shot sync only.
- `workloads/mini-clean.yaml` — the tiny workload the generated Application
  points at: `Namespace/mini-clean`, `Deployment/mini-clean` (whoami),
  `Service/mini-clean`.

## Delivery wiring (not created yet)

Case 01 needs a live path to render/apply the ApplicationSet through
ConfigHub. The concrete wiring decision belongs to the operator:

- **Local kind cluster.** A dedicated small kind cluster keeps scope clean.
  If no setup helper exists in the current checkout, stop and report that
  setup is missing before asking for Gate A approval.
- **ArgoCD install.** The ApplicationSet controller must exist in `argocd`.
- **ConfigHub target/worker.** A worker with a Kubernetes provider must be
  `Ready` and bound to a target that reaches the case cluster.
- **ApplicationSet source.** The fixture points at this public repo:
  `https://github.com/confighub/examples` path
  `incubator/mini-kubara/case-01-clean-handoff/fixtures/workloads` on `main`.
  That is intentional. A fresh kind Argo install should not need private
  GitHub credentials to fetch the workload source.

## Stop conditions to honour

- ApplicationSet fan-out > 1 — the generator must stay a single-element list.
- `syncPolicy.automated` is added back — auto-sync must stay off.
- Any resource outside the `mini-clean` namespace is touched.
- `cub-scout` is run against the wrong cluster context.
