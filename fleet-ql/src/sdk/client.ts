// The typed ConfigHub API client: openapi-fetch bound to the generated `paths`
// map. This is the one place HTTP is configured (base URL, auth, credentials);
// the transport (api/fqlTransport.ts) calls `cub.GET` / `cub.POST` and gets
// request params + response bodies fully typed against the OpenAPI contract.

import createClient, { type Middleware } from 'openapi-fetch';

import { authHeaders } from '../api/auth';
import type { components, paths } from './confighub.openapi';

/** Entity shapes from the OpenAPI components, for the row-flattening helpers. */
export type Schemas = components['schemas'];

// Base URL. In the browser the app is same-origin with the API, so `/api` (the
// Vite dev proxy / same-origin deployment) is correct. Under Node (scripts/
// live.ts) there is no document base, and openapi-fetch builds a `new Request()`
// which needs an absolute URL — so the harness sets an absolute base on
// globalThis before importing the transport.
const baseUrl =
  (globalThis as { __CUB_API_BASE__?: string }).__CUB_API_BASE__ ?? '/api';

// Attach the bearer token (dev) on every request; same-origin cookie auth works
// without it in production (credentials: 'include').
const auth: Middleware = {
  onRequest({ request }) {
    for (const [k, v] of Object.entries(authHeaders())) request.headers.set(k, v);
    return request;
  },
};

export const cub = createClient<paths>({ baseUrl, credentials: 'include' });
cub.use(auth);
