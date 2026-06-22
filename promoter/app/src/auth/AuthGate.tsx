import {
  AppBar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Link,
  Paper,
  TextField,
  Toolbar,
  Typography,
} from '@mui/material';
import { ReactNode, useState, useSyncExternalStore } from 'react';

import { useGetMeQuery } from '../sdk/confighubapi.gen';
import {
  AUTH_EXPIRED_EVENT,
  clearStoredToken,
  getStoredToken,
  setStoredToken,
} from '../sdk/confighubapi';

/**
 * Subscribes to auth-expired notifications from the API client so a rejected
 * token re-renders the gate without a manual reload.
 */
function useAuthVersion(): number {
  return useSyncExternalStore(
    (onChange) => {
      window.addEventListener(AUTH_EXPIRED_EVENT, onChange);
      return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onChange);
    },
    // Token presence is the store snapshot: it flips when a 401 clears it
    // or token entry sets it.
    () => (getStoredToken() ? 1 : 0),
  );
}

interface TokenSetupProps {
  onSubmit: (token: string) => void;
}

/** Dev-mode fallback: paste a bearer token from `cub auth get-token`. */
function TokenSetup({ onSubmit }: TokenSetupProps) {
  const [token, setToken] = useState('');
  return (
    <Container maxWidth='sm' sx={{ mt: 10 }}>
      <Paper sx={{ p: 4 }}>
        <Typography variant='h5' gutterBottom>
          Connect to ConfigHub
        </Typography>
        <Typography color='text.secondary' sx={{ mb: 2 }}>
          No session found. Paste an API token — get one with:
        </Typography>
        <Box component='pre' sx={{ bgcolor: 'grey.100', p: 1.5, borderRadius: 1, mb: 2 }}>
          cub auth get-token
        </Box>
        <TextField
          fullWidth
          size='small'
          type='password'
          label='Bearer token'
          value={token}
          onChange={(e) => setToken(e.target.value)}
          sx={{ mb: 2 }}
        />
        <Button
          variant='contained'
          disabled={token.trim() === ''}
          onClick={() => onSubmit(token.trim())}
        >
          Connect
        </Button>
        <Typography variant='body2' color='text.secondary' sx={{ mt: 2 }}>
          The token is kept in sessionStorage only. In a same-origin deployment
          this screen is skipped: the standard{' '}
          <Link href='/auth/login'>ConfigHub session login</Link> is used instead.
        </Typography>
      </Paper>
    </Container>
  );
}

interface AuthGateProps {
  children: ReactNode;
}

/**
 * Blocks rendering until the ConfigHub identity is established (session
 * cookie or pasted bearer token), then renders the app shell with the
 * identity chip.
 */
export function AuthGate({ children }: AuthGateProps) {
  useAuthVersion();
  const { data: me, isLoading, isError, refetch } = useGetMeQuery();

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (isError || !me) {
    return (
      <TokenSetup
        onSubmit={(token) => {
          setStoredToken(token);
          refetch();
        }}
      />
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
            label={me.DisplayName ?? me.ExternalID ?? 'connected'}
            color='secondary'
            size='small'
            sx={{ mr: 2 }}
          />
          {getStoredToken() !== null && (
            <Button
              color='inherit'
              size='small'
              onClick={() => {
                clearStoredToken();
                refetch();
              }}
            >
              Disconnect
            </Button>
          )}
        </Toolbar>
      </AppBar>
      {children}
    </>
  );
}
