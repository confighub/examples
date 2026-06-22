# Hosted demo deployment

This directory deploys the promoter app to ConfigHub's own infrastructure
at **https://promoter.test.confighub.net**, running against the production
API at hub.confighub.com. It doubles as a reference for hosting your own UI on
top of the ConfigHub API.

## How it works

The app is a static SPA that only talks to relative `/api` and `/auth` paths.
A small nginx container serves the built bundle and proxies those two paths to
the ConfigHub API, making the deployment same-origin: the standard ConfigHub
session-cookie login works with no OAuth configuration in the app, and no CORS
is required.

```
Browser ── https://promoter.test.confighub.net
              │
              ▼
           nginx ──── /            → SPA bundle (dist/)
                 ──── /api, /auth  → ConfigHub API (Host: hub.confighub.com)
```

Pieces:

- `Dockerfile`, `nginx.conf`, `docker-entrypoint.sh` — multi-stage build
  (Node → nginx:alpine, non-root). `API_BACKEND_URL` is substituted into the
  nginx config at container start; the demo points it at the in-cluster API
  service, but `https://hub.confighub.com` works from anywhere.
- `k8s.yaml` — Service, Deployment, and Traefik IngressRoutes. This is the
  initial data for a long-lived ConfigHub unit (`promoter` in the
  `prod-use2-ui-preview` space) — the deployment is itself managed as
  ConfigHub config, and Argo CD syncs it from the space's OCI bundle.
- `../../.github/workflows/deploy-promoter.yml` — on push to main, builds
  and pushes `ghcr.io/confighub/promoter:<sha>`, then updates the unit's
  image reference with `cub function do set-image-reference`.

The hostname rides the existing `*.test.confighub.net` wildcard DNS, TLS cert,
and Keycloak redirect-URI allowlist used by UI previews, so no DNS, cert, or
IdP changes are needed.

## One-time setup

1. Create a deploy worker in the target space and store its credentials as
   repo secrets:

   ```sh
   cub worker create promoter-deploy --space prod-use2-ui-preview
   cub worker get-secret promoter-deploy --space prod-use2-ui-preview

   gh secret set PROMOTER_DEPLOY_WORKER_ID --repo confighub/examples
   gh secret set PROMOTER_DEPLOY_WORKER_SECRET --repo confighub/examples
   ```

2. After the first workflow run pushes the image, make the
   `ghcr.io/confighub/promoter` package **public** (GitHub → org Packages
   → promoter → Package settings → Change visibility). The manifest
   deliberately has no `imagePullSecrets`; until the package is public the
   pod will sit in ImagePullBackOff, and it recovers on its own once flipped.

The first workflow run creates the `promoter` unit from `k8s.yaml`
automatically (with a `critical` delete gate, since Argo prunes live
resources if the unit is deleted). Subsequent runs only bump the image tag —
the unit, not this file, is the source of truth for any later config edits.

## Hosting your own UI like this

The same pattern works against the public API from any infrastructure: serve
your SPA and proxy `/api` to `https://hub.confighub.com`. The session-cookie
login flow requires your origin to be in ConfigHub's IdP redirect allowlist,
so outside ConfigHub-operated infrastructure use the app's bearer-token mode
instead: get a token with `cub auth get-token` and paste it at the prompt
(see `../app/README.md`).
