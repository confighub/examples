# Deployment (reference pattern)

This directory shows how to host the sec-scanner console the same way the
[rbac-manager](../../rbac-manager/deploy/) demo is hosted: a small nginx
container serves the built SPA and proxies `/api` and `/auth` to the ConfigHub
API, making the deployment **same-origin** so the standard session-cookie login
works with no OAuth config in the app and no CORS.

```
Browser ── https://sec-scanner.<your-domain>
              │
              ▼
           nginx ──── /            → SPA bundle (dist/)
                 ──── /api, /auth  → ConfigHub API (Host: hub.confighub.com)
```

Unlike rbac-manager, this example ships **no live hosted instance and no CI
workflow** — the files here are a template:

- `Dockerfile`, `nginx.conf`, `docker-entrypoint.sh` — multi-stage build
  (Node → nginx:alpine, non-root). `API_BACKEND_URL` is substituted into the
  nginx config at container start; point it at an in-cluster API service or at
  `https://hub.confighub.com`.
- `k8s.yaml` — Service, Deployment, and Traefik IngressRoutes. Adjust the
  namespace, hostname, and image before applying.

The console is a static SPA that needs **only the ConfigHub API**. The scan
verdict the UI renders is already stored on each Unit as annotations, so neither
the CVE database (`../cvedb/`) nor the scanner (`../scanner/`) is part of this
deployment — they run wherever you run scans and write the results back into
ConfigHub.

## Build and run locally

```bash
# from the example root (build context must include app/)
docker build -f deploy/Dockerfile -t sec-scanner:dev .
docker run --rm -p 8080:8080 -e API_BACKEND_URL=https://hub.confighub.com sec-scanner:dev
# open http://localhost:8080 and paste a token from `cub auth get-token`
```

## Hosting your own UI like this

The same pattern works against the public API from any infrastructure: serve the
SPA and proxy `/api` to `https://hub.confighub.com`. The session-cookie login
requires your origin to be in ConfigHub's IdP redirect allowlist; outside
ConfigHub-operated infrastructure use the app's bearer-token mode instead
(`cub auth get-token`, paste at the prompt — see `../app/README.md`).
