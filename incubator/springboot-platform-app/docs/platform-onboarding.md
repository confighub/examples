# Platform Onboarding: Spring Boot Apps

Welcome to the Spring Boot platform. This guide explains what the platform provides, what you can configure, and what requires platform approval.

## Quick Start

```bash
# See what the platform provides
./generator/render.sh --platform-summary

# Understand why a field is blocked
./generator/render.sh --explain-field spring.datasource.url

# See field-by-field ownership
./generator/render.sh --trace
```

## What the Platform Provides

| Capability | You Get | Platform Manages |
|------------|---------|------------------|
| **Managed Datasource** | PostgreSQL connection | HA, encryption, backups, connection pooling |
| **Runtime Hardening** | Secure defaults | runAsNonRoot, mTLS sidecar |
| **Observability** | Health endpoints | Alerting at 99.9% SLO, p95 < 250ms |

Your app automatically gets these capabilities without configuration. The platform handles the operational burden.

## What You Can Configure

### Directly in ConfigHub (mutable-in-ch)

These fields are safe to change per-deployment without code changes:

| Field Pattern | Example | Use Case |
|---------------|---------|----------|
| `feature.inventory.*` | `reservationMode=optimistic` | Feature flag rollouts, gradual rollouts |

**How to change:**
```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "rollout: enable optimistic reservation mode" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

cub unit apply --space inventory-api-prod inventory-api
```

These changes:
- Are stored in ConfigHub with audit trail
- Survive generator refreshes (PRESERVE policy)
- Apply immediately to targets

### Via Upstream PR (lift-upstream)

These fields require code changes and will be routed back to your app repo:

| Field Pattern | Example | Why |
|---------------|---------|-----|
| `spring.cache.*` | Adding Redis caching | Needs pom.xml and application.yaml changes |

**How to change:**
1. Request the change in ConfigHub (captures intent)
2. ConfigHub routes it to lift-upstream
3. A PR is created against your app repo
4. After merge, platform re-renders operational config
5. ConfigHub refreshes from the new state

Preview what the PR would look like:
```bash
./lift-upstream.sh --render-diff
```

## What You Cannot Configure

### Platform-Controlled (generator-owned)

These fields are managed by the platform and will be blocked:

| Field Pattern | Why Blocked | To Change |
|---------------|-------------|-----------|
| `spring.datasource.*` | Managed datasource provides HA, encryption, backups | Contact platform-team |
| `securityContext.*` | Security compliance requirements | Contact platform-team |

**What happens if you try:**
1. ConfigHub blocks the mutation
2. You receive an error with escalation instructions
3. The change is not applied

**Escalation:**
- Slack: #platform-support
- Email: platform@example.com
- Process: Open a ticket explaining why the change is needed

## Understanding Field Ownership

To check if a field is mutable:

```bash
# Check a specific field
./generator/render.sh --explain-field spring.datasource.url

# See all field ownership
./generator/render.sh --trace
```

Field ownership is determined by:
1. **Who provides the value** — app inputs vs platform policies
2. **What happens if it diverges** — safe vs risky
3. **Policy in platform.yaml** — explicit field routes

## The Platform Boundary

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            YOUR APP                                      │
│  upstream/app/                                                          │
│    application.yaml         ← You own this                              │
│    application-prod.yaml    ← You own this                              │
│                                                                          │
│  What you can change directly:                                          │
│    feature.inventory.*      ✓ mutable-in-ch                            │
│                                                                          │
│  What needs upstream PR:                                                 │
│    spring.cache.*           ↑ lift-upstream                             │
├─────────────────────────────────────────────────────────────────────────┤
│                         PLATFORM BOUNDARY                                │
├─────────────────────────────────────────────────────────────────────────┤
│                         THE PLATFORM                                     │
│  upstream/platform/                                                      │
│    platform.yaml            ← Platform team owns this                   │
│    runtime-policy.yaml      ← Platform team owns this                   │
│                                                                          │
│  What platform controls:                                                 │
│    spring.datasource.*      ✗ generator-owned (blocked)                 │
│    securityContext.*        ✗ generator-owned (blocked)                 │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────────┐
                    │          GENERATOR            │
                    │ Combines app + platform       │
                    │ Produces operational config   │
                    └───────────────────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────────┐
                    │    operational/               │
                    │    deployment.yaml            │
                    │    configmap.yaml             │
                    │    service.yaml               │
                    └───────────────────────────────┘
```

## Getting Help

| Question | Where to Look |
|----------|---------------|
| "Why is this field blocked?" | `./generator/render.sh --explain-field <field>` |
| "What can I configure?" | `./generator/render.sh --platform-summary` |
| "How did this value get here?" | `./generator/render.sh --trace` |
| "I need to change a blocked field" | Slack #platform-support |

## See Also

- [Platform Concept Design](./platform-concept-design.md) — Design rationale
- [Generator README](../generator/README.md) — How the transformation works
- [Platform Manifest](../upstream/platform/platform.yaml) — Full policy definition
- [Field Routes](../operational/field-routes.yaml) — Mutation routing rules
