# Apply Here

Direct mutation in ConfigHub. The change applies to this deployment and survives refreshes.

## When This Happens

A field matches a route with `defaultAction: mutable-in-ch` and is owned by the app team.

## Example

**Request**: Change `feature.inventory.reservationMode` in prod from `strict` to `optimistic`.

**Why it applies here**:
- The field is app-owned (not platform-owned)
- It's per-deployment operational tuning
- It doesn't require upstream code changes

## Field Route

From `../springboot-platform-app/operational/field-routes.yaml`:

```yaml
- match: feature.inventory.*
  owner: app-team
  defaultAction: mutable-in-ch
  reason: Per-deployment rollout tuning is safe to keep in ConfigHub.
```

## What Happens

1. Developer requests the change via `cub function do` or UI
2. ConfigHub checks field routes
3. Route says `mutable-in-ch` -> change is allowed
4. ConfigHub stores the mutation with audit trail
5. Unit is marked dirty (pending apply)
6. `cub unit apply` delivers to target
7. Future refreshes preserve this mutation (policy-driven merge)

## Try It

After running `./setup.sh`:

```bash
# See current state
cub unit get --space inventory-api-prod inventory-api --json | jq '.Unit.Objects'

# Make the change
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "rollout: optimistic reservation mode" \
  set-env FEATURE_INVENTORY_RESERVATIONMODE optimistic

# Apply to target
cub unit apply --space inventory-api-prod inventory-api

# Verify the change
cub unit get --space inventory-api-prod inventory-api --json | jq '.Unit.Objects'
```

## Files

- Route rule: `../springboot-platform-app/operational/field-routes.yaml`
- Upstream default: `../springboot-platform-app/upstream/app/src/main/resources/application-prod.yaml`
- Detailed docs: `../springboot-platform-app/changes/01-mutable-in-ch.md`

## Key Insight

This is the "golden path" for operational tuning. The change is:
- Safe to apply locally
- Audited in ConfigHub
- Preserved across refreshes
- Immediately deliverable to targets
