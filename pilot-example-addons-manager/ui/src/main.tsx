import { ConfigHubAuthProvider } from '@confighub/react-auth';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import './index.css';

const baseUrl = (import.meta.env.VITE_CONFIGHUB_BASE_URL ?? 'https://hub.confighub.com').trim();
const clientId = (import.meta.env.VITE_OAUTH_CLIENT_ID ?? '').trim();

const root = createRoot(document.getElementById('root')!);

if (!clientId) {
  root.render(
    <main className="app-shell">
      <section className="panel">
        <p className="eyebrow">ConfigHub operational app</p>
        <h1>OAuth client required</h1>
        <p>
          Run <code>npm run oauth:register</code>, then start the SDK UI with
          <code> VITE_CONFIGHUB_BASE_URL</code> and <code>VITE_OAUTH_CLIENT_ID</code>.
        </p>
      </section>
    </main>,
  );
} else {
  root.render(
    <StrictMode>
      <ConfigHubAuthProvider baseUrl={baseUrl} clientId={clientId}>
        <App />
      </ConfigHubAuthProvider>
    </StrictMode>,
  );
}
