# Hosted demo deployment

This directory deploys the rbac-manager app to ConfigHub's own infrastructure
at **https://rbac-manager.test.confighub.net**, running against the production
API at hub.confighub.com. It doubles as a reference for hosting your own UI on
top of the ConfigHub API.

## How it works

The app is a static SPA that talks browser-direct to the ConfigHub instance and
signs in with OIDC PKCE (`@confighub/react-auth`). nginx serves the bundle and
nothing else — there is no proxy to configure and no session cookie to rewrite.
The instance URL and the app's OAuth client id are baked into the bundle at
image-build time, so the image is specific to the origin it is served from.

```
Browser ── https://rbac-manager.test.confighub.net
              │                          │
              │                          └── https://hub.confighub.com  (API + IdP,
              ▼                                                          bearer token)
           nginx ──── /  → SPA bundle (dist/)
```

Pieces:

- `Dockerfile`, `nginx.conf` — multi-stage build (Node → nginx:alpine,
  non-root). The build context is the **repository root**, not this example:
  the app depends on `../../webkit` as a `file:` dependency, so that directory
  has to be in the context. `VITE_OAUTH_CLIENT_ID` and
  `VITE_CONFIGHUB_BASE_URL` are build args; an empty client id builds an image
  that renders the app's setup hint instead of a login button.
- `k8s.yaml` — Service, Deployment, and Traefik IngressRoutes. This is the
  initial data for a long-lived ConfigHub unit (`rbac-manager` in the
  `prod-use2-ui-preview` space) — the deployment is itself managed as
  ConfigHub config, and Argo CD syncs it from the space's OCI bundle.
- `../../.github/workflows/deploy-rbac-manager.yml` — on push to main, builds
  and pushes `ghcr.io/confighub/rbac-manager:<sha>`, then updates the unit's
  image reference with `cub function do set-image-reference`.

The hostname rides the existing `*.test.confighub.net` wildcard DNS, TLS cert,
and Keycloak redirect-URI allowlist used by UI previews, so no DNS, cert, or
IdP changes are needed.

## One-time setup (already done for this repo)

1. Create a deploy worker in the target space and store its credentials as
   repo secrets:

   ```sh
   cub worker create rbac-manager-deploy --space prod-use2-ui-preview
   cub worker get-secret rbac-manager-deploy --space prod-use2-ui-preview

   gh secret set RBAC_MANAGER_DEPLOY_WORKER_ID --repo confighub/examples
   gh secret set RBAC_MANAGER_DEPLOY_WORKER_SECRET --repo confighub/examples
   ```

2. After the first workflow run pushes the image, make the
   `ghcr.io/confighub/rbac-manager` package **public** (GitHub → org Packages
   → rbac-manager → Package settings → Change visibility). The manifest
   deliberately has no `imagePullSecrets`; until the package is public the
   pod will sit in ImagePullBackOff, and it recovers on its own once flipped.

The first workflow run creates the `rbac-manager` unit from `k8s.yaml`
automatically (with a `critical` delete gate, since Argo prunes live
resources if the unit is deleted). Subsequent runs only bump the image tag —
the unit, not this file, is the source of truth for any later config edits.

## Hosting your own UI like this

The same pattern works from any infrastructure, because nothing about it is
ConfigHub-operated: serve the static bundle anywhere. What makes the origin
work is registering it, which anyone can do for their own organization:

```bash
cub oauthclient create my-console --redirect-uri https://console.example.com/
```

Then build with that client id. The redirect URI must match the origin exactly,
so a new origin means a new client (or another `--redirect-uri` on the existing
one) — not a change to this repository.
