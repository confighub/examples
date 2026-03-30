# Spring Platform Examples

Start here because this is where Spring/Kubernetes teams already are: app config, platform policy, generated manifests, and ownership boundaries.

```
App inputs + Platform inputs → Generator → Operational config → ConfigHub → Delivery
```

## Three Views of the Same Model

| Example | What it shows | Start here if... |
|---------|---------------|------------------|
| [`springboot-platform-app`](./springboot-platform-app/) | Generator transformation and field lineage | You want to understand how config gets generated |
| [`springboot-platform-app-centric`](./springboot-platform-app-centric/) | One app across dev/stage/prod | You want to see one app across environments |
| [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) | Platform organizing multiple apps | You manage multiple apps (experimental) |

All three share the same mutation routes:

| Route | When | Example |
|-------|------|---------|
| Apply here | App-owned, safe to mutate locally | `feature.inventory.reservationMode` |
| Lift upstream | Needs source change | `spring.cache.*` |
| Block/escalate | Platform-owned | `spring.datasource.*` |

## What's Implemented

| Capability | Status |
|------------|--------|
| Generator transformation | Real |
| Field lineage / explain-field | Real |
| ConfigHub mutation + history | Real |
| Real Kubernetes delivery | Real (core example) / Noop only (others) |
| Noop target simulation | Real |
| `lift upstream` automated PR | Bundle only, no PR |
| `block/escalate` enforcement | Documented, not enforced |

## Quick Start

Pick an example and run:

```bash
cd springboot-platform-app    # or -app-centric, or -platform-centric
./setup.sh --explain          # see what it does (read-only)
./setup.sh                    # create ConfigHub objects
./verify.sh                   # check consistency
```

## AI Guidance

Each example has an `AI_START_HERE.md` for paced demos. Start with [`AI_START_HERE.md`](./AI_START_HERE.md) for orientation.
