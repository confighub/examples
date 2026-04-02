# Spring Platform in ConfigHub

This example is about an app platform.

App teams keep writing normal Spring Boot apps in Git. Platform tooling then maps those app inputs into the operational artifacts needed to run the service on Kubernetes with GitOps.

That mapping is intentionally constrained. The model shown here uses a safe form of config generation:

- 1:1 and invertible
- provenance for every operational field
- explicit ownership boundaries
- three mutation models derived from provenance:
  - apply here
  - lift upstream
  - block/escalate

The point is not "generate all the YAML." The point is to generate only the operational config that the platform must own, while keeping the path back to app inputs clear.

In this repo, the app is `inventory-api`, a Spring Boot service deployed across `dev`, `stage`, and `prod`. The platform layer maps that app into the things needed to run it:
- ConfigHub units and spaces
- Kubernetes manifests
- platform policy
- GitOps-facing operational state

This repo shows that same model in three teaching views:

| View | Example | What it helps you see |
|------|---------|------------------------|
| Vanilla ConfigHub | [`springboot-platform-app`](./springboot-platform-app/) | How app inputs and platform policy become operational outputs |
| ADT | [`springboot-platform-app-centric`](./springboot-platform-app-centric/) | How one app becomes deployments and targets |
| ADTP | [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) | How platform ownership applies across multiple apps |

These are three lenses on the same app-platform model, not three different products.

## Start Here

- If you want to understand the model, start in this repo.
- If you want to try the real product-side path on your own Spring Boot app, go to [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) and use `cub-gen springboot init`.

## The Three Mutation Stories

A major product launch is in 24 hours. Three requests land at the same time:

1. **Flip a feature flag in prod** — safe, urgent, do it now
2. **Add Redis caching** — valuable, but it requires a code change
3. **Point staging at a different database** — dangerous, must be refused

These map to the three mutation routes every platform team needs:

| Request | Route | Why |
|---------|-------|-----|
| Enable optimistic reservation mode | **Apply here** | App-owned operational tuning |
| Add Redis-backed caching | **Lift upstream** | Requires source changes |
| Change the staging datasource | **Block/escalate** | Crosses a platform-owned boundary |

The rest of this directory shows how those three stories look in Vanilla ConfigHub, ADT, and ADTP.

## Phase 1: How Config Gets Generated

See how `application.yaml` + platform policy becomes the `ConfigMap`, `Deployment`, and `Service` for this app — and why some fields are mutable while others route upstream or stop at the platform boundary.

```bash
cd springboot-platform-app
./setup.sh --explain                                          # what this creates
./generator/render.sh --trace                                 # field-by-field: input → output
./generator/render.sh --explain-field feature.inventory.reservationMode  # MUTABLE: app-owned
./generator/render.sh --explain-field spring.datasource.url              # BLOCKED: platform policy
```

Then create ConfigHub objects and handle request #1 (the feature flag):

```bash
./confighub-setup.sh                                          # create dev/stage/prod spaces
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "release-day: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
./confighub-compare.sh                                        # see the * on prod
./confighub-refresh-preview.sh prod                           # PRESERVE: your change survives
```

Handle request #2 (Redis caching — needs to go back to source):

```bash
./lift-upstream.sh --explain                                  # why this routes upstream
./lift-upstream.sh --render-diff                              # the exact patch bundle
```

Handle request #3 (datasource override — must be refused):

```bash
./block-escalate.sh --explain                                 # why this is blocked
./block-escalate.sh --render-attempt                          # what the dry-run looks like
```

## Phase 2: One App Across Environments

See the same challenge through the app-deployment-target lens: one app, three environments, three mutation outcomes.

```bash
cd springboot-platform-app-centric
./setup.sh --explain          # the ADT view: app → deployments → targets
./setup.sh                    # create spaces, units, noop targets
./demo.sh                     # walk through all three mutation outcomes
```

## Phase 3: Platform Governing Multiple Apps

See how the same ownership model applies across `inventory-api` and `catalog-api` on one platform.

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

## Can I Deploy My Own Spring Boot App?

Two paths:

**1. Scaffold from this example** — adapt the fixed example to your app shape:

```bash
cd springboot-platform-app
./bin/scaffold-app my-service --output ../my-service
```

The scaffold handles mechanical renaming. You still need to replace the app code and review field ownership. Full guide: [`BRING-YOUR-OWN-APP.md`](./BRING-YOUR-OWN-APP.md).

**2. Use the real generator** — run `cub-gen springboot init` on your app:

```bash
cub-gen springboot init --dry-run ./path/to/your-spring-app
cub-gen springboot init --app my-service ./path/to/your-spring-app
```

This detects your Spring Boot app and generates starter cub-gen material. See [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas) for the full product-side path.

## From Demo to Product

`spring-platform` is the easiest place to learn the model: fixed Spring inputs, fixed platform policy, and explain scripts that show why each field is mutable, lifted upstream, or blocked.

`cub-gen/examples/springboot-paas` is the product-side path: real generator, real detection, real enforcement.

Full concept mapping and recommended path: [`FROM-DEMO-TO-PRODUCT.md`](./FROM-DEMO-TO-PRODUCT.md).

## What Teams Will Ask Next

If this challenge works, the next question a serious platform engineer will ask:

> "Is `render.sh` the real generator, or a demo prop?"

That question should take you straight to the section above. In `spring-platform`, `render.sh` explains the fixed example. In `cub-gen`, `springboot-paas` is the real generator.

## AI Guidance

See [`AI_START_HERE.md`](./AI_START_HERE.md) for paced AI-assisted walkthroughs of the full challenge.
