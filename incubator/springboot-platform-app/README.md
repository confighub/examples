# Spring Boot ConfigHub Example

ConfigHub stores Kubernetes YAML with mutation history and applies it to real clusters.

This example deploys a Spring Boot app (`inventory-api`) to a local Kind cluster via ConfigHub, then proves you can mutate config and see the change in the running app.

## What This Proves

1. **Store** - Kubernetes manifests (ServiceAccount, ConfigMap, Deployment, Service) stored as a ConfigHub unit
2. **Mutate** - Change config via `cub function do set-env` with audit trail
3. **Apply** - Real `kubectl apply` via ConfigHub worker
4. **Verify** - HTTP call to running pod confirms the mutation

## Quick Start

```bash
# Setup infrastructure (once)
./bin/create-cluster
./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker

# Deploy
export WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets

# Verify (hits real pod, shows reservationMode=strict)
./verify-e2e.sh

# Mutate
cub function do --space inventory-api-prod \
  --change-desc "change reservation mode" \
  -- set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
cub unit apply --space inventory-api-prod inventory-api

# Verify mutation (now shows reservationMode=optimistic)
./verify-e2e.sh

# Cleanup
./confighub-cleanup.sh
./bin/teardown
```

## Prerequisites

- `kind`, `kubectl`, Docker
- `cub` CLI (authenticated via `cub auth login`)
- Java 21+ and Maven (for building the app)

## What's in the Box

```
upstream/app/           # Spring Boot application source
confighub/*.yaml        # Kubernetes manifests per environment
bin/                    # Infrastructure scripts
  create-cluster        # Creates Kind cluster
  build-image           # Builds app, loads into Kind
  install-worker        # Starts ConfigHub worker
  teardown              # Cleanup
```

The app is trivial - an inventory API that returns JSON including a configurable `reservationMode`. The point is proving the ConfigHub workflow, not the app logic.

## Alternative: Simulation Mode

If you don't want a real cluster, use Noop targets:

```bash
./confighub-setup.sh --with-noop-targets
./confighub-verify.sh --noop-targets
```

This proves ConfigHub storage and mutation but does NOT deploy anything. No running pod to verify.

## What This Does NOT Prove

This example sketches two additional workflows that are **not implemented**:

- **Lift-upstream**: Route changes back to source repo via PR. There's a diff file in `lift-upstream/` but no automation.
- **Block/escalate**: Prevent certain field changes via policy. There's documentation in `changes/03-generator-owned.md` but no enforcement.

These are design concepts, not working features. See `changes/` directory for the design docs.

## Files

| Path | Purpose |
|------|---------|
| `confighub-setup.sh` | Creates ConfigHub spaces/units, optionally deploys |
| `confighub-cleanup.sh` | Deletes ConfigHub objects |
| `verify-e2e.sh` | HTTP verification against running pod |
| `confighub/*.yaml` | Kubernetes manifests per environment |

## Local App Development

The Spring Boot app can run standalone:

```bash
cd upstream/app
mvn test                    # Run tests
mvn spring-boot:run         # Run locally on :8080
curl localhost:8080/api/inventory/summary
```

## Troubleshooting

**Cluster not reachable**: Run `./bin/create-cluster` and ensure Docker is running.

**Worker not found**: Run `CUB_SPACE=springboot-infra ./bin/install-worker`.

**Pods not starting**: Check `kubectl get pods -n inventory-api` and `kubectl describe deployment inventory-api -n inventory-api`.
