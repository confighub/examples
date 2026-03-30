# Spring Boot Generator Example

How `cub-gen` transforms app + platform inputs into governed operational config.

## This View

This is the **Plain ConfigHub / Generator** view of the Spring platform model.

Use this example to understand:

- How the generator transforms app inputs and platform policies into operational Kubernetes config
- How ConfigHub stores and governs that operational config
- How field lineage explains ownership and mutation routes
- The artifact chain from inputs to deployed config

This is the best place to understand the generator itself.

## The Same Underlying Model

This is one of three views of the same underlying system:

| View | Example | Core question answered |
|------|---------|------------------------|
| **Plain ConfigHub** | **This example** | How does `cub-gen` transform app + platform into governed operational config? |
| ADT | [`springboot-platform-app-centric`](../springboot-platform-app-centric/) | How do I understand one app across deployments and targets? |
| Experimental ADTP | [`springboot-platform-platform-centric`](../springboot-platform-platform-centric/) | How do I make platform explicit above apps and deployments? |

All three examples share the same mutation routes and truth matrix structure. This example has the fullest implementation including real Kubernetes delivery.

## What Is Real Today

| Capability | Status |
|------------|--------|
| Generator transformation | Real |
| Field lineage / explain-field | Real |
| ConfigHub mutation storage | Real |
| Mutation history / audit trail | Real |
| Refresh preview | Real |
| Real Kubernetes delivery | Real (Kind cluster) |
| Noop target simulation | Real |
| Running app HTTP verification | Real |

## What Is Simulated Today

| Capability | Status |
|------------|--------|
| Noop target mode | Simulated (accepts apply, does not deliver) |

## What Is Not Implemented Yet

| Capability | Status |
|------------|--------|
| `lift upstream` automated PR | Bundle exists, no automated PR creation |
| `block/escalate` server-side enforcement | Documented, not enforced |
| Flux/Argo delivery path | See `global-app-layer` examples |

## Quick Start

```bash
cd spring-platform/springboot-platform-app

# Preview the example (read-only)
./setup.sh --explain
./setup.sh --explain-json | jq

# Verify fixtures are consistent
./verify.sh

# See the generator transformation
./generator/render.sh --explain
./generator/render.sh --explain-field spring.datasource.url
```

### ConfigHub Setup (mutates ConfigHub)

```bash
# Create spaces and units
./confighub-setup.sh

# Verify
./confighub-verify.sh

# Clean up
./confighub-cleanup.sh
```

### Real Kubernetes Deployment (requires Kind cluster)

```bash
# Setup infrastructure
./bin/create-cluster
./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig
export WORKER_SPACE=springboot-infra

# Deploy and verify
./confighub-setup.sh --with-targets
./verify-e2e.sh

# Cleanup
./confighub-cleanup.sh
./bin/teardown
```

## Artifact Chain

The generator transforms inputs into operational config:

```
upstream/app/           + upstream/platform/     → operational/
(Spring app inputs)       (Platform policies)      (Kubernetes manifests)
```

### Inputs

| Path | Owner | Purpose |
|------|-------|---------|
| `upstream/app/pom.xml` | App team | Build dependencies |
| `upstream/app/src/main/resources/application.yaml` | App team | App configuration |
| `upstream/app/src/main/resources/application-prod.yaml` | App team | Prod overrides |
| `upstream/platform/platform.yaml` | Platform team | Platform manifest |
| `upstream/platform/runtime-policy.yaml` | Platform team | Security and runtime policy |

### Generator

The generator is the transformation step:

```bash
./generator/render.sh --explain       # What the generator does
./generator/render.sh --trace         # Field-by-field mapping
```

### Outputs

| Path | Purpose |
|------|---------|
| `operational/deployment.yaml` | Kubernetes Deployment |
| `operational/configmap.yaml` | Kubernetes ConfigMap |
| `operational/service.yaml` | Kubernetes Service |
| `operational/field-routes.yaml` | Field ownership and mutation routes |

### ConfigHub

ConfigHub stores the operational config and governs mutations:

| Space | Unit | Environment |
|-------|------|-------------|
| `inventory-api-dev` | `inventory-api` | Development |
| `inventory-api-stage` | `inventory-api` | Staging |
| `inventory-api-prod` | `inventory-api` | Production |

## Mutation Routes

Every field has a route determined by its lineage:

| Route | When | Example field |
|-------|------|---------------|
| Apply here | App-owned, safe to mutate locally | `feature.inventory.reservationMode` |
| Lift upstream | App-owned, but needs source change | `spring.cache.*` |
| Block/escalate | Platform-owned | `spring.datasource.*` |

### Apply Here

```bash
# Change the reservation mode for prod
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
```

This mutation is stored in ConfigHub and survives future refreshes.

### Lift Upstream

```bash
# See the Redis caching bundle (read-only)
./lift-upstream.sh --explain
./lift-upstream.sh --render-diff
```

The bundle shows exactly what would change in upstream inputs and ConfigHub YAMLs.

### Block/Escalate

```bash
# See the datasource override boundary (read-only)
./block-escalate.sh --explain
./block-escalate.sh --render-attempt
```

The boundary is documented; server-side enforcement is not yet implemented.

### Why Is This Field Blocked?

```bash
./generator/render.sh --explain-field spring.datasource.url
# → BLOCKED: generator injects from platform policy

./generator/render.sh --explain-field feature.inventory.reservationMode
# → MUTABLE: comes from app inputs
```

## Compare This View To The Other Two

| Aspect | Plain ConfigHub (this) | ADT | Experimental ADTP |
|--------|------------------------|-----|-------------------|
| Focus | Generator transformation | App across environments | Platform organizing apps |
| Entry question | How does config get generated? | How does my app deploy? | How do I manage multiple apps? |
| Key insight | Field lineage → mutation routes | Deployments → spaces | Platform → apps |
| Best for | Understanding the machinery | Operating one app | Platform team operations |

## Key Files

### Public Commands

| Command | Purpose |
|---------|---------|
| `./setup.sh --explain` | Preview the example |
| `./verify.sh` | Verify fixtures |
| `./confighub-setup.sh` | Create ConfigHub objects |
| `./confighub-verify.sh` | Verify ConfigHub objects |
| `./confighub-cleanup.sh` | Delete ConfigHub objects |

### Generator

| Path | Purpose |
|------|---------|
| `generator/render.sh` | Generator CLI |
| `generator/README.md` | Generator documentation |

### Route Proofs (read-only)

| Command | Purpose |
|---------|---------|
| `./lift-upstream.sh` | Lift-upstream bundle preview |
| `./block-escalate.sh` | Block/escalate boundary preview |

### Internal Support

| Path | Purpose |
|------|---------|
| `bin/create-cluster` | Create Kind cluster |
| `bin/build-image` | Build Docker image |
| `bin/install-worker` | Start ConfigHub worker |
| `bin/teardown` | Clean up infrastructure |

## Prerequisites

**Structural proof:**
- `bash`, `jq`

**Local app proof:**
- Java 21+, Maven

**ConfigHub proof:**
- `cub` CLI, authenticated (`cub auth login`)

**Real Kubernetes deployment:**
- `kind`, `kubectl`, Docker

## AI Handoff

- AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Copyable prompts: [`prompts.md`](./prompts.md)
- Stable contracts: [`contracts.md`](./contracts.md)
