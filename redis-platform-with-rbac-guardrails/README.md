# Redis Platform With RBAC Guardrails

Status: runnable offline companion example.

This example explains the larger-product pattern for
[`rbac-manager-for-agents`](../rbac-manager-for-agents/README.md): Redis is one
part of a platform or app stack, and RBAC guardrails are managed across the whole
product.

In this shape, the RBAC manager is not "added to Redis." It is a specialist
operations tool for the full ConfigHub-managed product.

## Shape

```text
Component: payments-platform

Variants:
  payments-platform/base
  payments-platform/dev
  payments-platform/staging
  payments-platform/prod-us
  payments-platform/prod-eu

Included pieces:
  redis
  payments-api
  ingress
  external-secrets
  monitoring
  RBAC policy and guardrails
```

Each piece may come from a different source:

| Piece | Source |
| --- | --- |
| Redis | public Helm chart loaded through a ConfigHub recipe |
| API service | custom app Units |
| Ingress | public Helm chart or platform component |
| External Secrets | public Helm chart plus target prerequisites |
| RBAC policy | ConfigHub guardrail Units and RBAC-manager edits |

## Why this is useful

Real apps are not one chart. They are a product shape with multiple components,
target facts, variants, policies, and operating rules.

The RBAC manager can answer questions across the product:

| Product question | RBAC-manager route |
| --- | --- |
| Which service accounts can read Secrets in prod? | `who-can get secrets --space-where ...` |
| Does dev have broader RBAC than staging? | `snapshot` plus variant comparison |
| Which bindings were introduced by Redis versus the API? | `list` joined with ConfigHub Unit labels |
| Can we tighten the API role in every dev environment? | `fleet-edit` dry-run |
| Can the staging RBAC policy be promoted to prod? | `promote` with ConfigHub diff and gates |

## Example agent task

```text
Task:
  Prepare an RBAC hardening change for payments-platform/prod-us.

Agent route:
  1. Use cub-rbac snapshot over the payments-platform component.
  2. Run who-can and findings across redis, payments-api, ingress, and
     external-secrets Units.
  3. Propose the smallest RBAC edit.
  4. Keep the edit as a ConfigHub dry-run until a human or policy gate approves.
  5. Promote the approved policy through staging and prod variants.
  6. Observe the live result.
```

## Variant boundary

Use the source of the change to decide where it belongs:

| Change | Better place |
| --- | --- |
| Redis chart values change the rendered RBAC object set | Redis base variant or recipe |
| Prod needs a narrower RoleBinding than dev | Derived prod variant |
| Every app in the platform needs the same warning | Guardrail policy Unit |
| A one-off incident fix was made live | Reverse-reconcile only if an authority rule allows it |

## What this example does not claim

- It does not define a finished payments-platform product.
- It does not claim Redis needs custom RBAC in every install.
- It does not run a live promotion.
- It does not replace human review for security-sensitive changes.

## Run it

Preview the example without writing anything:

```bash
cd redis-platform-with-rbac-guardrails
./setup.sh --explain
./setup.sh --explain-json | jq
```

Generate the local RBAC outputs and verify them:

```bash
./setup.sh
./verify.sh
```

This writes only local files under `sample-output/`. It does not mutate
ConfigHub and it does not touch a Kubernetes cluster.

## What the concrete `payments` example contains

The fixture models one component, `payments-platform`, with `base`, `dev`,
`staging`, `prod-us`, and `prod-eu` variants.

The `prod-us` shape contains:

- Redis from a public Helm chart recipe.
- A custom `payments-api` service.
- A Redis Secret supplied by the target.
- RBAC guardrail Units.
- One deliberately visible finding: the API Role can list all Secrets in the
  namespace and should be narrowed before promotion.

The generated outputs show:

- the component map;
- an RBAC snapshot;
- who can get Secrets in `payments-prod`;
- the current finding;
- the dry-run hardening edit.

It shows the product shape: multiple Helm and custom app pieces can be treated
as one ConfigHub-managed product, and a domain-specific agent tool can operate
over that product without reducing the work to raw YAML edits.
