# AI Start Here

Use the stronger repo-level AI guide first:

- [AI-README-FIRST.md](./AI-README-FIRST.md)

That file explains:

- how to access live ConfigHub through `cub`
- which commands are read-only
- which commands have stable JSON output
- common CLI gotchas
- where the important docs and examples live

Then use this file for the incubator-specific path:

- [`incubator/AI_START_HERE.md`](./incubator/AI_START_HERE.md)

## CRITICAL: Demo Pacing

When walking a human through any example in this repo, you MUST pause after every stage.

After each stage:

1. run only the command or commands for that stage
2. print the full output on screen; do not abbreviate or summarize it away
3. explain what the output means in plain English
4. if there is a GUI URL or GUI checkpoint, print it explicitly
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if no issue exists yet, say that explicitly
8. tell the human to open the GUI and give them time to click through it
9. stop and ask `Ready to continue?` or `Want to inspect this more?`
10. do not proceed until the human tells you to continue

If there is no GUI checkpoint for a stage, say that explicitly instead of skipping the note.

When a stage has GUI relevance, prefer this shape:

- `GUI now:` exact URL or click path and what is visible today
- `GUI gap:` what the GUI cannot show yet
- `GUI feature ask:` what the GUI should show next, with issue number if known
- `PAUSE:` tell the human to open the GUI and inspect it before continuing

## Suggested Prompt For Humans

Use this prompt when you want the AI to behave like a demo guide rather than a fast script runner:

```text
Read the relevant AI_START_HERE.md file and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## Where To Start

If the user is asking for GitHub import with Argo or Flux, start with the published docs and the runnable incubator examples in this repo:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- [`connect-and-compare`](./connect-and-compare/README.md)
- [`import-from-live`](./import-from-live/README.md)
- [`import-from-bundle`](./import-from-bundle/README.md)
- [`incubator/fleet-import`](./incubator/fleet-import/README.md)
- [`incubator/demo-data-adt`](./incubator/demo-data-adt/README.md)
- [`graph-export`](./graph-export/README.md)
- [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md)
- [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md)

Use the first four when the user wants no-cluster evidence or offline import paths before moving into the live Argo and Flux examples.

If the user wants a stable app-style GitOps layout rather than an import flow, start here:

- [`apptique-flux-monorepo`](./apptique-flux-monorepo/README.md)
- [`apptique-argo-applicationset`](./apptique-argo-applicationset/README.md)

Use it when the user wants:

- one app
- one shared base
- dev and prod overlays
- Flux-managed rollout without the extra complexity of the Argo siblings

Use `apptique-argo-applicationset` when the user wants:

- directory-driven environment discovery
- generated Argo Applications per environment
- the clearest stable Argo app-style layout in the repo

If the user is asking about worker extensibility, validation, policy checks, or custom execution paths, use the official worker examples in this repo:

- [`custom-workers/hello-world-bridge`](./custom-workers/hello-world-bridge/README.md)
- [`custom-workers/hello-world-function`](./custom-workers/hello-world-function/README.md)
- [`custom-workers/kube-score`](./custom-workers/kube-score/README.md)
- [`custom-workers/kyverno`](./custom-workers/kyverno/README.md)
- [`custom-workers/kyverno-server`](./custom-workers/kyverno-server/README.md)
- [`custom-workers/opa-gatekeeper`](./custom-workers/opa-gatekeeper/README.md)

Use `cub-scout` as companion material when the local examples here are not enough, especially for Helm-first workflows or microservice app-style comparisons:

- [cub-scout examples index](https://github.com/confighub/cub-scout/tree/main/examples)
- [Helm quickstart](https://github.com/confighub/cub-scout/blob/main/docs/reference/cub-track-quickstart-helm.md)
- [Apptique microservice examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples)
