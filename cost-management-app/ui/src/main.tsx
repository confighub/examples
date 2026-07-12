import { ConfigHubAuthProvider } from '@confighub/react-auth';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import workflowData from '../../data/operational-workflow.json';
import './index.css';

const baseUrl = (import.meta.env.VITE_CONFIGHUB_BASE_URL ?? 'https://hub.confighub.com').trim();
const clientId = (import.meta.env.VITE_OAUTH_CLIENT_ID ?? '').trim();
const workflow = workflowData as {
  app: { name: string };
  scenario: { jobToBeDone: string };
};

const root = createRoot(document.getElementById('root')!);

if (!clientId) {
  // First screen before any sign-in is possible: say what the app is for,
  // that nothing is live, and the one next step for a person. The npm/env
  // mechanics stay available but demoted to a collapsed detail line.
  root.render(
    <main className="app-shell">
      <section className="panel hero">
        <p className="eyebrow">ConfigHub operational app</p>
        <h1>{workflow.app.name}</h1>
        <p className="purpose">{workflow.scenario.jobToBeDone}</p>
        <p className="status-note">Not yet connected to ConfigHub. Nothing here is live.</p>
        <p className="next-action">
          This app has not been registered yet — ask whoever operates it to run the setup step.
        </p>
        <details className="setup-details">
          <summary>Setup step (for the person operating this app)</summary>
          <p>
            Run <code>npm run oauth:register</code>, then start the app with
            <code> VITE_CONFIGHUB_BASE_URL</code> and <code>VITE_OAUTH_CLIENT_ID</code> set.
          </p>
        </details>
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
