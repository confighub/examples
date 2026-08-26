// Cost Estimator console: reads each workload's cost estimate + budget verdict
// (the cost-estimator.confighub.com/* annotations the estimator wrote) and the
// guardrail ApplyGates from ConfigHub, and shows the fleet's spend and what's
// over budget. All reads go through the published typed client.

import { AppShell } from '@confighub/examples-webkit/auth';
import { useAuth } from '@confighub/react-auth';
import { Alert, Box, Button, CircularProgress, Tab, Tabs } from '@mui/material';
import { useCallback, useEffect, useState } from 'react';

import type { CostRow } from './cost/model';
import { loadSnapshot } from './cost/snapshot';
import { DashboardPage } from './pages/DashboardPage';
import { FleetPage } from './pages/FleetPage';
import { UnitDialog } from './pages/UnitPage';

function Console() {
  // The shell below renders its children only once signed in, but this component owns
  // both the shell and the load — so the load has to check for itself. Before there is a
  // session there is nothing to read the fleet with.
  const { status } = useAuth();
  const authenticated = status === 'authenticated';

  const [rows, setRows] = useState<CostRow[]>([]);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [err, setErr] = useState('');
  const [tab, setTab] = useState(0);
  const [selected, setSelected] = useState<CostRow | null>(null);

  const load = useCallback(async () => {
    if (!authenticated) return;
    setState('loading');
    try {
      setRows(await loadSnapshot());
      setState('ready');
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setState('error');
    }
  }, [authenticated]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <AppShell
      title="Cost Estimator"
      tagline="Sign in to see what your fleet costs and what is over budget."
      actions={
        <Button color="inherit" size="small" onClick={() => void load()}>
          Refresh
        </Button>
      }
    >
      <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Tabs value={tab} onChange={(_, v) => setTab(v)}>
          <Tab label="Dashboard" />
          <Tab label="Fleet" />
        </Tabs>
      </Box>

      {state === 'loading' && (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 6 }}>
          <CircularProgress />
        </Box>
      )}
      {state === 'error' && (
        <Alert severity="error" sx={{ m: 3 }}>
          {err}
        </Alert>
      )}
      {state === 'ready' && rows.length === 0 && (
        <Alert severity="info" sx={{ m: 3 }}>
          No cost-estimator workloads found. Seed the demo fleet with <code>./demo-setup.sh</code>.
        </Alert>
      )}
      {state === 'ready' && rows.length > 0 && (
        <>
          {tab === 0 && <DashboardPage rows={rows} />}
          {tab === 1 && <FleetPage rows={rows} onSelect={setSelected} />}
        </>
      )}

      <UnitDialog row={selected} onClose={() => setSelected(null)} />
    </AppShell>
  );
}

export default function App() {
  return <Console />;
}
