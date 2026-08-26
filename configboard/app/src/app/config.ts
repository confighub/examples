// Instance and client configuration. Shared with the other example consoles: one
// instance, one organization, read once from the Vite env.
//
// configboard targets one organization on one instance. Cross-org and multi-instance are
// out of scope by design, not pending — there is no tenant switcher to add.

export { BASE_URL, CLIENT_ID, isConfigured } from '@confighub/examples-webkit';
