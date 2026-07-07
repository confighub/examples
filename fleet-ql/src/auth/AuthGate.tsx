// Blocks the app until a ConfigHub identity is established — a same-origin
// session cookie, or a bearer token pasted from `cub auth get-token` (dev). On
// success it renders the app shell (top bar + identity chip) around the
// explorer. No RTK / generated SDK: identity is checked with a plain GET /api/me.

import {
  AppBar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Paper,
  TextField,
  Toolbar,
  Typography,
} from '@mui/material';
import { ReactNode, useCallback, useEffect, useState } from 'react';

import {
  clearStoredToken,
  fetchIdentity,
  getStoredToken,
  Identity,
  setStoredToken,
} from '../api/auth';

function TokenSetup({ onSubmit }: { onSubmit: (token: string) => void }) {
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
          onKeyDown={(e) => {
            if (e.key === 'Enter' && token.trim() !== '') onSubmit(token.trim());
          }}
          sx={{ mb: 2 }}
        />
        <Button variant='contained' disabled={token.trim() === ''} onClick={() => onSubmit(token.trim())}>
          Connect
        </Button>
        <Typography variant='body2' color='text.secondary' sx={{ mt: 2 }}>
          The token is kept in sessionStorage only. In a same-origin deployment
          this screen is skipped — the ConfigHub session cookie is used instead.
        </Typography>
      </Paper>
    </Container>
  );
}

export function AuthGate({ children }: { children: ReactNode }) {
  const [state, setState] = useState<'loading' | 'in' | 'out'>('loading');
  const [me, setMe] = useState<Identity | null>(null);

  const check = useCallback(async () => {
    setState('loading');
    const id = await fetchIdentity();
    if (id) {
      setMe(id);
      setState('in');
    } else {
      setState('out');
    }
  }, []);

  useEffect(() => {
    void check();
  }, [check]);

  if (state === 'loading') {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (state === 'out') {
    return (
      <TokenSetup
        onSubmit={(token) => {
          setStoredToken(token);
          void check();
        }}
      />
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <AppBar position='static'>
        <Toolbar variant='dense'>
          <Typography variant='h6' sx={{ flexGrow: 1 }}>
            Fleet-QL
          </Typography>
          <Chip
            label={me?.DisplayName ?? me?.ExternalID ?? 'connected'}
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
                void check();
              }}
            >
              Disconnect
            </Button>
          )}
        </Toolbar>
      </AppBar>
      <Box sx={{ flexGrow: 1, minHeight: 0 }}>{children}</Box>
    </Box>
  );
}
