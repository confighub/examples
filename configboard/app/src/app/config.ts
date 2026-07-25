/** Instance and client configuration, read once from the Vite env. */

const rawBaseUrl = (import.meta.env.VITE_CONFIGHUB_BASE_URL ?? 'https://hub.confighub.com')
  .trim()
  .replace(/\/+$/, '');

/** Origin of the ConfigHub instance — the UI root that deep links are built from. */
export const BASE_URL = rawBaseUrl.replace(/\/api$/, '');

export const CLIENT_ID = (import.meta.env.VITE_OAUTH_CLIENT_ID ?? '').trim();

/**
 * configboard targets one organization on one instance. Cross-org and multi-instance
 * are out of scope by design, not pending — there is no tenant switcher to add.
 */
export const isConfigured = CLIENT_ID.length > 0;
