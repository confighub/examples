import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// In dev, /api and /auth are proxied to a ConfigHub server so the browser sees
// a single origin (the same shape a production nginx proxy would use). Point
// CONFIGHUB_URL at a local server or a hosted instance.
const target = process.env.CONFIGHUB_URL ?? 'https://hub.confighub.com';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5190,
    proxy: {
      '/api': { target, changeOrigin: true, cookieDomainRewrite: 'localhost' },
      '/auth': { target, changeOrigin: true, cookieDomainRewrite: 'localhost' },
    },
  },
});
