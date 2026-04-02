# Spring Boot Generator Example

See how app config + platform policy becomes the `ConfigMap`, `Deployment`, and `Service` for this app, and why each field routes to apply-here, lift-upstream, or block.

```bash
./setup.sh --explain
```

This example has the most complete proof path: real Kubernetes delivery via Kind, field-by-field lineage tracing, and all three mutation routes shown end to end. The apply-here route is proven with audited mutation; lift-upstream and block-escalate are still bundle-and-boundary workflows rather than server-enforced behavior.

## Quick Start

```bash
./setup.sh --explain           # see the generator model (read-only)
./verify.sh                    # check fixtures
./generator/render.sh --explain  # see the transformation
```

### ConfigHub Setup

```bash
./confighub-setup.sh           # create spaces and units
./confighub-verify.sh          # verify
./confighub-cleanup.sh         # delete
```

### Real Kubernetes (requires Kind cluster)

```bash
./bin/create-cluster && ./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets
./verify-e2e.sh
```

## The Artifact Chain

```
upstream/app/           + upstream/platform/     → operational/
(Spring app inputs)       (Platform policies)      (Kubernetes manifests)
```

The generator reads app inputs (`pom.xml`, `application.yaml`) and platform policies (`runtime-policy.yaml`), then produces ConfigMap, Deployment, and Service manifests with field-level provenance.

```bash
./generator/render.sh --trace   # field-by-field mapping
```

## Mutation Routes

Every field has a route based on its source:

```bash
./generator/render.sh --explain-field spring.datasource.url
# → BLOCKED: generator injects from platform policy

./generator/render.sh --explain-field feature.inventory.reservationMode
# → MUTABLE: comes from app inputs
```

### Apply Here

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
```

### Lift Upstream

```bash
./lift-upstream.sh --explain      # see the Redis bundle
./lift-upstream.sh --render-diff  # see exact changes
```

### Block/Escalate

```bash
./block-escalate.sh --explain       # see the boundary
./block-escalate.sh --render-attempt  # see blocked attempt
```

Server-side enforcement is documented but not yet implemented.

## Can I Use My Own App?

Yes. Use the scaffold command to generate a renamed copy for your app:

```bash
./bin/scaffold-app my-service --output ../my-service
```

This handles the mechanical renaming (app name, Java package, ConfigHub slugs, field-routes). You still need to:

- Replace the stub app code with your actual service
- Review field ownership and ports
- Run the same proof path

The scaffold is regression-checked (`./bin/verify-scaffold`). Full details: [`../BRING-YOUR-OWN-APP.md`](../BRING-YOUR-OWN-APP.md)

## Key Files

| Path | Purpose |
|------|---------|
| `generator/render.sh` | Generator CLI (--explain, --trace, --explain-field) |
| `operational/field-routes.yaml` | Field ownership rules |
| `lift-upstream.sh` | Lift-upstream bundle preview |
| `block-escalate.sh` | Block/escalate boundary preview |
| `bin/*` | Kind cluster and worker setup |

## Prerequisites

- `bash`, `jq` for structural proof
- Java 21+, Maven for local app
- `cub` CLI for ConfigHub
- `kind`, `kubectl`, Docker for real deployment

## Related

See [`../README.md`](../README.md) for how the three examples compare.
See [`../BRING-YOUR-OWN-APP.md`](../BRING-YOUR-OWN-APP.md) for adapting the example to your own service.

AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
