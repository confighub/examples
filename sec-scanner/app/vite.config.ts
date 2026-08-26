import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

// Auth is browser-direct (OIDC PKCE against the instance's IdP), so there is no proxy
// to stand up: the app talks to the ConfigHub API cross-origin with a bearer token.
// The port is pinned because the redirect_uri registered with `cub oauthclient create`
// has to match exactly.
//
// webkit lives outside this app's root, so the dev server has to be told it may
// serve from there — an alias alone does not lift the fs allow-list.
const webkit = fileURLToPath(new URL('../../webkit/src', import.meta.url));

export default defineConfig({
  plugins: [react()],
  server: { port: 5182, strictPort: true, fs: { allow: ['.', webkit] } },
  resolve: {
    // webkit resolves its own bare imports from webkit/node_modules, so without this
    // the bundle gets a second copy of each of these — and a second React context,
    // which is what stops AppShell from seeing the provider mounted in main.tsx.
    // The dev server hides it by pre-bundling deps into one copy; only a production
    // build shows it.
    dedupe: [
      'react',
      'react-dom',
      '@confighub/api',
      '@confighub/react-auth',
      '@confighub/rtk-query',
      '@emotion/react',
      '@emotion/styled',
      '@mui/material',
    ],
    alias: {
      // Resolve the shared kit to its TypeScript source, so editing it hot-reloads here.
      '@confighub/examples-webkit': webkit,
    },
  },
});
