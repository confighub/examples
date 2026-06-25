// Dev auth: ConfigHub uses a bearer token in this standalone console. On load we
// verify GET /api/me; if it fails, prompt for a token (paste `cub auth get-token`)
// and store it in sessionStorage. In a same-origin deployment the session cookie
// works and this never appears.

import { Alert, Box, Button, CircularProgress, Link, Paper, Stack, TextField, Typography } from '@mui/material';
import { type ReactNode, useCallback, useEffect, useState } from 'react';

import { fetchIdentity, setStoredToken } from '../api/auth';

function Center({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
      {children}
    </Box>
  );
}

export function TokenGate({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<'checking' | 'ok' | 'need'>('checking');
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);

  const check = useCallback(async () => {
    setStatus('checking');
    const id = await fetchIdentity();
    setStatus(id ? 'ok' : 'need');
  }, []);

  useEffect(() => {
    void check();
  }, [check]);

  if (status === 'checking') {
    return (
      <Center>
        <CircularProgress />
      </Center>
    );
  }
  if (status === 'ok') return <>{children}</>;

  const connect = async () => {
    if (!token.trim()) return;
    setBusy(true);
    setStoredToken(token.trim());
    await check();
    setBusy(false);
  };

  return (
    <Center>
      <Paper variant="outlined" sx={{ p: 4, maxWidth: 520 }}>
        <Typography variant="h6" gutterBottom>
          Connect to ConfigHub
        </Typography>
        <Alert severity="info" sx={{ mb: 2 }}>
          Paste a bearer token from <code>cub auth get-token</code>. It is kept in this tab's
          session storage only.
        </Alert>
        <Stack spacing={2}>
          <TextField
            label="Bearer token"
            type="password"
            size="small"
            fullWidth
            value={token}
            onChange={(e) => setToken(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && void connect()}
          />
          <Button variant="contained" onClick={() => void connect()} disabled={busy || !token.trim()}>
            Connect
          </Button>
          <Typography variant="caption" color="text.secondary">
            Seed the demo fleet first with <code>./demo-setup.sh</code>, then explore it here. See{' '}
            <Link href="https://docs.confighub.com" target="_blank" rel="noreferrer">
              docs.confighub.com
            </Link>
            .
          </Typography>
        </Stack>
      </Paper>
    </Center>
  );
}
