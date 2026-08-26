# sec-scanner app

Static SPA (React + TypeScript + Vite + MUI) over ConfigHub's published API. All
durable state lives in ConfigHub; the browser holds only the session and UI state.

## Development

Register the app once to get an OAuth `client_id`. It is public, not a secret, and it
registers in whatever organization your `cub` is logged into — the app can only sign
users into that organization:

```bash
cub oauthclient create sec-scanner --redirect-uri http://localhost:5182/
cp .env.example .env      # paste the client_id into VITE_OAUTH_CLIENT_ID
```

```bash
npm install
npm run dev               # http://localhost:5182
```

Sign in with the Log in button: auth is the browser-direct OIDC PKCE flow run by
[`@confighub/react-auth`](https://github.com/confighub/js-sdk), so there is no proxy to
stand up and no token to paste. The port is pinned because it has to match the
registered `redirect-uri`.

Seed demo data first: `../setup.sh` (see the example's top-level README).

When you are done experimenting, remove the client: `cub oauthclient delete sec-scanner`.

## Build & checks

```bash
npm run lint         # tsc --noEmit
npm run build        # typecheck + production bundle in dist/
npm test             # this app's own tests
```

## Where the code comes from

There is no vendored SDK here. The typed client, its hooks, and the auth flow are the
published [`@confighub/api`](https://www.npmjs.com/package/@confighub/api),
[`@confighub/rtk-query`](https://www.npmjs.com/package/@confighub/rtk-query), and
[`@confighub/react-auth`](https://www.npmjs.com/package/@confighub/react-auth) packages.

What this app shares with the other example consoles — the auth shell, the fleet scope
and snapshot machinery, and config data access — lives in
[`../../webkit`](../../webkit). What is left in `src/` is this app's own: the pages, the
severity model, and the image-upgrade edits.
