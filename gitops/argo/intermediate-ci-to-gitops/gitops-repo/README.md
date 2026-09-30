# gitops-repo

This folder models the separate GitOps config repository: the one Argo CD
actually watches, and the one both pull requests in this example target. The
app repo (`../app-repo/`) never writes here directly; only its CI's pull
request, and only to `environments/apptique/dev/kustomization.yaml`.

- `applications/`: the Argo CD `Application` objects, one per environment.
  Reference material for what a real Argo CD instance would apply; this
  example does not apply them anywhere.
- `environments/apptique/base/`: the shared Deployment, Service and
  ServiceAccount.
- `environments/apptique/dev/`: Kustomize overlay. CI's pull request changes
  exactly one line here: the image tag.
- `environments/apptique/prod/`: Kustomize overlay, one promotion behind
  dev. `PROMOTED_FROM.md` records what the last promotion pull request
  actually promoted.
- `.github/workflows/promote-dev-to-prod.yaml`: illustrative only. See the
  warning banner at the top of that file. This example never runs it.

See [`../README.md`](../README.md) for what this example shows, the
read-only and ConfigHub-upload paths, and the break-it step.
