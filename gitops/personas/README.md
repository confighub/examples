# GitOps personas

These are the people the `gitops/` examples are written for. Each example
names one persona, and anything that claims to help Argo CD or Flux users
(ConfigHub docs, the workshop, AI assistants such as Pilot) can be checked
against the same descriptions instead of inventing its own.

A persona is a role, not a real person. Each one says what the person already
knows, the repo they bring, the questions they ask first, and what "this
helped me" means, written as checks you can test.

| Persona | Tool | Level | Brings | Example |
|---|---|---|---|---|
| [Argo beginner](./argo-beginner.md) | Argo CD | Beginner | One app in one or two environments, often set up by someone else | [`../argo/beginner-applicationset`](../argo/beginner-applicationset/README.md) |
| [Argo expert](./argo-expert.md) | Argo CD | Expert | A production Argo estate: app-of-apps, ApplicationSets, several clusters | planned |
| [Flux beginner](./flux-beginner.md) | Flux | Beginner | One app with a `clusters/`, `infrastructure/`, `apps/` layout | [`../flux/beginner`](../flux/beginner/README.md) |
| [Flux expert](./flux-expert.md) | Flux | Expert | A Flux fleet repo: several clusters, teams and promotion | planned |

## Words that mean different things

The examples use Argo CD, Flux and Kustomize terms. ConfigHub and ConfigHub
Workshop use some of the same words for other things, so be explicit when
both meet:

| Word | In these examples | In ConfigHub Workshop |
|---|---|---|
| app | an Argo CD `Application`, or a folder of manifests under `apps/` | `cub app`: a workload and what it needs from a platform |
| base | a Kustomize base that overlays patch | a base variant: one named, reviewed starting configuration |
| config | any manifests or settings | `cub config`: one rendered configuration you check or diff; it becomes a ConfigHub Unit only after upload |
| stack | loosely, a set of apps deployed together | `cub stack`: a certified composition of components |
| fleet | loosely, many clusters | `cub fleet`: governed placement across many targets |
| catalog | not used; the example list is called an index | the catalog of known-good configurations |

See Workshop's [model and vocabulary](https://confighub.github.io/helm-expt/site/d/docs/user/model-and-vocabulary.html).

## How to use a persona

- **Writing or reviewing an example:** the README's "Who this is for" section
  should match the persona, and the example should make the persona's first
  questions answerable.
- **Testing a tool or assistant:** play the persona against the example repo,
  ask the first questions, and score each item under "What helped means".
  An item passes only if the answer is correct for the example, not just
  plausible.
- **Adding a persona:** copy the shape of an existing file. Keep it general
  enough that many real teams recognise themselves in it, and never base it
  on one identifiable person or company.

Planned but not yet written: a platform-team persona that owns a whole fleet.
