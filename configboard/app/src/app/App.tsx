import { useAuth } from '@confighub/react-auth';
import Alert from '@mui/material/Alert';
import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import Container from '@mui/material/Container';
import Stack from '@mui/material/Stack';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import { useMemo, useState } from 'react';

import { bundledDashboards } from '../dashboards';
import { DashboardView } from './DashboardView';
import { BASE_URL, isConfigured } from './config';

/**
 * The most common setup failure, by a wide margin: an app can only sign in members of
 * the organization that owns its client_id, and a user who belongs to several orgs can
 * easily authenticate into the wrong one. The raw 403 does not say what to do about it.
 */
function explainAuthError(message: string): string | undefined {
  if (/not a member of the organization that owns this app/i.test(message)) {
    return (
      'You signed in to a different organization than the one that owns this app\'s ' +
      'client id. Sign in with the organization you registered the client in — or ' +
      'register a client in the organization you just used:  cub oauthclient create ' +
      'configboard-dev --redirect-uri <this origin>'
    );
  }
  if (/redirect_uri/i.test(message)) {
    return 'The origin serving this app is not a registered redirect URI for this client id.';
  }
  return undefined;
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh' }}>
      <Box sx={{ textAlign: 'center', maxWidth: 520 }}>{children}</Box>
    </Box>
  );
}

export function App() {
  const { status, login, logout, error } = useAuth();
  const dashboards = useMemo(() => bundledDashboards(), []);
  const [active, setActive] = useState(0);

  if (!isConfigured) {
    return (
      <Centered>
        <Alert severity="info" variant="outlined" sx={{ textAlign: 'left' }}>
          <Typography variant="subtitle2" gutterBottom>
            configboard is not configured yet
          </Typography>
          <Typography variant="body2" gutterBottom>
            Register the app to get a client id, then put it in <code>app/.env</code>:
          </Typography>
          <Box component="pre" sx={{ fontSize: 12, mt: 1 }}>
            {`cub oauthclient create configboard \\\n  --redirect-uri http://localhost:5173/\n\ncp .env.example .env\n# set VITE_OAUTH_CLIENT_ID`}
          </Box>
        </Alert>
      </Centered>
    );
  }

  if (status === 'loading') {
    return (
      <Centered>
        <CircularProgress />
      </Centered>
    );
  }

  if (status !== 'authenticated') {
    return (
      <Centered>
        <Typography variant="h5" sx={{ fontWeight: 600, mb: 1 }}>
          configboard
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          Dashboards over the configuration in {BASE_URL}.
        </Typography>
        {error && (
          <Alert severity="error" variant="outlined" sx={{ mb: 2, textAlign: 'left' }}>
            {explainAuthError(error.message) ? (
              <>
                <Typography variant="body2" gutterBottom>
                  {explainAuthError(error.message)}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {error.message}
                </Typography>
              </>
            ) : (
              error.message
            )}
          </Alert>
        )}
        <Button variant="contained" onClick={() => void login()}>
          Log in
        </Button>
      </Centered>
    );
  }

  const current = dashboards[active];

  return (
    <Box sx={{ minHeight: '100vh' }}>
      <AppBar position="static" color="transparent" elevation={0} sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Toolbar variant="dense" sx={{ gap: 2 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
            configboard
          </Typography>
          <Tabs
            value={active}
            onChange={(_, v: number) => setActive(v)}
            sx={{ flex: 1, minHeight: 40 }}
          >
            {dashboards.map((d) => (
              <Tab key={d.dashboard.slug} label={d.dashboard.title} sx={{ minHeight: 40 }} />
            ))}
          </Tabs>
          <Stack direction="row" spacing={1} alignItems="center">
            <Button size="small" onClick={logout}>
              Log out
            </Button>
          </Stack>
        </Toolbar>
      </AppBar>

      <Container maxWidth="xl" sx={{ py: 3 }}>
        {current ? (
          // Keyed by slug so switching dashboards remounts the view. Without this,
          // React keeps the previous dashboard's scope state — and a variable the new
          // dashboard declares but the old one didn't (a time window, say) stays
          // unset, which silently drops its clause and issues an unbounded query.
          <DashboardView
            key={current.dashboard.slug}
            dashboard={current.dashboard}
            errors={current.errors}
          />
        ) : (
          <Alert severity="error">No dashboards loaded.</Alert>
        )}
      </Container>
    </Box>
  );
}
