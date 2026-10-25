# Hosted demo deployment

This directory deploys the promoter app to ConfigHub's own infrastructure
at **https://promoter.test.confighub.net**, running against the production
API at hub.confighub.com. It doubles as a reference for hosting your own UI on
top of the ConfigHub API.

## How it works

The app is a static SPA that talks **browser-direct** to the ConfigHub API and
signs in with `@confighub/react-auth`'s OIDC PKCE flow. nginx just serves the
built bundle — there is no `/api` proxy — so hosting is pure static file serving.
The instance URL and this app's OAuth client id are baked into the bundle at
build time.

```
Browser ── https://promoter.test.confighub.net   (nginx: static SPA bundle)
   │
   └────────────────────────────────────────────▶ https://hub.confighub.com/api
                                                   (OIDC PKCE login + API, CORS)
```

Pieces:

- `Dockerfile`, `nginx.conf` — multi-stage build (Node → nginx:alpine,
  non-root). `npm run build` bakes in `VITE_CONFIGHUB_BASE_URL` and
  `VITE_OAUTH_CLIENT_ID` (passed as `--build-arg`s); nginx serves the result.
- `k8s.yaml` — Service, Deployment, and Traefik IngressRoutes. This is the
  initial data for a long-lived ConfigHub unit (`promoter` in the
  `prod-use2-ui-preview` space) — the deployment is itself managed as
  ConfigHub config, and Argo CD syncs it from the space's OCI bundle.
- Deploying is a manual step. Build and push the image from the repository
  root, then point the unit at it and publish the space's Release, which Argo
  CD syncs:

  ```sh
  tag=$(git rev-parse HEAD)
  docker build --build-arg VITE_CONFIGHUB_BASE_URL=https://hub.confighub.com \
    --build-arg VITE_OAUTH_CLIENT_ID=<client-id> \
    -t ghcr.io/confighub/promoter:$tag -f promoter/deploy/Dockerfile .
  docker push ghcr.io/confighub/promoter:$tag

  cub unit create promoter promoter/deploy/k8s.yaml --space prod-use2-ui-preview \
    --target prod-use2/prod-use2-oci --delete-gate critical   # first deploy only
  cub function do --space prod-use2-ui-preview --unit promoter \
    set-image-reference promoter ":$tag"
  cub release publish prod-use2-ui-preview
  ```

The hostname rides the existing `*.test.confighub.net` wildcard DNS and TLS
cert used by UI previews, so no DNS or cert changes are needed. Two things do
depend on the ConfigHub instance: the prod origin must have a registered OAuth
client (its redirect URI) and be allowed by the API's **CORS** policy.

## One-time setup

0. Register this deployment's OAuth client. Its id is the
   `VITE_OAUTH_CLIENT_ID` build arg; it is a public PKCE client id:

   ```sh
   cub oauthclient create promoter-prod --redirect-uri https://promoter.test.confighub.net/
   ```

   Also confirm hub.confighub.com's CORS policy allows
   `https://promoter.test.confighub.net`.


1. After the first deploy pushes the image, make the
   `ghcr.io/confighub/promoter` package **public** (GitHub → org Packages
   → promoter → Package settings → Change visibility). The manifest
   deliberately has no `imagePullSecrets`; until the package is public the
   pod will sit in ImagePullBackOff, and it recovers on its own once flipped.

The first deploy creates the `promoter` unit from `k8s.yaml`, with a `critical`
delete gate, since Argo prunes live resources if the unit is deleted.
Later deploys only bump the image tag —
the unit, not this file, is the source of truth for any later config edits.

## Hosting your own UI like this

The same pattern works from any static host — GitHub Pages, S3+CloudFront, a
plain nginx container. Serve the built bundle, register an OAuth client for your
origin (`cub oauthclient create <name> --redirect-uri <origin>`), and make sure
that origin is allowed by the API's CORS policy. No proxy, backend, or
server-side session is involved — the browser holds the minted token in memory
and calls `hub.confighub.com/api` directly.
