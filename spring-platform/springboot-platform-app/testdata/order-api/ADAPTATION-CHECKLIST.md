# Adaptation Checklist

This scaffold was generated from the `inventory-api` example. The mechanical renaming is done. You still need to review and adapt the following:

## Required: Replace the Sample App

The Java code in `upstream/app/` is still the inventory-api stub. Replace it with your actual application:

- [ ] Replace `src/main/java/.../` with your app's source code
- [ ] Update `pom.xml` with your dependencies
- [ ] Update `Dockerfile` if your build process differs

## Required: Review Ports and Health Paths

- [ ] `operational/deployment.yaml` — containerPort (currently 8081 for prod)
- [ ] `operational/service.yaml` — targetPort
- [ ] `confighub/*.yaml` — any port references
- [ ] Health check paths if not `/actuator/health`

## Required: Review Environment Variables

- [ ] `operational/deployment.yaml` — env section
- [ ] `confighub/*.yaml` — embedded env vars
- [ ] Add any app-specific env vars your service needs

## Required: Review Field Ownership

- [ ] `operational/field-routes.yaml` — decide which fields are:
  - `mutable-in-ch` (app team can change in ConfigHub)
  - `lift-upstream` (requires source change)
  - `generator-owned` (platform-controlled, blocked)


## Current Field Routes

```yaml
routes:
  - match: feature.orderapi.*
    owner: app-team
    defaultAction: mutable-in-ch
    reason: Per-deployment rollout tuning is safe to keep in ConfigHub.
  - match: spring.cache.*
    owner: app-team
    defaultAction: lift-upstream
    reason: Cache adoption changes the app contract and should update upstream app inputs.
  - match: spring.datasource.*
    owner: platform-engineering
    defaultAction: generator-owned
    reason: Datasource connectivity is part of the managed platform boundary.
  - match: securityContext.*
    owner: platform-engineering
    defaultAction: generator-owned
    reason: Runtime hardening must remain platform-controlled.
```

Update the `feature.orderapi.*` pattern to match your app's feature flags.

## Optional: Image Registry

- [ ] Update image references in `confighub/*.yaml` for your registry
- [ ] Update `bin/build-image` if using a different registry

## Optional: Platform Policy

- [ ] Review `upstream/platform/runtime-policy.yaml`
- [ ] Review `upstream/platform/slo-policy.yaml`

## Verification

After adaptation, run:

```bash
cd order-api
./verify.sh
./confighub-setup.sh --with-noop-targets
./confighub-verify.sh
```

## What Was Renamed

| Original | Renamed to |
|----------|------------|
| inventory-api | order-api |
| Inventory* (Java) | OrderApi* |
| com.example.inventory | com.example.orderapi |
| feature.inventory.* | feature.orderapi.* |
