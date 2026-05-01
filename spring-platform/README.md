# Spring Platform in ConfigHub

This repo teaches why platform teams want **Generators**.

A Generator is a function on config data. It reads files teams already keep in
Git, such as Spring Boot code, `application.yaml`, profile overrides, platform
policy, and environment settings. It returns deployable Kubernetes config plus
proof: where each field came from, who owns it, and whether a change should be
applied here, lifted back to source, or blocked.

This example is the teaching version. It uses fixed Spring inputs and explain
scripts so you can see the model without installing a full platform. The
product version is [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas).

The desired reaction after reading this is simple: "I want this Generator model
for my own apps, not a pile of one-off templates."

```text
Spring app + platform policy
  -> Generator
  -> deployable Kubernetes config
  -> ConfigHub proof
```

## Start Here

- Review the model in this repo if you want to understand Generators before
  running product tooling.
- Then use [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas)
  for the runnable product path. Start there with `demo-local.sh`, then the
  governed route and embedded-config wrappers, and use `cub-gen springboot init`
  when you want to onboard your own app.

Important distinction: this repo uses `cub function do set-env` as the
teaching-era apply-here mutation path. The current product path in `cub-gen`
uses `cub-gen springboot set-embedded-config` to mutate the embedded
`application.yaml` payload directly, and `cub-gen springboot validate-mutation`
to check routes before the mutation is accepted.

## What to Expect from Each Example

All three examples use the same app (`inventory-api`) and the same three
mutation requests (feature flag, Redis caching, datasource override). They
differ in *perspective* and *what you can run*.

**[`springboot-platform-app`](./springboot-platform-app/)** — Start here.
One app, one environment. You run shell scripts that generate Kubernetes
manifests from Spring config, then explain each field: who owns it, how it
can change, and why. If you have a Kind cluster, you can deploy and verify
end-to-end. If not, the scripts still work -- they show the transformation
and field routing without a cluster.

**[`springboot-platform-app-centric`](./springboot-platform-app-centric/)** —
Same app, now across three environments (dev, stage, prod). Each environment
is a ConfigHub space. You walk through all three mutation routes with
`demo.sh` and see how the same field has different effective values per
environment. No cluster needed -- this is a guided walkthrough.

**[`springboot-platform-platform-centric`](./springboot-platform-platform-centric/)** —
Two apps (`inventory-api` + `catalog-api`), five environments total. This
shows the platform team's view: how ownership rules apply across multiple
apps, and how `platform.sh --explain-field` answers "is this field blocked
for all apps, or just one?" No cluster needed.

**[`shared/`](./shared/)** — Not an example. Contains YAML files that the
app-centric and platform-centric examples reference. You don't need to look
at this unless you're modifying the examples.

These examples are still model-first, but they no longer stop at pure fixture
docs. In particular, `springboot-platform-app/upstream/app` now contains a
minimal Java source tree alongside the Spring config and platform inputs. If
you want the maintained product path for a real Spring Boot app, see the next
section.

## When You Want a Runnable Example

The examples in this repo teach the conceptual model with fixed inputs and
explain scripts. `cub-gen` is the current runnable product path that turns the
same Spring ownership model into repo-side provenance, governed route checks,
and connected ConfigHub flows.

If you want to run `cub-gen` against a real Spring Boot application with 
actual Java source code, config governance, and a local demo you can execute 
end-to-end without a cluster:

**[`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas)** has:

- A real (minimal) Spring Boot app: `InventoryApplication.java`, a REST
  controller, a service class, `pom.xml` with Spring Boot 3.3.2 / Java 21
- Spring config files: `application.yaml`, `application-dev.yaml`,
  `application-prod.yaml` with profile overrides
- Platform policies, a FrameworkRegistry, and Flux/Argo transport configs
- `demo-local.sh` for the repo-first local lifecycle
- `demo-governed-routes.sh` for the app-owned `ALLOW` versus platform-owned
  `BLOCKED` route proof
- `demo-embedded-config-mutation.sh` for direct embedded `application.yaml`
  mutation inside the ConfigHub payload
- `demo-connected.sh` for the deeper connected ConfigHub walkthrough
- standalone live-cluster proof with `verify-e2e.sh`

You can run it right now:

```bash
git clone https://github.com/confighub/cub-gen.git
cd cub-gen
go build -o ./cub-gen ./cmd/cub-gen
./examples/springboot-paas/demo-local.sh
./examples/springboot-paas/demo-governed-routes.sh
./examples/springboot-paas/demo-embedded-config-mutation.sh

# Connected path when you want ConfigHub-backed evidence
cub auth login
./examples/springboot-paas/demo-connected.sh
```

If you have studied the model here and want to move to real tooling, that is
the bridge.

## What This Repo Is For

**Use this repo to study the Generator model.** App teams keep writing normal
Spring Boot apps in Git. Platform tooling maps those app inputs into the
operational artifacts needed to run on Kubernetes with GitOps.

That operational layer includes ConfigHub units and spaces, Kubernetes
manifests, platform policy, and GitOps-facing state.

## The Model

App teams write Spring Boot applications. They use `application.yaml`, profiles,
and the normal Spring config surface. That stays the authoring experience.

Platform tooling then maps those inputs into:

- ConfigHub units and spaces
- Kubernetes manifests (ConfigMap, Deployment, Service)
- Platform policy (security, datasource boundaries)
- GitOps-facing operational state

This mapping from app to platform is a deterministic Generator. It is
intentionally constrained:

| Property | Why it matters |
|----------|----------------|
| Invertible mapping | Every operational field traces back to exactly one source field |
| Field provenance | You can explain why any field has its value |
| Ownership boundaries | Fields are app-owned or platform-owned, not ambiguous |
| Mutation from provenance | How a field can change depends on who owns it |

The point is not "generate all the YAML." The point is to generate only the
operational config that the platform must own, while keeping the path back to
app inputs clear.

## Mutation Routes

Every operational field change falls into one of three categories:

| Route | Owner | What happens |
|-------|-------|--------------|
| **Apply here** | App team | Teaching path here: mutate with ConfigHub `set-env`; product path: `cub-gen springboot set-embedded-config` |
| **Lift upstream** | App team | Requires source change, route back to Git; bundle exists, automated PR creation is not implemented here |
| **Block/escalate** | Platform | Boundary is documented; server-side block/escalate enforcement is not implemented here |

These routes are derived from field provenance and ownership, not assigned arbitrarily. If you know where a field comes from, you know how it can change.

That is the Config as Data lesson: make the Generator explicit, then treat its
inputs, outputs, field origins, owners, and edit routes as data.

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

That `set-env` command is the teaching shortcut in this repo. In the maintained
`cub-gen` Spring path, the equivalent apply-here proof is the direct embedded
payload mutation:

```bash
cub-gen springboot set-embedded-config \
  --routes ./operational/field-routes.yaml \
  --file ./confighub/inventory-api-prod.yaml \
  --configmap inventory-api-config \
  feature.inventory.reservationMode optimistic
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

If you already understand the model, use `cub-gen springboot init` and the
current Spring ownership helpers:

```bash
cub-gen springboot init --dry-run ./path/to/your-spring-app
cub-gen springboot init --app my-service ./path/to/your-spring-app

cub-gen springboot validate-mutation --routes ./operational/field-routes.yaml \
  feature.myservice.someFlag

cub-gen springboot set-embedded-config \
  --routes ./operational/field-routes.yaml \
  --file ./confighub/my-service-prod.yaml \
  --configmap my-service-config \
  feature.myservice.someFlag optimistic
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
| Direct embedded apply-here helper | Productized in `cub-gen`, not this repo |
| Lift-upstream automated PR | Bundle only, no PR |
| Block/escalate enforcement | Documented, not enforced |

## From This Repo to the Real Product Path

`spring-platform` teaches the model with fixed inputs and explain scripts. It shows why each field is mutable, lifted upstream, or blocked.

`cub-gen/examples/springboot-paas` is the product-side path with real
detection, real generation, route validation, direct embedded payload mutation,
and connected evidence flow.

| Here | There |
|------|-------|
| Fixed inventory-api example | Your actual app |
| Hardcoded field explanations | Computed from source |
| Scaffold for adaptation | `cub-gen springboot init` for onboarding |
| Documented boundaries | `cub-gen springboot validate-mutation` for local/CI route checks |
| Apply-here as `cub function do set-env` | `cub-gen springboot set-embedded-config` for direct payload mutation |
| Conceptual connected story | `./examples/springboot-paas/demo-connected.sh` |
| Model-only runtime story | `./examples/springboot-paas/verify-e2e.sh` |

Full concept mapping: [`FROM-DEMO-TO-PRODUCT.md`](./FROM-DEMO-TO-PRODUCT.md).

## What Teams Ask Next

> "Is `render.sh` the real generator, or a demo prop?"

In `spring-platform`, `render.sh` explains the fixed example. In `cub-gen`, `springboot-paas` has the real generator. That's the right next step for teams evaluating the product path.

## AI Guidance

See [`AI_START_HERE.md`](./AI_START_HERE.md) for paced AI-assisted walkthroughs.
