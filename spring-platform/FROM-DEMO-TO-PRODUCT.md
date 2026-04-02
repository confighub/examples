# From Demo to Product

This is the page for the question a platform engineer asks after running the examples:

> "Is `render.sh` the real generator, or a demo prop?"

Short answer: In `spring-platform`, it's a teaching tool. In `cub-gen/examples/springboot-paas`, it's the real generator path.

## Two Repos, One Model

| Repo | Purpose | When to use |
|------|---------|-------------|
| `examples/spring-platform` | Learn the model | Understanding generator transformation, mutation routes, field ownership |
| `cub-gen/examples/springboot-paas` | Run the real generator | Evaluating the product path for your own services |

Both implement the same release-day challenge (feature flag, Redis caching, datasource override) with the same three mutation routes. The difference is what's behind the scripts.

## Concept Mapping

| Capability | spring-platform | cub-gen/springboot-paas |
|------------|-----------------|-------------------------|
| Generator explanation | `./setup.sh --explain` | `./generator/render.sh --explain` |
| Field-by-field lineage | `./generator/render.sh --trace` | `./generator/render.sh --trace` |
| Explain single field | `./generator/render.sh --explain-field X` | `./generator/render.sh --explain-field X` |
| Apply-here mutation | `cub function do set-env ...` | `cub function do set-env ...` |
| Lift-upstream bundle | `./lift-upstream.sh --render-diff` | `./lift-upstream.sh --render-diff` |
| Block/escalate boundary | `./block-escalate.sh --render-attempt` | `./block-escalate.sh --render-attempt` |
| Real Kubernetes path | `./bin/create-cluster`, `./bin/build-image` | `./bin/create-cluster`, `./bin/build-image` |
| ConfigHub setup | `./confighub-setup.sh` | `./confighub-setup.sh` |
| Cross-env comparison | `./confighub-compare.sh` | `./confighub-compare.sh` |
| Refresh-survival preview | `./confighub-refresh-preview.sh` | `./confighub-refresh-preview.sh` |
| Scaffold your own app | `./bin/scaffold-app` | (not applicable) |

The command patterns are identical. The difference is implementation depth.

## What's Different

### Generator Implementation

In `spring-platform`:
- `generator/render.sh` is a bash script that explains fixed fixtures
- The transformation is documented, not computed
- Field explanations come from hardcoded mappings

In `cub-gen`:
- `cub-gen springboot` is a Go binary that detects Spring Boot projects
- The transformation is computed from `pom.xml`, `application.yaml`, and platform policies
- Field lineage is traced through actual code paths

### Source-Chain Integration

In `spring-platform`:
- Fixtures represent the output state
- No discover/import/bridge workflow

In `cub-gen`:
- `demo-local.sh` runs the source-side verification chain
- `demo-connected.sh` runs the full ConfigHub integration
- Real generator profiles detect and transform Spring Boot inputs

## Recommended Path

1. **Learn with spring-platform** — Run the visibility scripts, understand field ownership, see the three mutation routes
2. **Try scaffold** — Generate a renamed copy for your app shape using `./bin/scaffold-app`
3. **Evaluate with cub-gen** — Point the real generator at your app and see computed output

```bash
# Step 1: Learn the model
cd examples/spring-platform/springboot-platform-app
./setup.sh --explain
./generator/render.sh --trace

# Step 2: Scaffold your shape
./bin/scaffold-app my-service --dry-run
./bin/scaffold-app my-service --output ../my-service

# Step 3: Run the real generator
cd /path/to/cub-gen
go build -o ./cub-gen ./cmd/cub-gen
./examples/springboot-paas/demo-local.sh

# Step 4: Connected path
cub auth login
./examples/springboot-paas/demo-connected.sh
```

## What Carries Over

Everything you learn in `spring-platform` applies directly to `cub-gen`:

- **Field ownership model** — `feature.*` is app-owned, `spring.datasource.*` is platform-owned
- **Mutation routes** — apply-here, lift-upstream, block/escalate work the same way
- **ConfigHub commands** — `cub function do set-env`, `cub unit apply`, `cub mutation list`
- **Verification pattern** — `verify.sh` proves structural consistency in both repos

## What Changes

| In spring-platform | In cub-gen |
|--------------------|------------|
| Fixed inventory-api example | Your actual app |
| Hardcoded field explanations | Computed from source |
| Scaffold for adaptation | Generator for production |
| Teaching-oriented docs | Product-oriented docs |

## Links

- [spring-platform README](./README.md) — the teaching examples
- [BRING-YOUR-OWN-APP.md](./BRING-YOUR-OWN-APP.md) — scaffold workflow
- [cub-gen/examples/springboot-paas](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) — the product example
