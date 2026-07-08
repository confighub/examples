import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The app talks browser-direct to the ConfigHub instance named by
// VITE_CONFIGHUB_BASE_URL (auth via @confighub/react-auth's OIDC PKCE flow), so
// no dev proxy is needed. Register this origin's OAuth client first — see
// README.md.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5181,
  },
});
