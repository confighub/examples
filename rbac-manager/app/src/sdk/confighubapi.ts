// Forked from the ConfigHub SDK's rtkqueryclient/confighubapi.ts (MIT,
// https://github.com/confighub/sdk/tree/main/core/openapi/rtkqueryclient).
// confighubapi.gen.ts imports `confighubApi` from this module by name.
//
// Differences from upstream:
//  - Optional bearer-token mode for local development: if a token is present
//    in sessionStorage (pasted from `cub auth get-token`), it is sent as an
//    Authorization header and 401s notify the app instead of redirecting.
//  - In dev builds, 401 never auto-redirects to /auth/login (the IdP may not
//    allowlist the dev origin's redirect_uri); the app shows token setup.
//  - 403 destinations are app-local routes served by this SPA.
import type { BaseQueryFn } from '@reduxjs/toolkit/query';
import type { FetchArgs, FetchBaseQueryError } from '@reduxjs/toolkit/query';
import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

export const TOKEN_STORAGE_KEY = 'confighub-token';
/** Dispatched on window when a bearer token is rejected (401). */
export const AUTH_EXPIRED_EVENT = 'confighub:auth-expired';

export function getStoredToken(): string | null {
  return window.sessionStorage.getItem(TOKEN_STORAGE_KEY);
}

export function setStoredToken(token: string): void {
  window.sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
}

export function clearStoredToken(): void {
  window.sessionStorage.removeItem(TOKEN_STORAGE_KEY);
}

const baseQuery = fetchBaseQuery({
  baseUrl: '/api',
  credentials: 'include', // session-cookie mode (same-origin deployments)
  isJsonContentType: (headers) => {
    const ct = headers.get('Content-Type') ?? '';
    return ct.includes('json');
  },
  prepareHeaders: (headers, { endpoint }) => {
    const token = getStoredToken();
    if (token) {
      headers.set('Authorization', `Bearer ${token}`);
    }
    // Operations that require merge-patch+json (matches upstream client)
    if (
      endpoint.startsWith('patch') ||
      endpoint.startsWith('bulkPatch') ||
      endpoint.startsWith('bulkCreate') ||
      endpoint.startsWith('patchView')
    ) {
      headers.set('Content-Type', 'application/merge-patch+json');
    }
    return headers;
  },
});

const baseQueryWithReauth: BaseQueryFn<
  string | FetchArgs,
  unknown,
  FetchBaseQueryError
> = async (args, api, extraOptions) => {
  const result = await baseQuery(args, api, extraOptions);

  if (result.error && result.error.status === 401) {
    if (getStoredToken() || import.meta.env.DEV) {
      // Token mode (or dev): let the app prompt for a (new) token.
      window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
      return result;
    }
    // Cookie mode in production: same-origin session login flow.
    const redirectUri = encodeURIComponent(`${window.location.origin}/auth/callback`);
    window.location.href =
      '/auth/login?state=' +
      window.location.pathname +
      (window.location.search ?? '') +
      '&redirect_uri=' +
      redirectUri;
  }

  if (result.error && result.error.status === 403) {
    // Don't redirect if we're already on an error page (prevents infinite loop)
    if (
      window.location.pathname === '/access-denied' ||
      window.location.pathname === '/pending-approval'
    ) {
      return result;
    }

    const errorData = result.error.data as { message?: string } | undefined;
    const errorMessage = errorData?.message || '';
    const isPendingApproval = errorMessage.includes('pending approval');

    if (isPendingApproval) {
      const returnTo = encodeURIComponent(`${window.location.origin}/pending-approval`);
      window.location.href = `/auth/logout?return_to=${returnTo}`;
    } else {
      window.location.replace('/access-denied');
    }

    // Return a promise that never resolves to prevent further query processing
    return new Promise(() => {});
  }

  return result;
};

// Empty api service; confighubapi.gen.ts injects all endpoints into it.
export const confighubApi = createApi({
  baseQuery: baseQueryWithReauth,
  endpoints: () => ({}),
});
