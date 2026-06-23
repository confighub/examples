import { Box, Container, Tab, Tabs, Typography } from '@mui/material';
import { Link, Route, Routes, useLocation } from 'react-router-dom';

import { AuthGate } from './auth/AuthGate';
import { SnapshotProvider } from './fleet/SnapshotContext';
import { DashboardPage } from './pages/DashboardPage';
import { FindingsPage } from './pages/FindingsPage';
import { FleetPage } from './pages/FleetPage';
import { UnitPage } from './pages/UnitPage';

/** Terminal pages the API client may navigate to on 403. */
function MessagePage({ title, body }: { title: string; body: string }) {
  return (
    <Container sx={{ mt: 8 }}>
      <Typography variant='h4' gutterBottom>
        {title}
      </Typography>
      <Typography color='text.secondary'>{body}</Typography>
    </Container>
  );
}

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
  return (
    <Routes>
      <Route
        path='/access-denied'
        element={
          <MessagePage
            title='Access denied'
            body='Your account does not have access to this organization.'
          />
        }
      />
      <Route
        path='/pending-approval'
        element={
          <MessagePage
            title='Pending approval'
            body='Your account is awaiting approval. Sign in again once approved.'
          />
        }
      />
      <Route
        path='*'
        element={
          <AuthGate>
            <SnapshotProvider>
              <NavTabs />
              <Routes>
                <Route path='/' element={<DashboardPage />} />
                <Route path='/fleet' element={<FleetPage />} />
                <Route path='/findings' element={<FindingsPage />} />
                <Route path='/unit/:spaceId/:unitId' element={<UnitPage />} />
              </Routes>
            </SnapshotProvider>
          </AuthGate>
        }
      />
    </Routes>
  );
}
