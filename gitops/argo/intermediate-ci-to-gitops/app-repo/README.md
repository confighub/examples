# app-repo

This folder models the application repository: the one a product team owns
and pushes to. It is not the repository Argo CD watches; `../gitops-repo/`
is. The only thing this repo does toward delivery is build an image and open
a pull request against the GitOps repo. It never applies anything to a
cluster itself.

- `Dockerfile`, `src/` — the app. A static page standing in for `apptique`'s
  frontend, the same fictional app the other `gitops/` examples use.
- `.github/workflows/build-and-open-gitops-pr.yaml` — illustrative only. See
  the warning banner at the top of that file. This example never runs it.

See [`../README.md`](../README.md) for what this example shows and how to
run its read-only and ConfigHub-upload paths.
