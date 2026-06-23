# RBAC Manager Over A Redis Helm Chart

Status: companion design example.

This example explains the smallest useful pattern for
[`rbac-manager-for-agents`](../rbac-manager-for-agents/README.md): use the RBAC
manager as a specialist tool over one Helm-sourced component that has been
loaded into ConfigHub.

The RBAC manager is not installed as a dependency of Redis. Redis stays the app.
The RBAC manager reads and changes the RBAC-related ConfigHub Units that came
from the Redis chart.

## Shape

```text
Helm chart: bitnami/redis
ConfigHub component: redis
ConfigHub variants:
  redis/base
  redis/dev
  redis/prod

Units produced from the chart:
  StatefulSet
  Service
  ConfigMap
  Secret reference
  ServiceAccount
  Role or ClusterRole, if enabled by chart values
  RoleBinding or ClusterRoleBinding, if enabled by chart values
```

The RBAC manager works over those Units:

```text
redis/prod Units
  -> cub-rbac snapshot
  -> cub-rbac who-can get secrets
  -> cub-rbac findings --severity high
  -> cub-rbac edit ...     # dry-run by default
```

## Why this is useful

A human or AI agent should not have to inspect Redis YAML by hand to answer RBAC
questions. The RBAC manager gives the agent Kubernetes-specific operations:

| Question | RBAC-manager route |
| --- | --- |
| Does the Redis service account exist? | `snapshot` or `list --kind ServiceAccount` |
| Can the Redis service account read Secrets? | `access ServiceAccount:<namespace>/<name>` |
| Who can update Redis RBAC objects? | `who-can update roles` / `who-can update rolebindings` |
| Are any Redis bindings too broad? | `findings --severity high` |
| Can this be tightened safely? | `edit ...` dry-run, then ConfigHub diff and gates |

## Example agent task

```text
Task:
  Check whether redis/prod grants broader RBAC permissions than redis/base.

Agent route:
  1. Use cub-rbac snapshot scoped to the redis component.
  2. Compare ServiceAccount, Role, ClusterRole, RoleBinding, and
     ClusterRoleBinding Units between redis/base and redis/prod.
  3. Run cub-rbac findings.
  4. If a safe edit exists, create a dry-run change.
  5. Let ConfigHub show the exact Unit diff before commit.
```

## Ownership rule

If the changed RBAC field belongs to the upstream Redis base, prefer changing
the base or chart values. If the change is intentionally environment-specific,
make it a derived variant change and keep the ownership visible.

The RBAC manager should not silently patch a downstream Redis Unit in a way that
will be overwritten or confused by the next promotion from the base.

## What this example does not claim

- It does not add a new Redis workload.
- It does not replace the Redis Helm chart.
- It does not prove a live Redis install.
- It does not make every RBAC edit safe.

It shows the product shape: Helm produces explicit ConfigHub Units, and an
agentic domain tool operates over those Units with dry-run, guardrails, and
reviewable changes.
