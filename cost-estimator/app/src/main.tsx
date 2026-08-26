import { BASE_URL, CLIENT_ID } from '@confighub/examples-webkit';
import { ConfigHubAuthProvider } from '@confighub/react-auth';
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import App from './App';

const theme = createTheme({
  palette: {
    primary: { main: '#1a1f36' },
    secondary: { main: '#0a8754' }, // budget green
  },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigHubAuthProvider baseUrl={BASE_URL} clientId={CLIENT_ID}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <App />
      </ThemeProvider>
    </ConfigHubAuthProvider>
  </StrictMode>,
);
