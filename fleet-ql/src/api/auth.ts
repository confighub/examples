// Minimal auth for the explorer: a bearer token kept in sessionStorage (pasted
// from `cub auth get-token`), sent as Authorization on same-origin /api calls.
// In a same-origin deployment the session cookie works without a token; this is
// the dev fallback. The typed client (sdk/client.ts) attaches these headers to
// every request via middleware.

const TOKEN_KEY = 'confighub-token';

export function getStoredToken(): string | null {
  return window.sessionStorage.getItem(TOKEN_KEY);
}
export function setStoredToken(token: string): void {
  window.sessionStorage.setItem(TOKEN_KEY, token);
}
export function clearStoredToken(): void {
  window.sessionStorage.removeItem(TOKEN_KEY);
}

/** Authorization header (bearer token if present), spread into a fetch init. */
export function authHeaders(extra?: Record<string, string>): Record<string, string> {
  const token = getStoredToken();
  return { ...(extra ?? {}), ...(token ? { Authorization: `Bearer ${token}` } : {}) };
}

export interface Identity {
  DisplayName?: string;
  ExternalID?: string;
}

/** Verify the current credentials by calling GET /api/me. Resolves to the
 *  identity on success, or null if unauthenticated (401/403/network). */
export async function fetchIdentity(): Promise<Identity | null> {
  try {
    const res = await fetch('/api/me', { credentials: 'include', headers: authHeaders() });
    if (!res.ok) return null;
    return (await res.json()) as Identity;
  } catch {
    return null;
  }
}
