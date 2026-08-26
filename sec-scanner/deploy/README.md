# Deployment (reference pattern)

This directory shows how to host the sec-scanner console the same way the
[rbac-manager](../../rbac-manager/deploy/) demo is hosted: nginx serves the
built SPA and nothing else. The app talks browser-direct to the ConfigHub
instance and signs in with OIDC PKCE (`@confighub/react-auth`), so there is no
proxy to configure and no session cookie to rewrite.

```
Browser ── https://sec-scanner.<your-domain>
              │                          │
              │                          └── https://hub.confighub.com  (API + IdP,
              ▼                                                          bearer token)
           nginx ──── /  → SPA bundle (dist/)
```

Unlike rbac-manager, this example ships **no live hosted instance and no CI
workflow** — the files here are a template:

- `Dockerfile`, `nginx.conf` — multi-stage build (Node → nginx:alpine,
  non-root). The build context is the **repository root**, not this example:
  the app depends on `../../webkit` as a `file:` dependency, so that directory
  has to be in the context. `VITE_OAUTH_CLIENT_ID` and
  `VITE_CONFIGHUB_BASE_URL` are build args; an empty client id builds an image
  that renders the app's setup hint instead of a login button.
- `k8s.yaml` — Service, Deployment, and Traefik IngressRoutes. Adjust the
  namespace, hostname, and image before applying.

The console is a static SPA that needs **only the ConfigHub API**. The scan
verdict the UI renders is already stored on each Unit as annotations, so neither
the CVE database (`../cvedb/`) nor the scanner (`../scanner/`) is part of this
deployment — they run wherever you run scans and write the results back into
ConfigHub.

## Build and run locally

```bash
# from the repository root — the context has to include webkit/
cub oauthclient create sec-scanner-local --redirect-uri http://localhost:8080/
docker build \
  --build-arg VITE_OAUTH_CLIENT_ID=<the client id> \
  -f sec-scanner/deploy/Dockerfile -t sec-scanner:dev .
docker run --rm -p 8080:8080 sec-scanner:dev
# open http://localhost:8080 and click Log in
```

## Hosting your own UI like this

The same pattern works from any infrastructure, because nothing about it is
ConfigHub-operated: serve the static bundle anywhere, and register the origin
for your own organization.

```bash
cub oauthclient create my-console --redirect-uri https://console.example.com/
```

Then build with that client id. The redirect URI must match the origin exactly,
so a new origin means a new client (or another `--redirect-uri` on the existing
one).
