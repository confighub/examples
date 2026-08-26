import { AppShell } from '@confighub/examples-webkit/auth';
import { Route, Routes } from 'react-router-dom';

import { StageEditPage } from './pages/StageEditPage';
import { WorkflowPage } from './pages/WorkflowPage';
import { WorkflowsPage } from './pages/WorkflowsPage';

export default function App() {
  return (
    <AppShell
      title='Promoter'
      tagline='Sign in with your ConfigHub account to build and run promotion workflows.'
    >
      <Routes>
        <Route path='/' element={<WorkflowsPage />} />
        <Route path='/workflow/:slug' element={<WorkflowPage />} />
        <Route path='/workflow/:slug/edit' element={<StageEditPage />} />
      </Routes>
    </AppShell>
  );
}
