# Spring Platform Examples

One underlying model. Three views.

## The Model

All three examples show the same `cub-gen` generator story:

```
App inputs + Platform inputs → Generator → Operational config → ConfigHub → Delivery
```

Every example demonstrates the same mutation routes:

| Route | When | Example |
|-------|------|---------|
| Apply here | Field is app-owned, safe to mutate locally | `feature.inventory.reservationMode` |
| Lift upstream | Change should flow back to app source | `spring.cache.*` (adding Redis) |
| Block/escalate | Field is platform-owned | `spring.datasource.*` |

## The Three Views

| View | Example | Core question answered |
|------|---------|------------------------|
| Plain ConfigHub | [`springboot-platform-app`](./springboot-platform-app/) | How does `cub-gen` transform app + platform into governed operational config? |
| ADT | [`springboot-platform-app-centric`](./springboot-platform-app-centric/) | How do I understand one app across deployments and targets? |
| Experimental ADTP | [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/) | How do I make platform explicit above apps and deployments? |

### Plain ConfigHub / Generator View

**Start here** to understand:

- How app inputs and platform policies become operational Kubernetes config
- How ConfigHub stores and governs that config
- How field lineage determines mutation routes
- The generator transformation in detail

See: [`springboot-platform-app/README.md`](./springboot-platform-app/README.md)

### ADT View (App → Deployments → Targets)

**Start here** to understand:

- One app (`inventory-api`) across three deployments (dev, stage, prod)
- How deployments map to ConfigHub spaces
- How targets control where config delivers
- The three mutation outcomes for any field change

See: [`springboot-platform-app-centric/README.md`](./springboot-platform-app-centric/README.md)

### Experimental ADTP View (Platform → Apps → Deployments → Targets)

**Start here** to understand:

- One platform organizing multiple apps
- What is platform-owned vs app-owned
- How apps inherit platform policies
- Platform-wide discovery commands

Note: ADTP is experimental. The model is sound but tooling is incomplete.

See: [`springboot-platform-platform-centric/README.md`](./springboot-platform-platform-centric/README.md)

## Which View Should I Start With?

| Your question | Start with |
|---------------|------------|
| How does the generator transform inputs into operational config? | Plain ConfigHub |
| Show me one app across dev/stage/prod | ADT |
| How do I organize multiple apps under one platform? | Experimental ADTP |
| What is field lineage and why does it matter? | Plain ConfigHub |
| How do mutations get routed to the right outcome? | Any (same model) |

## What Is Real Today

The core example (`springboot-platform-app`) has the fullest implementation. The other two examples share most capabilities but differ in target modes:

| Capability | Status |
|------------|--------|
| Generator transformation | Real |
| Field lineage / explain-field | Real |
| ConfigHub mutation storage | Real |
| Mutation history / audit trail | Real |
| Refresh preview | Real |
| Real Kubernetes delivery | Real (Kind cluster, core example only) |
| Noop target simulation | Real |
| Running app HTTP verification | Real |

## What Is Not Implemented Yet

| Capability | Status |
|------------|--------|
| `lift upstream` automated PR | Bundle exists, no automated PR |
| `block/escalate` server-side enforcement | Documented, not enforced |
| Flux/Argo delivery path | See `global-app-layer` examples |

## AI-First Guidance

If you are driving these examples with an AI assistant:

- Folder-level AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Canonical pacing standard: [`../incubator/docs/ai-first-demo-standard.md`](../incubator/docs/ai-first-demo-standard.md)

Each example also has its own `AI_START_HERE.md` for example-specific guidance.
