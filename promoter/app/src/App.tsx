import { Route, Routes } from 'react-router-dom';

import { AuthGate } from './auth/AuthGate';
import { WorkflowsPage } from './pages/WorkflowsPage';
import { WorkflowPage } from './pages/WorkflowPage';
import { StageEditPage } from './pages/StageEditPage';

export default function App() {
  return (
    <AuthGate>
      <Routes>
        <Route path='/' element={<WorkflowsPage />} />
        <Route path='/workflow/:slug' element={<WorkflowPage />} />
        <Route path='/workflow/:slug/edit' element={<StageEditPage />} />
      </Routes>
    </AuthGate>
  );
}
