import { AppShell } from '@confighub/examples-webkit/auth';
import { ScopeSettings } from '@confighub/examples-webkit/fleet';
import { Box, Button, Tab, Tabs } from '@mui/material';
import { useState } from 'react';
import { Link, Route, Routes, useLocation } from 'react-router-dom';

import { scopeStore } from './fleet/scope';
import { SnapshotProvider } from './fleet/snapshot';
import { DashboardPage } from './pages/DashboardPage';
import { ExplorerPage } from './pages/ExplorerPage';
import { FindingsPage } from './pages/FindingsPage';
import { FleetPage } from './pages/FleetPage';
import { UnitPage } from './pages/UnitPage';
import { WhoCanPage } from './pages/WhoCanPage';

const NAV = [
  { path: '/', label: 'Dashboard' },
  { path: '/explorer', label: 'Explorer' },
  { path: '/who-can', label: 'Who can' },
  { path: '/findings', label: 'Findings' },
  { path: '/fleet', label: 'Fleet ops' },
];

function NavTabs() {
  const location = useLocation();
  const current = NAV.find((n) => n.path === location.pathname)?.path ?? '/';
  return (
    <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
      <Tabs value={current}>
        {NAV.map((n) => (
          <Tab key={n.path} value={n.path} label={n.label} component={Link} to={n.path} />
        ))}
      </Tabs>
    </Box>
  );
}

export default function App() {
  const [scopeOpen, setScopeOpen] = useState(false);

  return (
    <AppShell
      title='RBAC Manager'
      tagline='Sign in to analyze effective Kubernetes RBAC across your fleet.'
      actions={
        <Button color='inherit' size='small' onClick={() => setScopeOpen(true)}>
          Scope
        </Button>
      }
    >
      <ScopeSettings
        open={scopeOpen}
        store={scopeStore}
        onClose={(changed) => {
          setScopeOpen(false);
          // The snapshot provider reads scope at load time; a reload is the simplest way
          // to rebuild everything against the new scope.
          if (changed) window.location.reload();
        }}
      />
      <SnapshotProvider>
        <NavTabs />
        <Routes>
          <Route path='/' element={<DashboardPage />} />
          <Route path='/explorer' element={<ExplorerPage />} />
          <Route path='/who-can' element={<WhoCanPage />} />
          <Route path='/findings' element={<FindingsPage />} />
          <Route path='/fleet' element={<FleetPage />} />
          <Route path='/unit/:spaceId/:unitId' element={<UnitPage />} />
        </Routes>
      </SnapshotProvider>
    </AppShell>
  );
}
