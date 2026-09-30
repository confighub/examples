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
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`, `spaces`, `uploads`, `units_per_upload`, `cluster`, `teams`, `namespaces`, `apps`, `space_per_component`, `evaluation_modes`
- expected anchors:
  - `.example_name == "gitops-flux-multi-tenant"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 7`
  - `.spaces | unique | length == 7`
  - `.cluster == "shared"`
  - `.teams == ["team-storefront", "team-payments", "team-loyalty"]`
  - `.uploads | length == 7`
  - `.uploads | map(.space) == .spaces`
  - `.units_per_upload` says each upload makes one Unit per rendered resource, not one Unit per upload
- proves: the example plan before any mutation

## Structural Contract

A tool may assume all of the following about the files in this directory,
without running anything against a cluster:

- `clusters/shared` holds exactly one Flux `Kustomization`, named `tenants`,
  with `serviceAccountName: kustomize-controller`, Flux's own cluster-admin
  account in `flux-system`. It is the only object in this example that
  reconciles with the platform's identity rather than a team's.
- Each of `tenants/base/team-storefront`, `tenants/base/team-payments` and
  `tenants/base/team-loyalty` holds `rbac.yaml` (a `Namespace`, a
  `ServiceAccount`, a namespaced `Role` named `team-<name>-tenant`, and a
  `RoleBinding` to that Role in the same namespace), `guardrails.yaml` (a
  `ResourceQuota` and a `NetworkPolicy` with no `namespaceSelector`, so it
  admits ingress only from pods in the same namespace) and `sync.yaml` (a
  `GitRepository` and a `Kustomization`).
- No tenant Role grants anything beyond `get`, `list` and `watch` on
  NetworkPolicies, ResourceQuotas, LimitRanges, Namespaces, anything in the
  `rbac.authorization.k8s.io` group, or Flux's own objects, and no
  RoleBinding binds a team to the built-in `admin` or `edit` ClusterRole.
  NetworkPolicy allow rules are additive, so a NetworkPolicy write would
  let a team loosen the platform's same-namespace policy.
- The platform bootstrap Kustomization's `sourceRef` names the
  `flux-system` GitRepository that `flux bootstrap` generates. That source
  is not committed here.
- Every RoleBinding in a team's bootstrap sits in that team's namespace and
  has that team's own ServiceAccount, in that team's namespace, as its only
  subject. No other team's ServiceAccount, user or group is bound.
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
- A ConfigHub Space belongs to exactly one Component. This example never
  uploads two Components into the same Space: the platform's cluster
  control render, and each team's bootstrap and workloads, each go to their
  own Space. That is also what lets the platform's Spaces and each team's
  Space carry different, correct `Owner` and `Environment` labels.

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 7 Spaces, one per Component-and-variant, never shared:
  - `gitops-flux-multi-tenant-platform` (Component `cluster-control`, Owner `platform`)
  - `gitops-flux-multi-tenant-team-storefront-bootstrap` (Component `tenant-bootstrap`, Owner `platform`)
  - `gitops-flux-multi-tenant-team-storefront-workloads` (Component `tenant-workloads`, Owner `team-storefront`)
  - `gitops-flux-multi-tenant-team-payments-bootstrap` (Component `tenant-bootstrap`, Owner `platform`)
  - `gitops-flux-multi-tenant-team-payments-workloads` (Component `tenant-workloads`, Owner `team-payments`)
  - `gitops-flux-multi-tenant-team-loyalty-bootstrap` (Component `tenant-bootstrap`, Owner `platform`)
  - `gitops-flux-multi-tenant-team-loyalty-workloads` (Component `tenant-workloads`, Owner `team-loyalty`)

  `cub variant upload` makes one Unit per rendered resource, so there are
  seven uploads but many more Units: one for the cluster-level
  Kustomization, one per resource in each team's bootstrap (Namespace,
  ServiceAccount, Role, RoleBinding, ResourceQuota, NetworkPolicy,
  GitRepository, Kustomization) and one per resource in each team's workloads
  (a Deployment, plus a Service for storefront and payments). `./verify.sh`
  prints the resource count for each render.
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands
  it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-flux-multi-tenant checks passed.`
- proves:
  - the plan names 7 distinct Spaces, never fewer, and never a repeated name
    (the check that catches a bootstrap and a workloads upload aimed at the
    same Space)
  - `kustomize build` succeeds for the cluster layer and every team's
    bootstrap and workloads
  - the platform bootstrap Kustomization runs as `kustomize-controller`
  - the platform bootstrap Kustomization reads the `flux-system` source
  - every team has its own `Namespace`, `ServiceAccount`, `Role` and
    `RoleBinding`, and the RoleBinding points at that team's own Role
  - every RoleBinding in a team's bootstrap is in that team's namespace and
    has exactly one subject, that team's own ServiceAccount in that
    team's own namespace (a foreign subject fails and the error names it), and
    no ClusterRoleBinding is present
  - no tenant Role can write NetworkPolicies, ResourceQuotas, LimitRanges,
    Namespaces, RBAC or Flux objects, and each still grants Deployments
    and Services
  - every team has a `ResourceQuota` and a same-namespace-only
    `NetworkPolicy`
  - every team's own Kustomization impersonates its own ServiceAccount and
    targets its own namespace (the structural stand-in for the tenant-escape
    break this example ships)
  - `team-storefront` and `team-payments` render a Deployment and a
    Service; `team-loyalty` renders a Deployment only
  - every path any Kustomization points at exists in this repo
- does not require ConfigHub, Flux, or a live cluster
- does not read Flux controller flags (`--no-cross-namespace-refs`,
  `--default-service-account`) and does not check who may push to the repo;
  the README names both as things the isolation depends on
- does not prove that a live cluster would return `Forbidden` for the
  tenant-escape edit; that is named explicitly as a proof gap in the README

## Live Rendering Reference

### `kustomize build clusters/shared`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: one Flux Kustomization, `tenants`, running as
  `kustomize-controller` and no team, pointed at `tenants/base`

### `kustomize build tenants/base/team-payments`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: `team-payments`'s Namespace, ServiceAccount, Role, RoleBinding,
  ResourceQuota, NetworkPolicy, GitRepository and Kustomization, the last of
  which impersonates `team-payments` and targets the `team-payments`
  namespace
