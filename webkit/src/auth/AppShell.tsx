// The shell every example app renders inside: it blocks until a ConfigHub identity
// exists, then puts the app's own chrome under a bar carrying the identity and sign-out.
//
// Auth is the browser-direct OIDC PKCE flow run by @confighub/react-auth: login()
// redirects to the IdP, the minted token is held in memory, and every request reads it
// through getToken(). There is no proxy, no cookie, and no token to paste — an app is
// reachable at any origin its client_id was registered for.

import {
  Alert,
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
import { useAuth } from '@confighub/react-auth';
import type { ReactNode } from 'react';

import { CLIENT_ID, isConfigured } from '../config';
import { explainAuthError } from './errors';

export interface AppShellProps {
  /** App name, shown in the bar and on the sign-in card. */
  title: string;
  /** One line on the sign-in card saying what the app does. */
  tagline?: string;
  /** Toolbar controls to the left of the identity chip (Scope, Refresh, …). */
  actions?: ReactNode;
  children: ReactNode;
}

/** The app name as `cub oauthclient create` would take it. */
const slug = (title: string): string => title.toLowerCase().replace(/\s+/g, '-');

/**
 * An auth failure, explained where it can be. The raw message stays visible underneath —
 * the explanation is a reading of it, not a replacement for it.
 */
function AuthError({ message, appName }: { message: string; appName: string }) {
  const explanation = explainAuthError(message, appName);
  return (
    <Alert severity='error' variant='outlined' sx={{ mt: 3, textAlign: 'left' }}>
      {explanation && (
        <Typography variant='body2' gutterBottom>
          {explanation}
        </Typography>
      )}
      <Typography variant='caption' color='text.secondary' sx={{ whiteSpace: 'pre-wrap' }}>
        {message}
      </Typography>
    </Alert>
  );
}

function Centered({ children }: { children: ReactNode }) {
  return (
    <Container maxWidth='sm' sx={{ mt: 10 }}>
      <Paper sx={{ p: 4 }}>{children}</Paper>
    </Container>
  );
}

export function AppShell({ title, tagline, actions, children }: AppShellProps) {
  const { status, user, error, login, logout } = useAuth();

  // An app with no client_id cannot start the flow at all, and the reason is not
  // something the IdP will report — say it here rather than failing at the redirect.
  if (!isConfigured) {
    return (
      <Centered>
        <Typography variant='h5' gutterBottom>
          {title} is not configured
        </Typography>
        <Typography color='text.secondary' sx={{ mb: 2 }}>
          Register this app to get an OAuth client_id (it is public, not a secret), then put
          it in <code>.env</code> as <code>VITE_OAUTH_CLIENT_ID</code>:
        </Typography>
        <Box
          component='pre'
          sx={{ bgcolor: 'grey.100', p: 1.5, borderRadius: 1, mb: 2, whiteSpace: 'pre-wrap' }}
        >
          {`cub oauthclient create ${slug(title)} \\
  --redirect-uri ${window.location.origin}/`}
        </Box>
        <Typography variant='body2' color='text.secondary'>
          The client registers in whatever organization your <code>cub</code> is logged into,
          and the app can only sign users into that organization.
        </Typography>
      </Centered>
    );
  }

  if (status === 'loading') {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (status !== 'authenticated' || !user) {
    return (
      <Centered>
        <Typography variant='h5' gutterBottom>
          Connect to ConfigHub
        </Typography>
        <Typography color='text.secondary' sx={{ mb: 3 }}>
          {tagline ?? `Sign in with your ConfigHub account to use ${title}.`}
        </Typography>
        <Button variant='contained' onClick={login}>
          Log in
        </Button>
        {error && <AuthError message={error.message} appName={slug(title)} />}
        <Typography variant='caption' color='text.secondary' sx={{ display: 'block', mt: 3 }}>
          Signing in as client <code>{CLIENT_ID}</code>. If you belong to more than one
          organization, pick the one this app is registered in.
        </Typography>
      </Centered>
    );
  }

  return (
    <>
      <AppBar position='static'>
        <Toolbar variant='dense'>
          <Typography variant='h6' sx={{ flexGrow: 1 }}>
            {title}
          </Typography>
          {actions}
          <Chip label={`org ${user.organizationId}`} color='secondary' size='small' sx={{ mx: 2 }} />
          <Button color='inherit' size='small' onClick={logout}>
            Sign out
          </Button>
        </Toolbar>
      </AppBar>
      {children}
    </>
  );
}
