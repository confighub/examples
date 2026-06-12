# rbac-manager — Multi-Cluster Kubernetes RBAC as Data

Kubernetes RBAC management where a console action and a reviewable, versioned,
variant-aware, fleet-appliable artifact are **the same object** — because RBAC
config lives in ConfigHub as data, not as templates in a Git tree.

This example seeds a realistic multi-cluster RBAC fleet and demonstrates:

- **No change ceremony.** Adding one verb to one role is one operation with a
  change description and a diff — not a base + patch + overlay + kustomization
  tree.
- **Fleet selectors.** "Add a verb to the developer role on every staging
  cluster" is a single command over a label selector, not N pull requests.
- **Enforced guardrails, not advisory linting.** Wildcard rules, privilege
  escalation verbs, and cluster-admin bindings are blocked by Apply Gates
  before they reach a cluster. Prod changes additionally require approval.
- **Variants as data.** Each cluster's personas are clones of a canonical
  base; intentional divergence (dev may delete, prod may not) is tracked,
  diffable, and survives base upgrades.
- **Instant rollback.** Any change rolls back by restoring a prior revision —
  also recorded, also described.

A static webapp (an RBAC console built on ConfigHub's published SDK and API)
is being built on top of this layout; the seeded fleet is its demo dataset and
test fixture.

## What setup creates

```
rbac-demo-policy     Guardrail Triggers + Filters (no Units)
rbac-demo-base       Canonical persona Units: developer, operator, viewer, ci
rbac-demo-dev        Cluster Space (env=dev)     — persona clones + planted violations
rbac-demo-staging    Cluster Space (env=staging) — persona clones
rbac-demo-prod       Cluster Space (env=prod)    — persona clones, approval required
```

These are "paper clusters": Spaces only, no Targets or Workers — nothing
touches live infrastructure. The planted violations in dev make the audit and
gate stories visible immediately:

| Unit | What's wrong | Result |
|---|---|---|
| `legacy-wildcard-admin` | wildcard verbs/resources/apiGroups | **gated** by `no-wildcards` |
| `breakglass-cluster-admin` | standing cluster-admin binding | **gated** by `no-cluster-admin-binding` |
| `orphaned-grafana-binding` | roleRef to a Role that doesn't exist | no gate — surfaced by audit analysis |

## Prerequisites

- [cub CLI](https://docs.confighub.com/get-started/setup/#install-the-cli) installed
- Authenticated: `cub auth login`

## Usage

```bash
./setup.sh --explain        # preview the plan (no mutation)
./setup.sh                  # seed the fleet (idempotent; ConfigHub only)
./verify.sh                 # assert the layout and the gate matrix
```

Use `PREFIX=my-prefix ./setup.sh` to change the `rbac-demo-` Space prefix.

## Try it

```bash
# The missing fleet inventory: one query across all clusters
cub unit list --space "*" --where "Labels.persona = 'developer'"

# A gated violation, with the gate that blocks it
cub unit get legacy-wildcard-admin --space rbac-demo-dev -o jq=".Unit.ApplyGates"

# One fleet edit across a selector of clusters (server-side, comment-preserving)
cub function do --space "*" --where "Labels.persona = 'developer' AND Labels.env = 'staging'" \
  --change-desc "Allow developers to deletecollection in staging" \
  -- yq-i 'select(.kind == "ClusterRole").rules[0].verbs += ["deletecollection"]'

# And roll it back
cub unit update developer --space rbac-demo-staging --patch \
  --restore Before:HeadRevisionNum --change-desc "Roll back the staging edit"
```

For a paced, stage-by-stage walkthrough, see [AI_START_HERE.md](AI_START_HERE.md).
Stable command contracts for automation are in [contracts.md](contracts.md).

## Boundaries

This example manages desired-state RBAC configuration. It is deliberately
**not** an identity provider or OIDC group mapper, **not** just-in-time/
time-bound elevation, and **not** runtime admission control (Kyverno/OPA are
complementary — and their policies can be managed as ConfigHub data too).
