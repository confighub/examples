# promoter app

Static SPA (React + TypeScript + Vite + MUI) over ConfigHub's published API.
All durable state lives in ConfigHub; the browser holds only the session and
UI state.

## Development

The app talks browser-direct to the ConfigHub instance and signs in with
`@confighub/react-auth`'s OIDC PKCE flow, so it needs its own registered OAuth
client for the dev origin (port 5181):

```bash
cub oauthclient create promoter-dev --redirect-uri http://localhost:5181/
```

```bash
npm install
cp .env.example .env.local     # then set VITE_OAUTH_CLIENT_ID to the id above
npm run dev                    # http://localhost:5181
```

`VITE_CONFIGHUB_BASE_URL` (default `https://hub.confighub.com`) selects the
instance. Click **Log in** to run the browser-direct auth flow — no token
paste, no proxy. Clean up the client later with
`cub oauthclient delete promoter-dev`.

For realistic component/variant data, seed an org with the
`../../promotion-demo-data` example first (its Spaces carry the `Component`
and `Variant` labels this app reads).

## Build & checks

```bash
npm run lint         # tsc --noEmit
npm run build        # typecheck + production bundle in dist/
```

## SDK

Data and auth come from the published ConfigHub JS SDK
([confighub/js-sdk](https://github.com/confighub/js-sdk)):

- [`@confighub/rtk-query`](https://www.npmjs.com/package/@confighub/rtk-query) —
  RTK Query hooks (`useListSpacesQuery`, `usePatchUnitMutation`, …). Configured
  once in `src/main.tsx` via `configureConfigHub({ baseUrl, getToken })` and
  mounted in `src/api/store.ts`.
- [`@confighub/react-auth`](https://www.npmjs.com/package/@confighub/react-auth) —
  the `<ConfigHubAuthProvider>` + `useAuth()` OIDC PKCE flow driving
  `src/auth/AuthGate.tsx`; its `getAccessToken()` is the token source for both
  the RTK client and the raw `/data` fetch in `src/api/raw.ts`.

## Layout

- `model/` — the workflow document (`workflow.ts`) and the pluggable
  promotion-status abstraction (`status.ts`).
- `data/` — ConfigHub access: `catalog.ts` (read Spaces by `Component`/
  `Variant`), `storage.ts` (workflow units in the `promoter` Space), and
  `promote.ts` (upstream-link inspection + upgrade).
- `pages/` — `WorkflowsPage` (list), `WorkflowPage` (pipeline view + promote),
  `StageEditPage` (build stages and per-component variant choices).
