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

If the user is asking for GitHub import with Argo or Flux, start with the published docs and the runnable incubator examples in this repo:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md)
- [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md)

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
