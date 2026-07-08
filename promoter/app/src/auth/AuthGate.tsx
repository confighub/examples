import {
  AppBar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Paper,
  Toolbar,
  Typography,
} from '@mui/material';
import { ReactNode } from 'react';
import { useAuth } from '@confighub/react-auth';

interface AuthGateProps {
  children: ReactNode;
}

/**
 * Blocks rendering until the ConfigHub identity is established. Auth is the
 * browser-direct OIDC PKCE flow run by @confighub/react-auth: `login()` redirects
 * to the IdP, the minted token is held in memory, and every RTK query reads it via
 * getAccessToken(). No proxy, cookie, or token paste.
 */
export function AuthGate({ children }: AuthGateProps) {
  const { status, user, error, login, logout } = useAuth();

  if (status === 'loading') {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (status !== 'authenticated' || !user) {
    return (
      <Container maxWidth='sm' sx={{ mt: 10 }}>
        <Paper sx={{ p: 4 }}>
          <Typography variant='h5' gutterBottom>
            Connect to ConfigHub
          </Typography>
          <Typography color='text.secondary' sx={{ mb: 3 }}>
            Sign in with your ConfigHub account to build and run promotion workflows.
          </Typography>
          <Button variant='contained' onClick={login}>
            Log in
          </Button>
          {error && (
            <Box
              component='pre'
              sx={{ bgcolor: '#fdecea', p: 1.5, borderRadius: 1, mt: 3, whiteSpace: 'pre-wrap' }}
            >
              {error.message}
            </Box>
          )}
        </Paper>
      </Container>
    );
  }

  return (
    <>
      <AppBar position='static'>
        <Toolbar variant='dense'>
          <Typography variant='h6' sx={{ flexGrow: 1 }}>
            Promoter
          </Typography>
          <Chip
            label={`org ${user.organizationId}`}
            color='secondary'
            size='small'
            sx={{ mr: 2 }}
          />
          <Button color='inherit' size='small' onClick={logout}>
            Sign out
          </Button>
        </Toolbar>
      </AppBar>
      {children}
    </>
  );
}
