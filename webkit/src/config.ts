// Instance and client configuration, read once from the Vite env. Every example app
// targets one ConfigHub instance and one organization: the `client_id` an app is
// registered under determines the org it can sign a user into, so there is no tenant
// switcher to add.

const rawBaseUrl = (import.meta.env.VITE_CONFIGHUB_BASE_URL ?? 'https://hub.confighub.com')
  .trim()
  .replace(/\/+$/, '');

/** Origin of the ConfigHub instance — the UI root that deep links are built from. */
export const BASE_URL = rawBaseUrl.replace(/\/api$/, '');

/** This app's registered OAuth client, from `cub oauthclient create`. */
export const CLIENT_ID = (import.meta.env.VITE_OAUTH_CLIENT_ID ?? '').trim();

/** False until the app has been given a client_id; the shell explains how to get one. */
export const isConfigured = CLIENT_ID.length > 0;
