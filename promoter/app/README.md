# promoter app

Static SPA (React + TypeScript + Vite + MUI) over ConfigHub's published API.
All durable state lives in ConfigHub; the browser holds only the session and
UI state.

## Development

```bash
npm install
npm run dev          # http://localhost:5181, proxies /api and /auth to ConfigHub
```

The dev proxy targets `https://hub.confighub.com` by default; override with
`CONFIGHUB_URL=http://localhost:9090 npm run dev` for a local server.

Authentication in dev: paste a token from `cub auth get-token` when prompted
(kept in sessionStorage). In a same-origin deployment the standard ConfigHub
session-cookie login is used automatically and no token is needed.

For realistic component/variant data, seed an org with the
`../../promotion-demo-data` example first (its Spaces carry the `Component`
and `Variant` labels this app reads).

## Build & checks

```bash
npm run lint         # tsc --noEmit
npm run build        # typecheck + production bundle in dist/
```

## SDK client

`src/sdk/confighubapi.gen.ts` and `src/sdk/validation.gen.ts` are vendored
from the [ConfigHub SDK](https://github.com/confighub/sdk) (MIT); refresh with
`npm run vendor-sdk`. `src/sdk/confighubapi.ts` is a deliberate fork of the
SDK's base client — see the header comment there and `src/sdk/VENDORED_FROM.md`.

## Layout

- `model/` — the workflow document (`workflow.ts`) and the pluggable
  promotion-status abstraction (`status.ts`).
- `data/` — ConfigHub access: `catalog.ts` (read Spaces by `Component`/
  `Variant`), `storage.ts` (workflow units in the `promoter` Space), and
  `promote.ts` (upstream-link inspection + upgrade).
- `pages/` — `WorkflowsPage` (list), `WorkflowPage` (pipeline view + promote),
  `StageEditPage` (build stages and per-component variant choices).
