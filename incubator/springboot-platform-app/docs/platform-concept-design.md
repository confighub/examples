# Platform Concept Design

This document designs how to make the "Platform" concept explicit and discoverable in the springboot examples.

## Problem Statement

The platform concept is currently **implicit**:
- Platform policies exist in `upstream/platform/` as scattered YAML files
- The generator transforms them into operational config, but the link is invisible
- Field ownership (mutable-in-ch vs generator-owned) is documented but not discoverable
- App teams don't have a clear way to understand "what does the platform provide/control?"

## Design Goals

1. **Make platform boundaries visible** — What the platform provides and controls
2. **Connect policies to fields** — Which policy controls which field, and why
3. **Enable discovery** — Tools to query platform rules without reading code
4. **Support onboarding** — Clear story for app teams joining the platform

## Current State

```
upstream/
  app/                        # App team owns this
    application.yaml          # Spring config (feature flags, etc.)
    application-prod.yaml     # Prod overrides

  platform/                   # Platform team owns this
    runtime-policy.yaml       # Security, managed services
    slo-policy.yaml          # Performance guarantees

operational/                  # Generator output (nobody edits directly)
  deployment.yaml            # Has fields from both sources
  configmap.yaml             # Embedded Spring configs
  field-routes.yaml          # Mutation rules (disconnected from policies)

generator/                    # Shows transformation but not provenance
  render.sh                  # Demonstrates the transformation
```

**Gap:** There's no way to answer "why is `spring.datasource.url` blocked?" without manually tracing through files.

## Proposed Design

### 1. Platform Manifest

Create a single, authoritative `Platform` resource that consolidates what the platform provides and controls:

```yaml
# upstream/platform/platform.yaml
apiVersion: platform.confighub.io/v1alpha1
kind: Platform
metadata:
  name: springboot-platform
spec:
  description: |
    Heroku-like Spring Boot platform providing managed datasource,
    runtime hardening, and observability defaults.

  # What the platform provides to apps
  provides:
    - name: managed-datasource
      type: postgres-shared
      description: Managed PostgreSQL with HA, encryption, and automated backups
      fieldPattern: spring.datasource.*

    - name: runtime-hardening
      description: Security defaults (runAsNonRoot, mTLS sidecar)
      fieldPattern: securityContext.*

    - name: observability
      description: Required health endpoints and SLO targets
      slo:
        availability: 99.9%
        p95LatencyMs: 250

  # Field ownership rules (replaces field-routes.yaml)
  fieldRoutes:
    - match: feature.inventory.*
      owner: app-team
      action: mutable-in-ch
      reason: Per-deployment rollout tuning is app-team safe

    - match: spring.cache.*
      owner: app-team
      action: lift-upstream
      reason: Cache adoption changes app contract, needs upstream code changes

    - match: spring.datasource.*
      owner: platform-engineering
      action: generator-owned
      provides: managed-datasource
      reason: Datasource is managed by platform for HA and security

    - match: securityContext.*
      owner: platform-engineering
      action: generator-owned
      provides: runtime-hardening
      reason: Runtime hardening must remain platform-controlled
```

### 2. Enhanced Generator with Provenance

Update `generator/render.sh` to support provenance queries:

```bash
# Query: What controls this field?
./generator/render.sh --explain-field spring.datasource.url

# Output:
# Field: spring.datasource.url
# Owner: platform-engineering
# Action: generator-owned (blocked in ConfigHub)
# Provides: managed-datasource
# Reason: Datasource is managed by platform for HA and security
#
# Source chain:
#   1. upstream/platform/platform.yaml → spec.provides[managed-datasource]
#   2. Platform injects: SPRING_DATASOURCE_URL=jdbc:postgresql://postgres.platform.svc:5432/inventory
#   3. Output: operational/deployment.yaml → env[SPRING_DATASOURCE_URL]

# Query: What does the platform provide to this app?
./generator/render.sh --platform-summary

# Output:
# Platform: springboot-platform
#
# Provides:
#   managed-datasource   PostgreSQL with HA, encryption, backups
#   runtime-hardening    Security defaults (runAsNonRoot, mTLS)
#   observability        Health endpoints, SLO targets
#
# App-owned fields (mutable-in-ch):
#   feature.inventory.*  Per-deployment rollout tuning
#
# App-owned fields (lift-upstream):
#   spring.cache.*       Cache adoption (requires code changes)
#
# Platform-owned fields (blocked):
#   spring.datasource.*  Managed datasource boundary
#   securityContext.*    Runtime hardening
```

### 3. Field Annotations in Operational Output

Add optional provenance annotations to generated manifests:

```yaml
# operational/deployment.yaml (with --annotate-provenance)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inventory-api
  annotations:
    platform.confighub.io/name: springboot-platform
    platform.confighub.io/provides: managed-datasource,runtime-hardening,observability
spec:
  template:
    spec:
      containers:
        - name: inventory-api
          env:
            - name: SPRING_DATASOURCE_URL
              value: jdbc:postgresql://postgres.platform.svc:5432/inventory
              # Could be in a separate provenance.yaml for cleaner manifests
            - name: FEATURE_INVENTORY_RESERVATIONMODE
              value: strict
```

### 4. Platform Onboarding Guide

Create `docs/platform-onboarding.md`:

```markdown
# Platform Onboarding: Spring Boot Apps

Welcome to the Spring Boot platform. This guide explains what the platform
provides, what you can configure, and what requires platform approval.

## What the Platform Provides

| Capability | Description | You get |
|------------|-------------|---------|
| Managed Datasource | PostgreSQL | HA, encryption, backups, connection pooling |
| Runtime Hardening | Security | runAsNonRoot, mTLS sidecar |
| Observability | Monitoring | Health endpoints, alerting at 99.9% SLO |

## What You Can Configure

### Directly in ConfigHub (mutable-in-ch)

These fields are safe to change per-deployment without upstream code changes:

| Field Pattern | Example | Use Case |
|---------------|---------|----------|
| `feature.inventory.*` | `reservationMode=optimistic` | Feature flag rollouts |

### Via Upstream PR (lift-upstream)

These fields require code changes and will be routed back to your app repo:

| Field Pattern | Example | Use Case |
|---------------|---------|----------|
| `spring.cache.*` | Adding Redis caching | Changes app dependencies |

## What You Cannot Configure

### Platform-Controlled (generator-owned)

These fields are managed by the platform and will be blocked:

| Field Pattern | Why | To Change |
|---------------|-----|-----------|
| `spring.datasource.*` | Managed datasource | Contact platform-team |
| `securityContext.*` | Security compliance | Contact platform-team |

## How to Check Field Ownership

```bash
# See what the platform provides
./generator/render.sh --platform-summary

# Explain a specific field
./generator/render.sh --explain-field spring.datasource.url
```
```

### 5. Directory Structure After Design

```
springboot-platform-app/
  upstream/
    app/                          # Unchanged
    platform/
      platform.yaml               # NEW: Consolidated platform manifest
      runtime-policy.yaml         # Kept for reference
      slo-policy.yaml            # Kept for reference

  operational/
    deployment.yaml              # Unchanged
    configmap.yaml               # Unchanged
    service.yaml                 # Unchanged
    field-routes.yaml            # DEPRECATED: Merged into platform.yaml

  generator/
    render.sh                    # ENHANCED: --explain-field, --platform-summary
    README.md                    # Updated with new commands

  docs/
    platform-concept-design.md   # This document
    platform-onboarding.md       # NEW: App team onboarding guide
```

## Implementation Phases

### Phase 1: Platform Manifest (This PR)

1. Create `upstream/platform/platform.yaml` consolidating policies and field routes
2. Update `generator/render.sh` with `--explain-field` and `--platform-summary`
3. Create `docs/platform-onboarding.md`
4. Update README to reference platform concept

### Phase 2: Generator Provenance (Future)

1. Add `--annotate-provenance` flag to generator
2. Generate field-to-policy mapping in operational output
3. Add provenance trace to generator README

### Phase 3: Discovery Tools (Future)

1. CLI: `cub platform show <app>` — list platform policies affecting an app
2. CLI: `cub platform explain <field>` — explain field ownership
3. GUI: Visual platform boundary in unit detail view

### Phase 4: Enforcement (Future)

1. Server-side field policy validation in ConfigHub
2. Real block/escalate with error messages
3. Approval workflows for escalations

## Trade-offs

| Approach | Pros | Cons |
|----------|------|------|
| Single Platform manifest | One source of truth, easy discovery | Duplication if policies also exist elsewhere |
| Keep separate policy files | Modular, composable | Discovery requires reading multiple files |
| Annotations in manifests | Provenance visible in output | Clutters manifests, may conflict with GitOps |

**Decision:** Start with single Platform manifest + enhanced generator. Provenance annotations are optional.

## Success Criteria

1. App team can run one command to understand platform boundaries
2. "Why is this field blocked?" is answerable without reading code
3. Platform onboarding takes < 5 minutes to understand
4. Field ownership is explicit, not implicit

## Open Questions

1. Should `platform.yaml` replace `field-routes.yaml` or coexist?
2. Should platform discovery be a generator feature or a separate tool?
3. How does this interact with ConfigHub's (future) field-level policy enforcement?
