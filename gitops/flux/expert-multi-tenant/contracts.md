# Contracts

What a tool may assume about this example.

## Read-Only Contracts

### `./setup.sh --explain`

- mutates: no
- output shape: plain text
- stable text anchors: `read-only setup plan`, `ConfigHub mutations if you run without --explain`
- proves: the plan before any ConfigHub mutation

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`, `spaces`, `units`, `cluster`, `teams`, `namespaces`, `apps`, `evaluation_modes`
- expected anchors:
  - `.example_name == "gitops-flux-multi-tenant"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 4`
  - `.cluster == "shared"`
  - `.teams == ["team-storefront", "team-payments", "team-loyalty"]`
  - `.units | length == 7`
- proves: the example plan before any mutation

## Structural Contract

A tool may assume all of the following about the files in this directory,
without running anything against a cluster:

- `clusters/shared` holds exactly one Flux `Kustomization`, named `tenants`,
  with no `serviceAccountName` set. It is the only object in this example
  that reconciles with the cluster's own trusted identity rather than a
  team's.
- Each of `tenants/base/team-storefront`, `tenants/base/team-payments` and
  `tenants/base/team-loyalty` holds `rbac.yaml` (a `Namespace`, a
  `ServiceAccount` and a `RoleBinding` to the built-in `admin` ClusterRole,
  scoped by the RoleBinding's own namespace), `guardrails.yaml` (a
  `ResourceQuota` and a `NetworkPolicy` with no `namespaceSelector`, so it
  admits ingress only from pods in the same namespace) and `sync.yaml` (a
  `GitRepository` and a `Kustomization`).
- Every team's `sync.yaml` Kustomization sets `serviceAccountName` to that
  team's own ServiceAccount name and `targetNamespace` to that team's own
  namespace. Those two values always match the namespace the team's
  `rbac.yaml` RoleBinding grants rights in.
- Each team's `workloads/` folder is excluded from that team's own
  `kustomization.yaml` (which lists only `rbac.yaml`, `guardrails.yaml` and
  `sync.yaml`). It is reconciled separately, by the `Kustomization` object
  `sync.yaml` defines, the same two-step shape `gitops/flux/expert-fleet`
  uses for its one tenant.
- `team-storefront` and `team-payments` each render a `Deployment` and a
  `Service`. `team-loyalty` renders a `Deployment` only. The three teams do
  not have to look identical.
- Every `Kustomization` path in this example points at a directory in this
  repo that contains a `kustomization.yaml`.

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 4 Spaces (`gitops-flux-multi-tenant-platform`,
  `gitops-flux-multi-tenant-team-storefront`,
  `gitops-flux-multi-tenant-team-payments`,
  `gitops-flux-multi-tenant-team-loyalty`), one cluster-control Unit plus
  one `tenant-bootstrap` and one `tenant-workloads` Unit per team
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands
  it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-flux-multi-tenant checks passed.`
- proves:
  - `kustomize build` succeeds for the cluster layer and every team's
    bootstrap and workloads
  - the platform bootstrap Kustomization sets no `serviceAccountName`
  - every team has its own `Namespace`, `ServiceAccount` and `RoleBinding`
    to the built-in `admin` ClusterRole
  - every team has a `ResourceQuota` and a same-namespace-only
    `NetworkPolicy`
  - every team's own Kustomization impersonates its own ServiceAccount and
    targets its own namespace (the structural stand-in for the tenant-escape
    break this example ships)
  - `team-storefront` and `team-payments` render a Deployment and a
    Service; `team-loyalty` renders a Deployment only
  - every path any Kustomization points at exists in this repo
- does not require ConfigHub, Flux, or a live cluster
- does not prove that a live cluster would return `Forbidden` for the
  tenant-escape edit; that is named explicitly as a proof gap in the README

## Live Rendering Reference

### `kustomize build clusters/shared`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: one Flux Kustomization, `tenants`, with no impersonation, pointed
  at `tenants/base`

### `kustomize build tenants/base/team-payments`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: `team-payments`'s Namespace, ServiceAccount, RoleBinding,
  ResourceQuota, NetworkPolicy, GitRepository and Kustomization, the last of
  which impersonates `team-payments` and targets the `team-payments`
  namespace
