# AI Guide: rbac-manager

Multi-cluster Kubernetes RBAC managed as data: canonical personas, per-cluster
variants, enforced guardrails (Apply Gates), fleet-wide edits over selectors,
and revision rollback.

## CRITICAL: Demo Pacing

Run ONE stage at a time. After each stage, STOP and wait for the human before
continuing. Do not run ahead, even if the next command is obvious. The human
may want to inspect the GUI between stages.

## Suggested Prompt

> Walk me through the rbac-manager example stage by stage. Pause after each
> stage so I can look around. I'm authenticated with cub already.

## Stage 1: Seed and verify the fleet

```bash
./demo-setup.sh --explain        # preview first — mutates nothing
./demo-setup.sh                  # then seed (ConfigHub only, no live infra)
./demo-verify.sh                 # 42 checks: layout + gate matrix
```

Expected: `All checks passed. (42 checks)`. Re-running demo-setup.sh is safe; it
skips existing entities.

GUI gap: there is no single fleet-level view of "which Spaces are RBAC
clusters and what's gated in each" — that is what the rbac-manager webapp
will add on top of this layout.

GUI feature ask: a Space-group dashboard summarizing Units, Apply Gates, and
unapplied changes across a label-selected set of Spaces.

**PAUSE.** Wait for the human.

## Stage 2: The missing fleet inventory

The community complaint this answers: "seeing who can do what is easy, but on
which cluster is another issue."

```bash
# Every variant of the developer persona, across all clusters, one query
cub unit list --space "*" --where "Labels.persona = 'developer'"

# What changed locally since the clone? (dev developers gained delete, on purpose)
cub unit diff -u developer --space rbac-demo-dev --from=1 --to=HeadRevisionNum

# What would an upgrade pull from upstream? When base moves, this previews the
# incoming change. "No new changes" means either upstream hasn't moved (the
# case here) or a local override already covers the upstream change — local
# divergence is preserved by upgrades, not overwritten.
cub unit update --patch --upgrade --dry-run -o mutations --space rbac-demo-dev developer
```

Expected: four developer Units (base + 3 clusters); the local diff shows
dev's extra `delete` verb; the upstream dry-run reports `No new changes`;
and `cub revision list developer --space rbac-demo-dev` shows the change
description that introduced the divergence.

**PAUSE.** Wait for the human.

## Stage 3: Guardrails are gates, not advice

```bash
# The planted wildcard role is blocked from ever being applied
cub unit get legacy-wildcard-admin --space rbac-demo-dev -o jq=".Unit.ApplyGates"

# So is the standing cluster-admin binding
cub unit get breakglass-cluster-admin --space rbac-demo-dev -o jq=".Unit.ApplyGates"

# Prod changes carry an approval gate out of the box
cub unit get developer --space rbac-demo-prod -o jq=".Unit.ApplyGates"
```

Expected gates: `rbac-demo-policy/no-wildcards/vet-celexpr`,
`rbac-demo-policy/no-cluster-admin-binding/vet-celexpr`, and
`rbac-demo-policy/require-approval/vet-approvedby` respectively. The orphaned
binding (`orphaned-grafana-binding`) has NO gate — dangling references are an
audit finding for analysis tooling, not a policy violation.

**PAUSE.** Wait for the human.

## Stage 4: One edit, a selector of clusters

The GitOps way to add one verb to one role is a base + patch + overlay tree.
Here it is as one operation over a fleet selector — server-side, so YAML
comments and formatting are preserved:

```bash
cub function do --space "*" \
  --where "Labels.persona = 'developer' AND Labels.env = 'staging'" \
  --change-desc "Allow developers to deletecollection in staging" \
  -- yq-i 'select(.kind == "ClusterRole").rules[0].verbs += ["deletecollection"]'

cub unit data --space rbac-demo-staging developer   # see the new verb, comments intact
```

Widen or narrow the blast radius by editing the `--where` clause — that
sentence is the whole "selective apply across clusters" feature.

**PAUSE.** Wait for the human.

## Stage 5: Roll it back

```bash
cub unit update developer --space rbac-demo-staging --patch \
  --restore Before:HeadRevisionNum \
  --change-desc "Roll back the staging fleet-edit demo"

./demo-verify.sh    # the fleet is back to its seeded state — all 42 checks pass
```

The rollback is itself a described, attributable revision — nothing is lost.

GUI gap: revision restore points (`Before:HeadRevisionNum`, ChangeSet refs)
are CLI-only vocabulary today.

GUI ask: a "restore to this revision" button on the revision history view
that composes the change description automatically.

**PAUSE.** Wait for the human.

## Cleanup

There is no cleanup script by design (demo data is meant to persist for the
webapp built on top of it). To tear down manually:

```bash
for s in rbac-demo-dev rbac-demo-staging rbac-demo-prod rbac-demo-base rbac-demo-policy; do
  cub space delete "$s" --recursive
done
```
