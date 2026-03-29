# Generator: Spring Boot Platform Renderer

This directory makes visible the "generator" - the transformation step that combines app inputs and platform policies to produce operational Kubernetes config.

## The Transformation

```
┌─────────────────────────────────────────────────────────────────┐
│                        GENERATOR                                 │
│                                                                  │
│  upstream/app/                    operational/                   │
│  ├── application.yaml     ──┐     ├── deployment.yaml           │
│  ├── application-stage.yaml ├──▶  ├── configmap.yaml            │
│  └── application-prod.yaml ──┤     └── service.yaml              │
│                              │                                   │
│  upstream/platform/          │                                   │
│  ├── runtime-policy.yaml   ──┤                                   │
│  └── slo-policy.yaml       ──┘                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# See what the generator does
./render.sh --explain

# Machine-readable version
./render.sh --explain-json | jq

# Field-by-field mapping
./render.sh --trace

# What would change if re-rendered
./render.sh --diff
```

## Why This Matters

The generator is the source of truth for field ownership. When you understand how a field got into `operational/deployment.yaml`, you know:

| Field Source | Mutation Route | Example |
|--------------|----------------|---------|
| App input (app-team owned) | `mutable-in-ch` | `feature.inventory.reservationMode` |
| App input (but durable change) | `lift-upstream` | `spring.cache.type` |
| Platform policy | `generator-owned` | `SPRING_DATASOURCE_URL` |

## Key Transformations

### 1. App Name → Kubernetes Labels

```yaml
# Input: application.yaml
spring:
  application:
    name: inventory-api

# Output: deployment.yaml, service.yaml, configmap.yaml
metadata:
  name: inventory-api
  labels:
    app.kubernetes.io/name: inventory-api
```

### 2. Platform Datasource → Environment Variable

```yaml
# Input: runtime-policy.yaml
spec:
  managedDatasource: postgres-shared

# Output: deployment.yaml
env:
  - name: SPRING_DATASOURCE_URL
    value: jdbc:postgresql://postgres.platform.svc:5432/inventory
```

This field is `generator-owned`. App teams cannot change it in ConfigHub because it's controlled by platform policy.

### 3. Spring Configs → ConfigMap

```yaml
# Input: application.yaml, application-prod.yaml, application-stage.yaml

# Output: configmap.yaml
data:
  application.yaml: |
    spring:
      application:
        name: inventory-api
    # ...
  application-prod.yaml: |
    feature:
      inventory:
        reservationMode: strict
```

Fields inside these configs can be `mutable-in-ch` (like `feature.inventory.reservationMode`) or `lift-upstream` (like `spring.cache.type`).

### 4. Port → Container and Service

```yaml
# Input: application-prod.yaml
server:
  port: 8081

# Output: deployment.yaml
containers:
  - ports:
      - containerPort: 8081

# Output: service.yaml
spec:
  ports:
    - targetPort: 8081
```

## Field Routes

The generator also produces `operational/field-routes.yaml` which tells ConfigHub how to handle mutations to each field:

```yaml
routes:
  - match: feature.inventory.*
    owner: app-team
    defaultAction: mutable-in-ch
    reason: Per-deployment rollout tuning is safe to keep in ConfigHub.

  - match: spring.cache.*
    owner: app-team
    defaultAction: lift-upstream
    reason: Cache adoption changes the app contract.

  - match: spring.datasource.*
    owner: platform-engineering
    defaultAction: generator-owned
    reason: Datasource connectivity is part of the managed platform boundary.
```

## What This Example Does NOT Include

This is a **demonstrative** generator, not a production one. It does not:

- Actually render YAML (operational/ files are pre-rendered)
- Support arbitrary app inputs
- Integrate with CI/CD pipelines
- Generate provenance metadata

For real platform rendering, see tools like:
- Helm templates
- Kustomize
- cue-lang
- Jsonnet
- Custom platform generators (Backstage, Kratix, etc.)

## See Also

- `../operational/field-routes.yaml` - Field ownership rules
- `../changes/03-generator-owned.md` - What happens when you try to change generator-owned fields
- `../README.md` - Full example documentation
