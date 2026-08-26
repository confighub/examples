import { BASE_URL, CLIENT_ID } from '@confighub/examples-webkit';
import { ConfigHubAuthProvider, getAccessToken } from '@confighub/react-auth';
import { configureConfigHub } from '@confighub/rtk-query';
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Provider } from 'react-redux';
import { BrowserRouter } from 'react-router-dom';

import App from './App';
import { store } from './api/store';

// Point the RTK Query api at this instance and hand it the token source. The token is
// read per request from @confighub/react-auth's non-React accessor, so login state flows
// into every query with no auth slice of our own.
configureConfigHub({ baseUrl: BASE_URL, getToken: getAccessToken });

const theme = createTheme({
  palette: {
    primary: { main: '#1a1f36' },
    secondary: { main: '#635bff' },
  },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigHubAuthProvider baseUrl={BASE_URL} clientId={CLIENT_ID}>
      <Provider store={store}>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </ThemeProvider>
      </Provider>
    </ConfigHubAuthProvider>
  </StrictMode>,
);
