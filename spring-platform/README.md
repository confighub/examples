# Spring Platform in ConfigHub

This repo teaches an experimental app-platform model for Spring Boot services and ConfigHub.  We have several different 'versions' of the idea, for comparison and learning. The example assumes that many users will want some generated config, and looks at a way to enable this with the CH paradigm.

## Start Here

- Review the model in this repo if you want to see the ideas
- Later, for a 'real Boot app', the version at [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) has a path for your own Spring Boot app. Start there with `cub-gen springboot init`.  Also very experimental.

## What to Expect from Each Example

This repo contains three variants of the same model plus a shared dependency
directory. They differ in scope, runnability, and intended audience. The table
below tells you what you can and cannot do with each one.

| Directory | What it is | What you can do | What it does NOT do |
|-----------|-----------|-----------------|---------------------|
| [`springboot-platform-app`](./springboot-platform-app/) | Working generator with a Kind cluster pipeline | Deploy to Kind, run `verify-e2e.sh`, see the full artifact chain from Spring config to running pod | Not a minimal starting point -- this is the heaviest example |
| [`springboot-platform-app-centric`](./springboot-platform-app-centric/) | Teaching fixture (ADT model) -- one app across 3 environments | Run `demo.sh`, explore mutation routes (apply-here / lift-upstream / block), read `--explain` output | No Kubernetes deployment, no compiled Java app |
| [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) | Teaching fixture (ADTP model) -- 2 apps across 5 environments | Run `platform.sh --summary`, see multi-app governance and cross-app field ownership | No Kubernetes deployment, noop targets only |
| [`shared/`](./shared/) | Reusable YAML referenced by ADT and ADTP examples | Nothing standalone -- this is a dependency, not an example | Not runnable on its own |

### Reading order

If you are new to the model, start with **app-centric** -- it is the simplest
lens and introduces the three mutation routes with a single app. Move to
**platform-centric** when you want to see how ownership scales across multiple
apps. Use **vanilla app** when you want to see the full generator pipeline
deployed to a real cluster.

### Concrete expectations

**springboot-platform-app** has real shell scripts (`setup.sh`, `render.sh`,
`confighub-setup.sh`, `verify-e2e.sh`) and a `generator/` directory that
transforms Spring config inputs into Kubernetes manifests. The `upstream/`
directory contains a `pom.xml` and `application.yaml` -- these are config
fixtures, not a compilable Java project. You can deploy to a Kind cluster and
run end-to-end verification.

**springboot-platform-app-centric** is a walkthrough. It has `demo.sh` with
`--explain` and `--trace` flags, `flows/` with mutation walkthroughs, and YAML
files describing the ADT (App-Deployment-Target) structure. No Java source, no
Kubernetes resources -- this is for understanding the model.

**springboot-platform-platform-centric** adds a second app (`catalog-api`) and
shows how a platform team governs shared infrastructure. It has `platform.sh`
for discovery (`--apps`, `--summary`, `--explain-field`) and uses noop targets
for demonstration. Same scope as app-centric but from the platform team's
perspective.

**shared/** contains ConfigHub unit YAML files and `field-routes.yaml`. Both
ADT and ADTP examples reference these via `../shared/` paths.

## When You Want a Runnable Example

The examples in this repo teach the conceptual model with fixed inputs and
explain scripts. If you want to run `cub-gen` against a real Spring Boot
application with actual Java source code, config governance, and a local demo
you can execute end-to-end without a cluster:

**[`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas)** has:

- A real (minimal) Spring Boot app: `InventoryApplication.java`, a REST
  controller, a service class, `pom.xml` with Spring Boot 3.3.2 / Java 21
- Spring config files: `application.yaml`, `application-dev.yaml`,
  `application-prod.yaml` with profile overrides
- Platform policies, a FrameworkRegistry, and Flux/Argo transport configs
- `demo-local.sh` that runs the full discover/import/publish/verify lifecycle
  with no external dependencies

You can run it right now:

```bash
cd cub-gen
go build -o ./cub-gen ./cmd/cub-gen
./cub-gen gitops discover --space platform --json ./examples/springboot-paas
./cub-gen gitops import --space platform --json ./examples/springboot-paas ./examples/springboot-paas
```

If you have studied the model here and want to move to real tooling, that is
the bridge.

## What This Repo Is For

**Use this repo to study the model.** App teams keep writing normal Spring Boot apps in Git. Platform tooling maps those app inputs into the operational artifacts needed to run on Kubernetes with GitOps.

That operational layer includes ConfigHub units and spaces, Kubernetes manifests, platform policy, and GitOps-facing state.

## The Model

App teams write Spring Boot applications. They use `application.yaml`, profiles, and the normal Spring config surface. That stays the authoring experience.

Platform tooling then maps those inputs into:
- ConfigHub units and spaces
- Kubernetes manifests (ConfigMap, Deployment, Service)
- Platform policy (security, datasource boundaries)
- GitOps-facing operational state

This mapping from app to platform is a deterministic config generator, and it is intentionally constrained:

| Property | Why it matters |
|----------|----------------|
| Invertible mapping | Every operational field traces back to exactly one source field |
| Field provenance | You can explain why any field has its value |
| Ownership boundaries | Fields are app-owned or platform-owned, not ambiguous |
| Mutation from provenance | How a field can change depends on who owns it |

The point is not "generate all the YAML." The point is to generate only the operational config that the platform must own, while keeping the path back to app inputs clear.

## Mutation Routes

Every operational field change falls into one of three categories:

| Route | Owner | What happens |
|-------|-------|--------------|
| **Apply here** | App team | Mutate directly in ConfigHub |
| **Lift upstream** | App team | Requires source change, route back to Git |
| **Block/escalate** | Platform | Cannot be changed without platform approval |

These routes are derived from field provenance and ownership, not assigned arbitrarily. If you know where a field comes from, you know how it can change.

## Three Versions of the app platform

This repo shows the same model through three lenses:

| View | Example | What it helps you see |
|------|---------|------------------------|
| Vanilla ConfigHub | [`springboot-platform-app`](./springboot-platform-app/) | How app inputs and platform policy generate operational outputs |
| ADT | [`springboot-platform-app-centric`](./springboot-platform-app-centric/) | How the CH app, deployment and target model works |
| ADTP | [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) | How platform ownership applies across multiple apps |

These are three 'lenses' on the same example, not three different examples.

## STORY: The Release-Day Proof

We imagine that "a major product launch is in 24 hours". Three requests land at the same time:

1. **Flip a feature flag in prod** — safe, urgent, do it now
2. **Add Redis caching** — valuable, but it requires a code change
3. **Point staging at a different database** — dangerous, must be refused

Each request maps to one of the three mutation routes:

| Request | Route | Why |
|---------|-------|-----|
| Enable optimistic reservation mode | **Apply here** | App-owned operational tuning |
| Add Redis-backed caching | **Lift upstream** | Requires source changes |
| Change the staging datasource | **Block/escalate** | Crosses a platform-owned boundary |

The worked examples show how each story looks in practice.

## Quick Start: Learn the Model

```bash
cd springboot-platform-app
./setup.sh --explain                                          # what this creates
./generator/render.sh --trace                                 # field-by-field: input → output
./generator/render.sh --explain-field feature.inventory.reservationMode  # MUTABLE: app-owned
./generator/render.sh --explain-field spring.datasource.url              # BLOCKED: platform policy
```

Create ConfigHub objects and handle the feature flag request:

```bash
./confighub-setup.sh                                          # create dev/stage/prod spaces
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "release-day: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
./confighub-compare.sh                                        # see the * on prod
./confighub-refresh-preview.sh prod                           # PRESERVE: your change survives
```

Handle the Redis request (needs to go back to source):

```bash
./lift-upstream.sh --explain                                  # why this routes upstream
./lift-upstream.sh --render-diff                              # the exact patch bundle
```

Handle the datasource request (must be refused):

```bash
./block-escalate.sh --explain                                 # why this is blocked
./block-escalate.sh --render-attempt                          # what the dry-run looks like
```

## Quick Start: Onboard Your Own App

If you already understand the model, use `cub-gen springboot init`:

```bash
cub-gen springboot init --dry-run ./path/to/your-spring-app
cub-gen springboot init --app my-service ./path/to/your-spring-app
```

This generates platform policy skeletons, field ownership rules, and ConfigHub unit starters. See [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) for the full product-side path.

## ADT View: One App Across Environments

```bash
cd springboot-platform-app-centric
./setup.sh --explain          # the ADT view: app → deployments → targets
./setup.sh                    # create spaces, units, noop targets
./demo.sh                     # walk through all three mutation outcomes
```

## ADTP View: Platform Governing Multiple Apps

```bash
cd springboot-platform-platform-centric
./setup.sh --explain          # the platform view
./setup.sh                    # create 6 spaces, 5 units
./platform.sh --summary       # what the platform provides
./platform.sh --apps          # which apps run on it
./platform.sh --explain-field spring.datasource.url   # BLOCKED — for all apps on this platform
```

## What's Implemented

| Capability | Status |
|------------|--------|
| Generator transformation | Real |
| Field lineage / explain-field | Real |
| ConfigHub mutation + audit history | Real |
| Real Kubernetes delivery (Kind) | Real (Phase 1 only) |
| Noop target simulation | Real |
| Refresh-survival preview | Simulated (client-side) |
| Lift-upstream automated PR | Bundle only, no PR |
| Block/escalate enforcement | Documented, not enforced |

## From This Repo to the Real Product Path

`spring-platform` teaches the model with fixed inputs and explain scripts. It shows why each field is mutable, lifted upstream, or blocked.

`cub-gen/examples/springboot-paas` is the product-side path with real detection, real generation, and real enforcement.

| Here | There |
|------|-------|
| Fixed inventory-api example | Your actual app |
| Hardcoded field explanations | Computed from source |
| Scaffold for adaptation | `cub-gen springboot init` for onboarding |
| Documented boundaries | `cub-gen springboot validate-mutation` for CI |

Full concept mapping: [`FROM-DEMO-TO-PRODUCT.md`](./FROM-DEMO-TO-PRODUCT.md).

## What Teams Ask Next

> "Is `render.sh` the real generator, or a demo prop?"

In `spring-platform`, `render.sh` explains the fixed example. In `cub-gen`, `springboot-paas` has the real generator. That's the right next step for teams evaluating the product path.

## AI Guidance

See [`AI_START_HERE.md`](./AI_START_HERE.md) for paced AI-assisted walkthroughs.
