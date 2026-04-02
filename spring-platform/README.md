# Spring Platform in ConfigHub

A major product launch is in 24 hours. Three requests land at the same time:

1. **Flip a feature flag in prod** — safe, urgent, do it now
2. **Add Redis caching** — valuable, but it requires a code change
3. **Point staging at a different database** — dangerous, must be refused

These map to the three mutation routes every platform team needs:

| Request | Route | Why |
|---------|-------|-----|
| Enable optimistic reservation mode | **Apply here** | App-owned operational tuning — mutate directly in ConfigHub |
| Add Redis-backed caching | **Lift upstream** | Requires new Maven dependency and Spring config — route back to source |
| Change the staging datasource | **Block/escalate** | Platform-owned boundary — the managed datasource must not diverge |

The app is `inventory-api`, a Spring Boot service deployed across `dev`, `stage`, and `prod`.

## Three Views of the Same Challenge

Each example handles the same three requests from a different angle:

| Phase | Example | Core question |
|-------|---------|---------------|
| 1 | [`springboot-platform-app`](./springboot-platform-app/) | Why does this field have this value in production? |
| 2 | [`springboot-platform-app-centric`](./springboot-platform-app-centric/) | For this app, where should this change actually happen? |
| 3 | [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) | What belongs to the platform team versus the app team? |

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

Yes, but today this is a worked example you adapt, not a one-command import flow.

What is real today:

- the Spring Boot app in `springboot-platform-app/upstream/app/`
- the ConfigHub setup and verification path
- the real Kind deployment path in Phase 1
- the mutation model: apply-here, lift-upstream, block/escalate

Use the scaffold command to generate a renamed copy for your app:

```bash
cd springboot-platform-app
./bin/scaffold-app my-service --output ../my-service
```

The scaffold handles mechanical renaming. You still need to:

- replace the stub app code with your actual service
- decide which fields are app-owned versus platform-owned
- review ports, health paths, and environment variables

Full guide: [`BRING-YOUR-OWN-APP.md`](./BRING-YOUR-OWN-APP.md).

## From Demo to Product

`spring-platform` is the easiest place to learn the model: fixed Spring inputs, fixed platform policy, and explain scripts that show why each field is mutable, lifted upstream, or blocked.

If you want the real generator path, go to [`cub-gen/examples/springboot-paas`](https://github.com/confighub/cub-gen/tree/main/examples/springboot-paas).

```bash
cd /path/to/cub-gen
go build -o ./cub-gen ./cmd/cub-gen
./examples/springboot-paas/demo-local.sh

# when you want the connected path
cub auth login
./examples/springboot-paas/demo-connected.sh
```

Use `spring-platform` to understand the model and mutation routes. Use `springboot-paas` to see the real generator path in the product repo.

Full concept mapping and recommended path: [`FROM-DEMO-TO-PRODUCT.md`](./FROM-DEMO-TO-PRODUCT.md).

## What Teams Will Ask Next

If this challenge works, the next question a serious platform engineer will ask:

> "Is `render.sh` the real generator, or a demo prop?"

That question should take you straight to the section above. In `spring-platform`, `render.sh` explains the fixed example. In `cub-gen`, `springboot-paas` is the real generator example.

## AI Guidance

See [`AI_START_HERE.md`](./AI_START_HERE.md) for paced AI-assisted walkthroughs of the full challenge.
