import { useEffect, useMemo, useState } from 'react';
import { useAuth, useConfigHub } from '@confighub/react-auth';
import workflowData from '../../data/operational-workflow.json';

type Variant = {
  id: string;
  app: string;
  variant: string;
  space: string;
  unit: string;
  status: string;
  risk: string;
  nextAction: string;
  configHubUrl?: string;
};

type ProofTab = {
  id: string;
  label: string;
  layer: string;
  status: string;
};

type Workflow = {
  app: { name: string; summary: string };
  scenario: { jobToBeDone: string };
  uiTool: { name: string; apiHook: string; authHook: string };
  workflowSteps: string[];
  variants: Variant[];
  proofTabs: ProofTab[];
  stopRules: string[];
};

const workflow = workflowData as Workflow;

export function App() {
  const { status, user, error, login, logout } = useAuth();
  const api = useConfigHub();
  const [selectedId, setSelectedId] = useState(workflow.variants[0]?.id ?? '');
  const [apiStatus, setApiStatus] = useState('not checked');
  const selected = useMemo(
    () => workflow.variants.find(variant => variant.id === selectedId) ?? workflow.variants[0],
    [selectedId],
  );

  useEffect(() => {
    if (status !== 'authenticated') return;
    let active = true;
    api.GET('/me')
      .then(({ data, error }) => {
        if (!active) return;
        setApiStatus(error ? 'ConfigHub /api/me returned an error' : `ConfigHub /api/me OK for org ${data?.OrganizationID ?? user?.organizationId ?? 'current'}`);
      })
      .catch(reason => {
        if (active) setApiStatus(`ConfigHub /api/me failed: ${String(reason?.message ?? reason)}`);
      });
    return () => {
      active = false;
    };
  }, [api, status, user?.organizationId]);

  if (status === 'loading') {
    return <main className="app-shell"><section className="panel">Signing in...</section></main>;
  }

  if (status !== 'authenticated') {
    return (
      <main className="app-shell">
        <section className="panel hero">
          <p className="eyebrow">{workflow.uiTool.name}</p>
          <h1>{workflow.app.name}</h1>
          <p>{workflow.scenario.jobToBeDone}</p>
          <button type="button" onClick={login}>Sign in with ConfigHub</button>
          {error && <pre className="error">{error.message}</pre>}
        </section>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">{workflow.uiTool.name}</p>
          <h1>{workflow.app.name}</h1>
        </div>
        <div className="auth">
          <span>{apiStatus}</span>
          <button type="button" onClick={logout}>Sign out</button>
        </div>
      </header>

      <section className="panel">
        <h2>Workflow</h2>
        <p>{workflow.scenario.jobToBeDone}</p>
        <ol>{workflow.workflowSteps.map(step => <li key={step}>{step}</li>)}</ol>
      </section>

      <section className="grid">
        <aside className="panel">
          <h2>Variants</h2>
          <div className="variant-list">
            {workflow.variants.map(variant => (
              <button
                className={variant.id === selected?.id ? 'variant active' : 'variant'}
                key={variant.id}
                type="button"
                onClick={() => setSelectedId(variant.id)}
              >
                <strong>{variant.app} / {variant.variant}</strong>
                <span>{variant.space} / {variant.unit}</span>
                <span>{variant.status} · risk {variant.risk}</span>
              </button>
            ))}
          </div>
        </aside>

        <section className="panel detail">
          <h2>Scope</h2>
          {selected ? (
            <>
              <dl>
                <dt>Variant</dt><dd>{selected.variant}</dd>
                <dt>Space</dt><dd>{selected.space}</dd>
                <dt>Unit</dt><dd>{selected.unit}</dd>
                <dt>ConfigHub URL</dt><dd>{selected.configHubUrl || 'not bound yet'}</dd>
                <dt>Next action</dt><dd>{selected.nextAction}</dd>
              </dl>
              <div className="actions">
                <button type="button">Preview change</button>
                <button type="button">Prepare approval</button>
                <button type="button" disabled>Run allowed action</button>
              </div>
            </>
          ) : <p>No Variant rows were generated.</p>}
        </section>
      </section>

      <section className="panel proof">
        <h2>Proof Tabs</h2>
        <div className="proof-grid">
          {workflow.proofTabs.map(tab => (
            <article key={tab.id}>
              <strong>{tab.label}</strong>
              <span>{tab.layer} · {tab.status}</span>
            </article>
          ))}
        </div>
      </section>

      <section className="panel">
        <h2>Stop Rules</h2>
        <ul>{workflow.stopRules.map(rule => <li key={rule}>{rule}</li>)}</ul>
      </section>
    </main>
  );
}
