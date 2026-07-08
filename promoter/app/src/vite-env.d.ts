/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** ConfigHub instance URL, e.g. https://hub.confighub.com. */
  readonly VITE_CONFIGHUB_BASE_URL: string;
  /** This app's registered OAuth (PKCE) client id for the current origin. */
  readonly VITE_OAUTH_CLIENT_ID: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
