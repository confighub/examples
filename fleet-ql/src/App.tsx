import { AuthGate } from './auth/AuthGate';
import { ExplorerPage } from './pages/ExplorerPage';

export default function App() {
  return (
    <AuthGate>
      <ExplorerPage />
    </AuthGate>
  );
}
