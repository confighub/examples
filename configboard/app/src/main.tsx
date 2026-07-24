import { ConfigHubAuthProvider, getAccessToken } from '@confighub/react-auth';
import { configureConfigHub } from '@confighub/rtk-query';
import CssBaseline from '@mui/material/CssBaseline';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Provider } from 'react-redux';

import { App } from './app/App';
import { BASE_URL, CLIENT_ID } from './app/config';
import { store } from './app/store';
import { ThemeProvider } from './app/theme';

// The only contract between the auth layer and the data client is getToken().
configureConfigHub({ baseUrl: BASE_URL, getToken: getAccessToken });

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigHubAuthProvider baseUrl={BASE_URL} clientId={CLIENT_ID}>
      <Provider store={store}>
        <ThemeProvider>
          <CssBaseline />
          <App />
        </ThemeProvider>
      </Provider>
    </ConfigHubAuthProvider>
  </StrictMode>,
);
