import { CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Provider } from 'react-redux';
import { BrowserRouter } from 'react-router-dom';
import { ConfigHubAuthProvider, getAccessToken } from '@confighub/react-auth';
import { configureConfigHub } from '@confighub/rtk-query';

import App from './App';
import { store } from './api/store';

const baseUrl = (import.meta.env.VITE_CONFIGHUB_BASE_URL ?? '').trim();
const clientId = (import.meta.env.VITE_OAUTH_CLIENT_ID ?? '').trim();

// Point the RTK Query api at this instance and hand it the token source. The
// token is read per request from @confighub/react-auth's non-React accessor, so
// login state flows into every query with no auth slice of our own.
configureConfigHub({ baseUrl, getToken: getAccessToken });

const theme = createTheme({
  palette: {
    primary: { main: '#1a1f36' },
    secondary: { main: '#635bff' },
  },
});

const root = createRoot(document.getElementById('root')!);

if (!baseUrl || !clientId) {
  // Missing build config: OIDC can't run without the instance URL and this app's
  // registered OAuth client id. Render a setup hint instead of a broken login.
  root.render(
    <StrictMode>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <div style={{ maxWidth: 640, margin: '4rem auto', fontFamily: 'sans-serif' }}>
          <h1>Promoter</h1>
          <pre style={{ background: '#fdecea', padding: '1rem', borderRadius: 8 }}>
            Set VITE_CONFIGHUB_BASE_URL and VITE_OAUTH_CLIENT_ID and restart (see
            app/README.md).
          </pre>
        </div>
      </ThemeProvider>
    </StrictMode>,
  );
} else {
  root.render(
    <StrictMode>
      <Provider store={store}>
        <ConfigHubAuthProvider baseUrl={baseUrl} clientId={clientId}>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <BrowserRouter>
              <App />
            </BrowserRouter>
          </ThemeProvider>
        </ConfigHubAuthProvider>
      </Provider>
    </StrictMode>,
  );
}
