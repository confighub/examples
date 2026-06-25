# Generated API client types

`confighub.openapi.ts` is generated from the ConfigHub **OpenAPI spec** (MIT)
with [`openapi-typescript`](https://openapi-ts.dev):

- Repo: https://github.com/confighub/sdk
- Spec: `core/openapi/openapi.json`
- Generated: 2026-06-24 (ref: main)

It is a **types-only** `paths` map (no runtime). The transport
(`src/api/fqlTransport.ts`) pairs it with [`openapi-fetch`](https://openapi-ts.dev/openapi-fetch/)
— a ~6 KB, zero-dependency typed wrapper over native `fetch` — to get a fully
typed client (`src/sdk/client.ts`) with no React/Redux. We use the OpenAPI spec
rather than the SDK's vendored RTK Query client because that client is built for
a Redux store + React hooks, whereas fleet-ql is a portable engine whose
transport also runs under Node (`scripts/live.ts`).

Do not edit `confighub.openapi.ts` by hand; refresh with `npm run vendor-sdk`
(`scripts/vendor-sdk.sh`), which fetches the spec and regenerates it.
