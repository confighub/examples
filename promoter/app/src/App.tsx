import { Container, Typography } from '@mui/material';
import { Route, Routes } from 'react-router-dom';

import { AuthGate } from './auth/AuthGate';
import { WorkflowsPage } from './pages/WorkflowsPage';
import { WorkflowPage } from './pages/WorkflowPage';
import { StageEditPage } from './pages/StageEditPage';

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
            <Routes>
              <Route path='/' element={<WorkflowsPage />} />
              <Route path='/workflow/:slug' element={<WorkflowPage />} />
              <Route path='/workflow/:slug/edit' element={<StageEditPage />} />
            </Routes>
          </AuthGate>
        }
      />
    </Routes>
  );
}
