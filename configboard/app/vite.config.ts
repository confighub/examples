import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

// The port is pinned because the redirect_uri registered with `cub oauthclient create`
// has to match exactly.
//
// webkit lives outside this app's root, so the dev server has to be told it may
// serve from there — an alias alone does not lift the fs allow-list.
const webkit = fileURLToPath(new URL('../../webkit/src', import.meta.url));

export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true, fs: { allow: ['.', webkit] } },
  resolve: {
    alias: {
      // Resolve the shared kit to its TypeScript source, so editing it hot-reloads here.
      '@confighub/examples-webkit': webkit,
    },
  },
});
