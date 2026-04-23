# Case 04: Dev/Prod Variants

## Goal

Make the strongest product lesson from Kubara explicit:

> ConfigHub should manage the difference between dev and prod variants instead
> of discovering those differences as broken ACME issuers, secret stores, or
> admission behavior during a live sync.

This is the promotion case. It should show why ConfigHub is more than a safer
`kubectl`.

Fresh-Claude handoff prompt: [`PROMPT.md`](./PROMPT.md).

## Scenario

One laptop cluster can still model two environments:

- `mini-dev` namespace or space;
- `mini-prod` namespace or space.

The workload is tiny, but the config differs:

- dev uses a fake issuer/email and local secret backend;
- prod requires real-looking issuer/email, stricter policy, and a different
  secret-store reference.

The app should be simple:

```text
ApplicationSet/mini-variant-dev
  -> Application/controlplane-mini-variant-dev

ApplicationSet/mini-variant-prod
  -> Application/controlplane-mini-variant-prod
```

The important object is not the Deployment. The important object is the
variant data and promotion rule.

## What This Trains

- Expressing dev/prod differences as governed data.
- Preventing dev-only values from being promoted silently.
- Showing promotion as a real product surface.
- Separating "dev acceptable WATCH" from "prod BLOCK."
- Explaining customer value without overloading the demo with real cloud
  infrastructure.

## Route

```text
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for variant data and appset delivery; PROMOTION for dev -> prod
Scope: one app across two variants
Wrong move: copying dev rendered YAML into prod without variant review
```

## Proposed Fixture

Local fixture in this repo:

```text
incubator/mini-kubara/case-04-dev-prod-variants/fixtures/
  applicationsets/mini-variant-dev.yaml
  applicationsets/mini-variant-prod.yaml
  variants/dev-values.yaml
  variants/prod-values.yaml
  policies/promotion-variant-check.md
```

Public graduation targets:

- `examples/confighub-promotions/`;
- Spring Platform app-centric examples;
- a future Kubara rerun with real dev/prod spaces.

## Prompt To Run

```text
Run Mini-Kubara Case 04: Dev/Prod Variants.

Start read-only. Identify the dev and prod variant data and state which fields
are allowed to differ. Use ConfigHub proof surfaces to show the current dev and
prod intended state.

Run the dev path first. Then propose a promotion gate that compares dev and
prod and blocks if dev-only values would leak into prod.

After approval, promote only the approved fields. Prove prod received the right
variant values, dev-only data stayed out, Argo converged, and the final report
names what would need real credentials or cloud resources in a production
version.
```

## Expected Proof

- Dev and prod variant values are visible before mutation.
- The promotion gate names allowed and forbidden differences.
- A deliberately bad promotion attempt is blocked or classified.
- The corrected promotion changes only approved fields.
- Dev and prod Applications converge independently.
- The report says which pieces are simulated because this is one local kind
  cluster.

## Stop Conditions

- Dev and prod values are not distinguishable before promotion.
- The same rendered YAML is copied to both variants without review.
- A fake ACME email, fake secret-store reference, or dev admission relaxation
  reaches prod.
- Promotion proof lacks ConfigHub revision/diff evidence.
- The case starts claiming production readiness from a one-laptop cluster.

## What This Does Not Prove

- Real cloud identity.
- Real ACME registration.
- Real multi-cluster promotion latency.

It proves the governance model for variants. That is enough for this mini case.
