# Block / Escalate

Stop or escalate the change request. Platform-owned fields cannot be mutated by app teams.

## When This Happens

A field matches a route with `defaultAction: generator-owned`. The field is platform-controlled and not safe for app-local divergence.

## Example

**Request**: Change `spring.datasource.url` to point to a different database.

**Why it's blocked**:
- Datasource connectivity is platform-owned
- The field is part of the runtime policy contract
- Direct mutation would bypass managed infrastructure

## Field Route

From `../springboot-platform-app/operational/field-routes.yaml`:

```yaml
- match: spring.datasource.*
  owner: platform-engineering
  defaultAction: generator-owned
  reason: Datasource connectivity is part of the managed platform boundary.

- match: securityContext.*
  owner: platform-engineering
  defaultAction: generator-owned
  reason: Runtime hardening must remain platform-controlled.
```

## What Happens

1. Developer requests datasource change via ConfigHub
2. ConfigHub checks field routes
3. Route says `generator-owned` -> change is blocked
4. ConfigHub flags the request for escalation
5. Platform engineer is notified
6. Platform engineer either:
   - Denies the request (explains why)
   - Makes the change through the platform path
   - Updates the runtime policy if appropriate

## Preview the Boundary

This example includes a dry-run boundary check:

```bash
../springboot-platform-app/block-escalate.sh --render-attempt
```

This shows what a datasource override attempt would look like.

## Files

- Route rule: `../springboot-platform-app/operational/field-routes.yaml`
- Platform policy: `../springboot-platform-app/upstream/platform/runtime-policy.yaml`
- Boundary renderer: `../springboot-platform-app/block-escalate.sh`
- Detailed docs: `../springboot-platform-app/changes/03-generator-owned.md`

## Key Insight

Not all fields are equal:
- **App-owned fields**: App team can mutate directly
- **Platform-owned fields**: Require platform approval or path

This separation keeps the platform boundary secure while giving app teams freedom where it's safe.

## Future State

Today, the block/escalate enforcement is documented but not fully server-enforced. The field-routes.yaml captures the policy, and ConfigHub's mutation layer will enforce it once field-level policy is implemented server-side.
