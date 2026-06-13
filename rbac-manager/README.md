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

**Real use** — install the guardrails on your own Spaces:

```bash
./setup.sh --explain                          # preview (no mutation)
./setup.sh                                    # all Spaces with Kubernetes/YAML units
./setup.sh --where-space "Slug LIKE 'prod-%'" # narrow with a filter expression
./setup.sh --policy-space my-policies         # choose where the guardrails live
./verify.sh                                   # confirm the guardrails are installed
```

The three guardrail Triggers are defined **once** in a policy Space
(`policy-guardrails` by default) and enforced fleet-wide via a shared Trigger
Filter — each in-scope Space's `TriggerFilterID` is pointed at that Filter,
not given its own copy. They're created with `Warn=true`: violations surface
as advisory **ApplyWarnings**, never blocking anyone. Promote a guardrail to a
blocking ApplyGate once the warnings are clean —
`cub trigger update <slug> --space policy-guardrails --unwarn` — and that one
change enforces it everywhere. Spaces that already select their Triggers
another way (a custom `WhereTrigger`, a different `TriggerFilterID`, or
Triggers of their own) are reported rather than modified. The app analyzes
everything you can view by default; narrow it from the **Scope** button
(filter expressions over Targets and Spaces).

**Demo** — a self-contained fleet with planted violations and blocking gates:

```bash
./demo-setup.sh --explain   # preview the plan (no mutation)
./demo-setup.sh             # seed the demo fleet (idempotent; ConfigHub only)
./demo-verify.sh            # assert the layout and the gate matrix
```

Use `PREFIX=my-prefix ./demo-setup.sh` to change the `rbac-demo-` Space prefix.

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
