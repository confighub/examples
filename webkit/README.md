# webkit

Shared building blocks for the ConfigHub example web consoles. Everything here was
duplicated across two or more of them before it moved in; nothing here is a wrapper for
its own sake.

It is not published. Each app depends on it as `file:` and aliases it to source, so
editing a file here hot-reloads in whichever app is running.

## What it holds

| Module | What it is |
|---|---|
| `@confighub/examples-webkit` | `BASE_URL`, `CLIENT_ID`, `isConfigured` — instance and client config, read once from the Vite env |
| `.../auth` | `AppShell` — the login gate, the top bar with identity and sign-out, and the "not configured yet" screen; `explainAuthError` |
| `.../api` | the shared typed client, config-data reads and writes, bulk data, base64, and server-side resource extraction |
| `.../fleet` | analysis scope (store + settings dialog), scoped-unit resolution, and the snapshot provider each app builds on |
| `.../rbac` | the RBAC engine: parse Kubernetes RBAC into cluster snapshots, resolve effective access, and find problems |

## What it deliberately does not hold

The **ConfigHub API client itself.** That is
[`@confighub/api`](https://www.npmjs.com/package/@confighub/api) and
[`@confighub/rtk-query`](https://www.npmjs.com/package/@confighub/rtk-query), and the auth
flow is [`@confighub/react-auth`](https://www.npmjs.com/package/@confighub/react-auth) —
all published from [confighub/js-sdk](https://github.com/confighub/js-sdk) against a
version-pegged spec. webkit sits on top of them. When something here looks like it
belongs in the SDK, it belongs in the SDK.

Each app's **Redux store** also stays in the app: it is eight lines of standard RTK
wiring, and an app that grows its own slices should not have to eject from a shared one.

## Using it from an app

```json
"dependencies": {
  "@confighub/api": "^0.1.2",
  "@confighub/react-auth": "^0.1.2",
  "@confighub/examples-webkit": "file:../../webkit"
}
```

```ts
// vite.config.ts — resolve to source so edits here hot-reload
resolve: {
  alias: {
    '@confighub/examples-webkit': fileURLToPath(new URL('../../webkit/src', import.meta.url)),
  },
},
```

```tsx
// main.tsx
import { BASE_URL, CLIENT_ID } from '@confighub/examples-webkit';
import { ConfigHubAuthProvider } from '@confighub/react-auth';

<ConfigHubAuthProvider baseUrl={BASE_URL} clientId={CLIENT_ID}>…</ConfigHubAuthProvider>
```

```tsx
// App.tsx
import { AppShell } from '@confighub/examples-webkit/auth';

<AppShell title='My Console' tagline='What this console is for.'>…</AppShell>
```

An app on RTK Query also wires the token source once, in `main.tsx`:

```ts
import { getAccessToken } from '@confighub/react-auth';
import { configureConfigHub } from '@confighub/rtk-query';

configureConfigHub({ baseUrl: BASE_URL, getToken: getAccessToken });
```

## Outside the browser

A validation harness has no OIDC session. It installs its own client before importing
anything that reads data — see `fleet-ql/scripts/live.ts`:

```ts
import { configureClient } from '@confighub/examples-webkit/api';

configureClient({ baseUrl: BASE, getToken: () => execSync('cub auth get-token').toString().trim() });
```

## Checks

```bash
npm run lint    # tsc --noEmit
npm test        # the RBAC engine suite
```
