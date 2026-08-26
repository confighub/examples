import { AppShell } from '@confighub/examples-webkit/auth';
import { ScopeSettings } from '@confighub/examples-webkit/fleet';
import { Box, Button, Tab, Tabs } from '@mui/material';
import { useState } from 'react';
import { Link, Route, Routes, useLocation } from 'react-router-dom';

import { scopeStore } from './fleet/scope';
import { SnapshotProvider } from './fleet/snapshot';
import { DashboardPage } from './pages/DashboardPage';
import { FindingsPage } from './pages/FindingsPage';
import { FleetPage } from './pages/FleetPage';
import { UnitPage } from './pages/UnitPage';

const NAV = [
  { path: '/', label: 'Dashboard' },
  { path: '/fleet', label: 'Fleet' },
  { path: '/findings', label: 'Findings' },
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
      title='Sec Scanner'
      tagline='Sign in to review image vulnerabilities across your fleet.'
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
          <Route path='/fleet' element={<FleetPage />} />
          <Route path='/findings' element={<FindingsPage />} />
          <Route path='/unit/:spaceId/:unitId' element={<UnitPage />} />
        </Routes>
      </SnapshotProvider>
    </AppShell>
  );
}
