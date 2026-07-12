export function readRuntimeConfig() {
  return {
    authMode: process.env.AUTH_MODE || 'browser-oauth',
    configHubBaseUrl: process.env.CONFIGHUB_BASE_URL || '',
    oauthClientId: process.env.OAUTH_CLIENT_ID || '',
    port: Number(process.env.PORT || 5173),
  };
}
