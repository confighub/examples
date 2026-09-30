# Flux expert: several teams on one cluster

This example is a single shared cluster with several teams on it, the shape
a platform team runs when it is not willing to give every team its own
cluster. One platform-owned bootstrap layer creates a namespace, a
ServiceAccount, a RoleBinding, a ResourceQuota and a NetworkPolicy for each
team. Each team then reconciles its own app through its own GitRepository
and Kustomization, impersonating its own ServiceAccount and nothing else. It
is part of the `gitops/` canonical example set. See
[`../../README.md`](../../README.md) for the full index.

## Who this is for

You run one Flux-managed cluster that several teams commit to, and you do
not want to give every team its own cluster. You know Kustomizations,
sources, `serviceAccountName` impersonation, RBAC and NetworkPolicy. You are
not looking for a bootstrap tutorial. You want to see whether a tool can
read the tenant boundaries you already have and tell you the truth about
where they hold and where they do not.

## The first question this person asks

"If one team's Kustomization tries to write into another team's namespace,
does it actually get refused, and can a tool show me that before it
happens?"

## What this example shows

- **A platform-owned bootstrap layer**: `clusters/shared/tenants.yaml` sets
  no `serviceAccountName`, so it reconciles with the cluster's own trusted
  identity. This is the only object in this example allowed to create
  namespaces, RoleBindings, quotas and network policies across more than
  one team.
- **Three teams, the same shape three times**: `team-storefront`,
  `team-payments` and `team-loyalty`, each with its own namespace, its own
  `ServiceAccount`, a namespaced `Role` and a `RoleBinding` to that Role,
  scoped to that one namespace and nowhere else. The Role is deliberately
  narrower than the built-in `admin` ClusterRole: the team can run and expose
  its own apps, but can only read NetworkPolicies, ResourceQuotas and
  LimitRanges, and gets no RBAC, Namespace or Flux object rights at all.
- **Least-privilege sync**: each team's own `GitRepository` and
  `Kustomization` set `serviceAccountName` to the team's own ServiceAccount
  and `targetNamespace` to the team's own namespace. Those two fields have
  to agree with what the platform's RoleBinding granted, or the apply is
  refused.
- **Cluster-level guardrails, not just RBAC**: each team's namespace also
  gets a `ResourceQuota` and a `NetworkPolicy` that only admits traffic from
  pods in the same namespace. These are platform-set, in the file next to
  the RBAC grant, and a team cannot loosen them from its own folder: its
  Role grants no write on NetworkPolicy or ResourceQuota. That matters
  because NetworkPolicy allow rules are additive. A team bound to `admin`
  or `edit` could add its own allow-all NetworkPolicy beside the platform's
  and undo it without touching the platform's file.
- **Three different app shapes**: `storefront` and `payments-api` each ship
  a `Deployment` and a `Service`. `loyalty-api` ships a `Deployment` only.
  Tenants do not have to look identical for the boundary to hold.

## Repo layout

```
gitops/flux/expert-multi-tenant/
  clusters/shared/
    flux-system/README.md        # what flux bootstrap writes, and why it is not here
    tenants.yaml                 # the one layer: platform bootstrap, no serviceAccountName
  tenants/base/
    team-storefront/
      rbac.yaml                  # Namespace, ServiceAccount, Role, RoleBinding (platform-owned)
      guardrails.yaml            # ResourceQuota, NetworkPolicy (platform-owned)
      sync.yaml                  # the team's own GitRepository + Kustomization
      workloads/                 # the team's own app; reconciled by sync.yaml, not by kustomize here
    team-payments/                # same shape
    team-loyalty/                 # same shape, Deployment only
```

## What a governed tool should be able to say about this repo

If a tool reads this repo and cannot answer all of these, it does not
understand the repo yet:

| Question | Answer this repo gives |
|---|---|
| Which teams are here? | `team-storefront`, `team-payments`, `team-loyalty`. |
| Which namespaces? | `flux-system`, `team-storefront`, `team-payments`, `team-loyalty`. |
| Who owns what? | The platform owns `clusters/shared/` and every team's `rbac.yaml` and `guardrails.yaml`. Each team owns its own `workloads/` folder and nothing above it. |
| What identity applies each layer? | The platform bootstrap runs with the cluster's own trusted identity. Each team's `workloads/` is applied as that team's own ServiceAccount only. |
| What stops one team reaching another team's namespace? | The RoleBinding: each ServiceAccount's grant is a Role in its own namespace, bound by a RoleBinding in that same namespace, so no other namespace is reachable, and the NetworkPolicy admits ingress only from pods in the same namespace, where the cluster's CNI enforces NetworkPolicy. That also blocks an ingress controller running in another namespace, and it does not restrict egress. |
| What stops a team's pods reaching the node? | Pod Security Admission on its Namespace: `baseline` is enforced (and `restricted` warned), so no privileged pods, host namespaces or hostPath mounts. The built-in admission controller does this on Kubernetes 1.25 and later. The ResourceQuota also bounds storage, since the team can create PersistentVolumeClaims. |
| What stops a team loosening its own guardrails? | Its Role: it grants only get, list and watch on NetworkPolicies, ResourceQuotas and LimitRanges, and nothing on RBAC, Namespaces or Flux objects. A team cannot add a second, wider NetworkPolicy, raise its quota, widen its own grant, or rewrite its own `sync.yaml`. |
| Which values does ConfigHub see per team? | Two components, in two Spaces: `tenant-bootstrap` (platform-authored) in that team's bootstrap Space, and `tenant-workloads` (team-authored) in that team's own workloads Space. A ConfigHub Space belongs to exactly one Component, so the platform's and the team's configuration are never uploaded into the same Space. |

## Repository write access is the real boundary

The table above says each team owns only its `workloads/` folder. Nothing in
this repo enforces that. All three teams' GitRepositories and the platform's
`flux-system` source read the same repo and the same branch (`main`, see each
`sync.yaml`). The `tenants` Kustomization, which runs with the cluster's own
identity, applies `rbac.yaml`, `guardrails.yaml` and `sync.yaml` from that
same repo. So anyone who can push to it can edit a RoleBinding, bind
themselves to `cluster-admin`, and have the platform apply it.

The Role, the RoleBinding and the NetworkPolicy hold against a team's
Kustomization. They do not hold against a person with write access to the
repo. Pick one of these:

- one repository per team for `workloads/`, with the platform's repo
  writable by the platform team only;
- one repository with branch protection and a `CODEOWNERS` file that gives
  `clusters/` and `tenants/` (outside each `workloads/`) to the platform team
  and each `workloads/` folder to its own team.

This example uses one repo because it is small. The verifier cannot check
who may push.

## Controller flags this depends on

The isolation holds only when Flux's controllers run with the multi-tenancy
lockdown flags:

- `--no-cross-namespace-refs=true` on kustomize-controller (and on
  helm-controller, if a team uses `HelmRelease`), so a team's Kustomization
  cannot name a source or another object in a namespace that is not its own.
- `--default-service-account=<name>` on kustomize-controller (and
  helm-controller). Without it, a Kustomization with no `serviceAccountName`
  is applied with the controller's own identity, which is cluster-admin. That
  is the "missing serviceAccountName" break below: one deleted line and a team
  runs with the platform's rights. With the flag set, such a Kustomization
  runs as the named account in its own namespace instead, and its apply is
  refused unless that account has been granted something.

The platform's `tenants` Kustomization also sets no `serviceAccountName`, so
on a cluster with `--default-service-account` it needs one too: a
platform-owned ServiceAccount that can create namespaces and RBAC. This
example does not ship that account. Check the exact behavior for your Flux
version.

`./verify.sh` is the offline stand-in. It checks the manifests in this repo.
It does not read controller flags and cannot tell you whether a cluster runs
with them. The patch that sets them is documented in
[`clusters/shared/flux-system/README.md`](./clusters/shared/flux-system/README.md).

## What this example does not do

It does not create or manage a live Kubernetes cluster, does not run
`flux bootstrap`, and does not reconcile anything. What `setup.sh` actually
does is render the platform layer and every team's layers with `kustomize`
and upload the result into ConfigHub, so you can inspect and diff the same
config Flux would reconcile, without a cluster.

## Read-only first

```bash
cd gitops/flux/expert-multi-tenant
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` renders everything and uploads it into seven ConfigHub Spaces:
one for the platform's cluster-level bootstrap, and two per team, one for
that team's platform-authored bootstrap and one for that team's own
workloads. A ConfigHub Space belongs to exactly one Component, so a team's
bootstrap and its workloads always go to separate Spaces; that separation
is also what lets the platform's Spaces and the team's Space carry
different, correct `Owner` and `Environment` labels. `cub variant upload`
makes one Unit per rendered resource, so each Namespace, Role, Deployment and
so on becomes its own Unit; `./verify.sh` prints how many resources each
render holds. This mutates ConfigHub. It does not touch a live cluster.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates seven Spaces: one
  platform Space, plus a bootstrap Space and a workloads Space for each of
  the three teams). Does not mutate live infrastructure.
- `./verify.sh`: read-only. Renders locally and checks the output; does not
  call ConfigHub, Flux, or a cluster.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands for what `setup.sh` created.

## Break it on purpose

Each of these is one edit, and each breaks the tenant boundary in a way a
real shared cluster breaks. Make the edit, run `./verify.sh`, and see
whether the tool you are testing catches it before Kubernetes does:

1. **Tenant escape.** In
   `tenants/base/team-payments/sync.yaml`, change the Kustomization's
   `targetNamespace` from `team-payments` to `team-storefront`. The
   manifests still render, because `kustomize build` does not know about
   RBAC. On a live cluster, `team-payments`'s RoleBinding only grants
   rights inside the `team-payments` namespace, so the Kubernetes API
   server refuses the apply with a `Forbidden` error, and Flux marks that
   Kustomization as not `Ready`. `./verify.sh` catches the same
   misconfiguration offline: it checks that every team's `targetNamespace`
   matches the namespace its own RoleBinding grants.
2. **Missing serviceAccountName.** Remove `serviceAccountName: team-loyalty`
   from `tenants/base/team-loyalty/sync.yaml`. On a cluster without
   `--default-service-account`, the Kustomization would now reconcile with
   the controller's own cluster-admin identity instead of `team-loyalty`'s
   narrow one, the opposite failure: too much access rather than a refusal.
   With that flag set it would run as the default account and fail instead
   (see "Controller flags this depends on"). `./verify.sh` catches the
   missing field either way.
3. **Guardrail removed.** Delete the `NetworkPolicy` from
   `tenants/base/team-storefront/guardrails.yaml`. Nothing about the RBAC
   boundary changes, but `team-storefront` can now receive traffic from
   pods in any namespace, not just its own. `./verify.sh` catches the
   missing `NetworkPolicy`.
4. **Over-broad tenant grant.** In
   `tenants/base/team-storefront/rbac.yaml`, change the RoleBinding's
   `roleRef` to `kind: ClusterRole` and `name: admin`. The namespace scope
   still holds, but `admin` includes NetworkPolicy writes, so the team
   could now apply its own allow-all NetworkPolicy from `workloads/` and
   undo the platform's guardrail. `./verify.sh` catches it: every team's
   RoleBinding must point at its own Role, and no tenant Role may write
   NetworkPolicies, ResourceQuotas, LimitRanges, Namespaces, RBAC or Flux
   objects.

5. **Foreign subject.** In `tenants/base/team-storefront/rbac.yaml`, add a
   second subject to the RoleBinding: `kind: ServiceAccount`, `name:
   team-payments`, `namespace: team-payments`. The RoleBinding still points
   at `team-storefront`'s own Role, and the manifests still render. But
   `team-payments`'s Kustomization would now hold `team-storefront`'s rights
   in `team-storefront`'s namespace. `./verify.sh` catches it: every
   RoleBinding in a team's bootstrap must sit in that team's namespace and
   name only that team's own ServiceAccount, and the error names the team
   and the foreign subject.

Undo any edit with `git checkout -- .` before moving on.

## What proof this example gives, and what it does not

This example is offline and point-in-time. `./verify.sh` proves that the
manifests are structurally consistent: every team's `targetNamespace`
matches its own RoleBinding, every RoleBinding names only its own team's
ServiceAccount in its own namespace, every guardrail is present, no team's Role
can write the platform's guardrails, RBAC or Flux objects, and every
Kustomization path resolves. It does not run `flux bootstrap`, does not
apply anything to a cluster, and does not prove that Kubernetes would
actually return `Forbidden` for the break-it edits above; that would need a
live cluster read (`kubectl describe kustomization`, or Flux's own status
fields), which is outside what this example does. Once uploaded, a real
proof pass would also need to name ConfigHub's own intent for each team's
Space, separately from whatever a live cluster reports back.

## Pilot tasks we check

- Read this repo and say which teams, namespaces and RBAC boundaries exist,
  without changing anything.
- Show that each team's guardrails (quota and NetworkPolicy) apply to that
  team's own namespace and no other.
- Given the tenant-escape edit above, say plainly that it would be refused
  on a live cluster, name what refuses it (the RoleBinding scope, not this
  repo's rendering), and say what this example can and cannot prove about it
  offline. Today `verify.sh` catches this edit.
- Planned: a Pilot access-plan command that reads the RBAC this repo declares
  across teams. It does not exist in Pilot 0.6.0, and this example does not
  rely on it.

## Attribution

The layout follows the upstream Apache-2.0 Flux multi-tenancy pattern. No
files were copied from it. See [`NOTICE`](./NOTICE).

## Related examples

- [`../expert-fleet`](../expert-fleet/README.md): three clusters, one
  tenant, layered dependency ordering, and a promotion path. This example
  is the same tenant shape, scaled to several teams sharing one cluster
  instead of one team on its own cluster.
- [`../beginner`](../beginner/README.md): the same app family, one team,
  no tenancy.
- [`../../personas/flux-expert.md`](../../personas/flux-expert.md): the
  persona this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
