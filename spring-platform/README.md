# Spring Platform Examples

This folder groups the Spring Boot platform examples that were moved out of
`incubator/`.

Use these when you want to explain the Spring app-platform model from different
angles without losing the AI-first demo flow.

## The Sequence

| # | Example | Focus |
|---|---------|-------|
| 1 | [`springboot-platform-app`](./springboot-platform-app/README.md) | Generator and authority story: app + platform -> operational |
| 2 | [`springboot-platform-app-centric`](./springboot-platform-app-centric/README.md) | App-centric story: App -> Deployments -> Targets |
| 3 | [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/README.md) | Platform-centric story: Platform -> Apps -> Deployments -> Targets |

## AI-First Guidance

If you are driving these examples with an AI assistant, start here:

- Folder-level AI entry point: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Canonical pacing standard: [`../incubator/docs/ai-first-demo-standard.md`](../incubator/docs/ai-first-demo-standard.md)
- Longer demo-pacing guide: [`../incubator/standard-ai-demo-pacing.md`](../incubator/standard-ai-demo-pacing.md)

## Example-Specific AI Guides

- [`springboot-platform-app/AI_START_HERE.md`](./springboot-platform-app/AI_START_HERE.md)
- [`springboot-platform-app-centric/AI_START_HERE.md`](./springboot-platform-app-centric/AI_START_HERE.md)
- [`springboot-platform-platform-centric/AI_START_HERE.md`](./springboot-platform-platform-centric/AI_START_HERE.md)

## Which One To Start With

- Start with [`springboot-platform-app-centric`](./springboot-platform-app-centric/README.md) if you want the clearest "one app, three deployments, three mutation outcomes" story.
- Start with [`springboot-platform-app`](./springboot-platform-app/README.md) if you want the generator, provenance, and authority-layer explanation first.
- Start with [`springboot-platform-platform-centric`](./springboot-platform-platform-centric/README.md) if you want to show how multiple apps share one platform contract.
