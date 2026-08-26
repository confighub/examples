import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

// The app talks browser-direct to the ConfigHub instance named by
// VITE_CONFIGHUB_BASE_URL (auth via @confighub/react-auth's OIDC PKCE flow), so
// no dev proxy is needed. Register this origin's OAuth client first — see
// README.md.
//
// webkit lives outside this app's root, so the dev server has to be told it may
// serve from there — an alias alone does not lift the fs allow-list.
const webkit = fileURLToPath(new URL('../../webkit/src', import.meta.url));

export default defineConfig({
  plugins: [react()],
  // The port is pinned because the redirect_uri registered with `cub oauthclient create`
  // has to match exactly.
  server: { port: 5181, strictPort: true, fs: { allow: ['.', webkit] } },
  resolve: {
    alias: {
      // Resolve the shared kit to its TypeScript source, so editing it hot-reloads here.
      '@confighub/examples-webkit': webkit,
    },
  },
});
