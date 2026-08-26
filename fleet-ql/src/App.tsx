import { AppShell } from '@confighub/examples-webkit/auth';

import { ExplorerPage } from './pages/ExplorerPage';

export default function App() {
  return (
    <AppShell
      title='fleet-ql'
      tagline='Sign in to query your ConfigHub fleet with SQL.'
    >
      <ExplorerPage />
    </AppShell>
  );
}
