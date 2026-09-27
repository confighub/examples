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
| What stops one team reaching another team's namespace? | The RoleBinding: each ServiceAccount's grant is a Role in its own namespace, bound by a RoleBinding in that same namespace, so no other namespace is reachable, and the NetworkPolicy denies cross-namespace traffic even where RBAC is not the deciding factor. |
| What stops a team loosening its own guardrails? | Its Role: it grants only get, list and watch on NetworkPolicies, ResourceQuotas and LimitRanges, and nothing on RBAC, Namespaces or Flux objects. A team cannot add a second, wider NetworkPolicy, raise its quota, widen its own grant, or rewrite its own `sync.yaml`. |
| Which values does ConfigHub see per team? | Two components, in two Spaces: `tenant-bootstrap` (platform-authored) in that team's bootstrap Space, and `tenant-workloads` (team-authored) in that team's own workloads Space. A ConfigHub Space belongs to exactly one Component, so the platform's and the team's configuration are never uploaded into the same Space. |

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
different, correct `Owner` and `Environment` labels. This mutates
ConfigHub. It does not touch a live cluster.

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
2. **Missing impersonation.** Remove `serviceAccountName: team-loyalty` from
   `tenants/base/team-loyalty/sync.yaml`. The Kustomization would now
   reconcile with the platform's own trusted identity instead of
   `team-loyalty`'s narrow one, the opposite failure: too much access
   rather than a refusal. `./verify.sh` catches the missing field.
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

Undo any edit with `git checkout -- .` before moving on.

## What proof this example gives, and what it does not

This example is offline and point-in-time. `./verify.sh` proves that the
manifests are structurally consistent: every team's `targetNamespace`
matches its own RoleBinding, every guardrail is present, no team's Role
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
- **Access reduction across teams**: `pilot access-plan` reads the RBAC this
  repo declares for all three teams and reports where a team's access is
  wider than its own namespace, so an over-broad grant is visible before
  anyone has to wait for a live refusal to find it. On this example it
  reports each team reaching only its own namespace.
- Given the tenant-escape edit above, say plainly that it would be refused,
  name what refuses it (the RoleBinding scope, not this repo's rendering),
  and say what this example can and cannot prove about it offline. Today
  `verify.sh` catches this edit; `pilot access-plan` does not flag it yet.

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
