# Lift Upstream

Route the change back to the app source repository. This is for changes that need to be in code, not just operational config.

## When This Happens

A field matches a route with `defaultAction: lift-upstream`. The change requires upstream code or config modification.

## Example

**Request**: Add Redis-backed caching to the service.

**Why it lifts upstream**:
- The app code must gain a Redis dependency (pom.xml)
- Spring config must grow cache settings (application.yaml)
- The operational shape changes as a consequence of the app change

## Field Route

From `../springboot-platform-app/operational/field-routes.yaml`:

```yaml
- match: spring.cache.*
  owner: app-team
  defaultAction: lift-upstream
  reason: Cache adoption changes the app contract and should update upstream app inputs.
```

## What Happens

1. Developer requests Redis caching via ConfigHub
2. ConfigHub checks field routes
3. Route says `lift-upstream` -> change cannot be applied locally
4. ConfigHub captures the intent
5. A lift-upstream bundle is generated (patch for app source)
6. Developer creates PR against app repo with the bundle
7. After merge, platform re-renders operational config
8. ConfigHub refreshes from the new rendered state

## The Bundle

This example includes a pre-built Redis bundle at:
```
../springboot-platform-app/lift-upstream/redis-cache/
```

Preview what would change:

```bash
../springboot-platform-app/lift-upstream.sh --render-diff
```

This shows:
- pom.xml changes (Redis dependency)
- application.yaml changes (cache type: redis)
- ConfigHub YAML changes (new env vars for Redis)

## Files

- Route rule: `../springboot-platform-app/operational/field-routes.yaml`
- Upstream pom.xml: `../springboot-platform-app/upstream/app/pom.xml`
- Upstream config: `../springboot-platform-app/upstream/app/src/main/resources/application.yaml`
- Redis bundle: `../springboot-platform-app/lift-upstream/redis-cache/`
- Bundle renderer: `../springboot-platform-app/lift-upstream.sh`
- Detailed docs: `../springboot-platform-app/changes/02-lift-upstream.md`

## Key Insight

Some changes cannot live in operational config because they require:
- New dependencies
- New code
- Changes to the app contract

ConfigHub detects these and routes them back upstream, keeping operational config clean and coherent.
