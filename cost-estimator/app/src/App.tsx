// Cost Estimator console: reads each workload's cost estimate + budget verdict
// (the cost-estimator.confighub.com/* annotations the estimator wrote) and the
// guardrail ApplyGates from ConfigHub, and shows the fleet's spend and what's
// over budget. All reads go through the generated openapi-fetch client.

import {
  Alert,
  AppBar,
  Box,
  Button,
  CircularProgress,
  Tab,
  Tabs,
  Toolbar,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';

import { TokenGate } from './auth/TokenGate';
import type { CostRow } from './cost/model';
import { loadSnapshot } from './cost/snapshot';
import { DashboardPage } from './pages/DashboardPage';
import { FleetPage } from './pages/FleetPage';
import { UnitDialog } from './pages/UnitPage';

function Console() {
  const [rows, setRows] = useState<CostRow[]>([]);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [err, setErr] = useState('');
  const [tab, setTab] = useState(0);
  const [selected, setSelected] = useState<CostRow | null>(null);

  const load = useCallback(async () => {
    setState('loading');
    try {
      setRows(await loadSnapshot());
      setState('ready');
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setState('error');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <Box>
      <AppBar position="static" color="primary">
        <Toolbar variant="dense">
          <Typography variant="h6" sx={{ flexGrow: 1 }}>
            Cost Estimator
          </Typography>
          <Button color="inherit" size="small" onClick={() => void load()}>
            Refresh
          </Button>
        </Toolbar>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} textColor="inherit" indicatorColor="secondary">
          <Tab label="Dashboard" />
          <Tab label="Fleet" />
        </Tabs>
      </AppBar>

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
    </Box>
  );
}

export default function App() {
  return (
    <TokenGate>
      <Console />
    </TokenGate>
  );
}
