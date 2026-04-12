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

## Quick Start for Your Own App

If you already understand the model and want to onboard your own Spring Boot app:

```bash
# Generate starter cub-gen material
cub-gen springboot init --dry-run ./path/to/your-spring-app
cub-gen springboot init --app my-service ./path/to/your-spring-app

# Validate field mutations before they reach ConfigHub
cub-gen springboot validate-mutation --routes ./operational/field-routes.yaml \
  feature.myservice.someFlag        # ALLOWED
cub-gen springboot validate-mutation --routes ./operational/field-routes.yaml \
  spring.datasource.url             # BLOCKED

# Mutate app-owned embedded application.yaml directly
cub-gen springboot set-embedded-config \
  --routes ./operational/field-routes.yaml \
  --file ./confighub/my-service-prod.yaml \
  --configmap my-service-config \
  feature.myservice.someFlag optimistic
```

This generates platform policy skeletons, field ownership rules, and ConfigHub unit starters. See the [springboot-paas README](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas#onboard-your-own-spring-boot-app) for details.

## Concept Mapping

| Capability | spring-platform | cub-gen/springboot-paas |
|------------|-----------------|-------------------------|
| Generator explanation | `./setup.sh --explain` | `./generator/render.sh --explain` |
| Field-by-field lineage | `./generator/render.sh --trace` | `./generator/render.sh --trace` |
| Explain single field | `./generator/render.sh --explain-field X` | `./generator/render.sh --explain-field X` |
| Apply-here mutation | `cub function do set-env ...` | `cub function do set-env ...` |
| Direct embedded payload mutation | Documented concept only | `./demo-embedded-config-mutation.sh` / `cub-gen springboot set-embedded-config` |
| Lift-upstream bundle | `./lift-upstream.sh --render-diff` | `./lift-upstream.sh --render-diff` |
| Block/escalate boundary | `./block-escalate.sh --render-attempt` | `./block-escalate.sh --render-attempt` |
| Governed route proof | Model explanation | `./demo-governed-routes.sh` / `cub-gen springboot validate-mutation` |
| Real Kubernetes path | `./bin/create-cluster`, `./bin/build-image` | `./bin/create-cluster`, `./bin/build-image` |
| ConfigHub setup | `./confighub-setup.sh` | `./confighub-setup.sh` |
| Cross-env comparison | `./confighub-compare.sh` | `./confighub-compare.sh` |
| Refresh-survival preview | `./confighub-refresh-preview.sh` | `./confighub-refresh-preview.sh` |
| Scaffold your own app | `./bin/scaffold-app` | (not applicable — use `cub-gen springboot init`) |
| Onboard your own app | (not applicable) | `cub-gen springboot init` |
| Enforce field routes | (documented, not enforced) | `cub-gen springboot validate-mutation` |

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
- The current Spring path includes direct embedded-config mutation and route validation helpers

### Source-Chain Integration

In `spring-platform`:
- Fixtures represent the output state
- No discover/import/bridge workflow

In `cub-gen`:
- `demo-local.sh` runs the source-side verification chain
- `demo-governed-routes.sh` proves `ALLOW` and `BLOCKED` route outcomes
- `demo-embedded-config-mutation.sh` proves direct embedded `application.yaml` mutation
- `demo-connected.sh` runs the full ConfigHub integration
- Real generator profiles detect and transform Spring Boot inputs

## Recommended Path

1. **Learn with spring-platform** — Run the visibility scripts, understand field ownership, see the three mutation routes
2. **Onboard with cub-gen** — Run `cub-gen springboot init` on your app to generate starter material
3. **Validate mutations** — Use `cub-gen springboot validate-mutation` to enforce field routes in CI or locally
4. **Prove apply-here directly** — Use `cub-gen springboot set-embedded-config` or the example wrapper for embedded payload mutation
5. **Connect when needed** — Use `demo-connected.sh` for the deeper ConfigHub-backed evidence path

```bash
# Step 1: Learn the model
cd examples/spring-platform/springboot-platform-app
./setup.sh --explain
./generator/render.sh --trace

# Step 2: Onboard your app
cub-gen springboot init --app my-service ./path/to/your-spring-app
cd ./path/to/your-spring-app
# Review generated files: platform/, operational/, confighub/, .cub-gen/

# Step 3: Validate field routes
cub-gen springboot validate-mutation --routes ./operational/field-routes.yaml \
  feature.myservice.reservationMode    # ALLOWED
cub-gen springboot validate-mutation --routes ./operational/field-routes.yaml \
  spring.datasource.url                # BLOCKED

# Step 4: Direct embedded apply-here path
cub-gen springboot set-embedded-config \
  --routes ./operational/field-routes.yaml \
  --file ./confighub/my-service-prod.yaml \
  --configmap my-service-config \
  feature.myservice.reservationMode optimistic

# Step 5: Connected path
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
| Scaffold for adaptation | `cub-gen springboot init` for onboarding |
| No direct apply-here helper | `cub-gen springboot set-embedded-config` for embedded payload mutation |
| No enforcement helper | `cub-gen springboot validate-mutation` for local/CI route checks |
| Teaching-oriented docs | Product-oriented docs |

## Links

- [spring-platform README](./README.md) — the teaching examples
- [BRING-YOUR-OWN-APP.md](./BRING-YOUR-OWN-APP.md) — scaffold workflow (for adapting the fixed example)
- [cub-gen/examples/springboot-paas](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) — the product example
- [cub-gen springboot init](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas#onboard-your-own-spring-boot-app) — onboard your own app
