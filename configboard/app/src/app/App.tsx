import { explainAuthError } from '@confighub/examples-webkit/auth';
import { useAuth } from '@confighub/react-auth';
import AddChartIcon from '@mui/icons-material/AddChart';
import PushPinIcon from '@mui/icons-material/PushPin';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import DataObjectIcon from '@mui/icons-material/DataObject';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import Alert from '@mui/material/Alert';
import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import Container from '@mui/material/Container';
import IconButton from '@mui/material/IconButton';
import Stack from '@mui/material/Stack';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import Toolbar from '@mui/material/Toolbar';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { useState } from 'react';

import { PanelBuilderDialog } from '../builder/PanelBuilderDialog';
import { ConfirmDialog, DuplicateDialog } from './Dialogs';
import { DashboardView } from './DashboardView';
import { PinnedDimensionsDialog } from './PinnedDimensionsDialog';
import { SourceDialog } from './SourceDialog';
import { BASE_URL, isConfigured } from './config';
import { type DashboardEntry, useDashboards } from './useDashboards';

/**
 * The most common setup failure, by a wide margin: an app can only sign in members of
 * the organization that owns its client_id, and a user who belongs to several orgs can
 * easily authenticate into the wrong one. The raw 403 does not say what to do about it.
 */
function Centered({ children }: { children: React.ReactNode }) {
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh' }}>
      <Box sx={{ textAlign: 'center', maxWidth: 520 }}>{children}</Box>
    </Box>
  );
}

export function App() {
  const { status, login, logout, error } = useAuth();
  // Dashboards load from ConfigHub once authenticated; before that there is no session
  // to read them with, and the bundled documents stand in.
  const store = useDashboards(status === 'authenticated');
  const dashboards = store.entries;

  const [active, setActive] = useState(0);
  // The dashboard the source dialog is editing, pinned when it opens. Deriving it from
  // the active tab at save time let the target drift from the document on screen.
  const [sourceEntry, setSourceEntry] = useState<DashboardEntry | null>(null);
  const [duplicating, setDuplicating] = useState<DashboardEntry | null>(null);
  const [deleting, setDeleting] = useState<DashboardEntry | null>(null);
  const [building, setBuilding] = useState<DashboardEntry | null>(null);
  const [pinning, setPinning] = useState(false);

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
            {explainAuthError(error.message, 'configboard') ? (
              <>
                <Typography variant="body2" gutterBottom>
                  {explainAuthError(error.message, 'configboard')}
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

  const current = dashboards[Math.min(active, Math.max(0, dashboards.length - 1))];

  return (
    <Box sx={{ minHeight: '100vh' }}>
      <AppBar position="static" color="transparent" elevation={0} sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Toolbar variant="dense" sx={{ gap: 2 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
            configboard
          </Typography>
          <Tabs
            value={Math.min(active, Math.max(0, dashboards.length - 1))}
            onChange={(_, v: number) => setActive(v)}
            variant="scrollable"
            scrollButtons="auto"
            sx={{ flex: 1, minHeight: 40 }}
          >
            {dashboards.map((d) => (
              <Tab key={d.dashboard.slug} label={d.dashboard.title} sx={{ minHeight: 40 }} />
            ))}
          </Tabs>
          <Stack direction="row" spacing={0.5} alignItems="center">
            {current && (
              <>
                {current.stored && (
                  <Tooltip title="Add a panel">
                    <IconButton size="small" onClick={() => setBuilding(current)}>
                      <AddChartIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                )}
                <Tooltip title="View and edit the dashboard document">
                  <IconButton size="small" onClick={() => setSourceEntry(current)}>
                    <DataObjectIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Duplicate this dashboard">
                  <IconButton size="small" onClick={() => setDuplicating(current)}>
                    <ContentCopyIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                {current.stored && (
                  <Tooltip title="Delete this dashboard">
                    <IconButton size="small" onClick={() => setDeleting(current)}>
                      <DeleteOutlineIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                )}
              </>
            )}
            <Tooltip title="Pinned dimensions — record config values into Unit.Values">
              <IconButton size="small" onClick={() => setPinning(true)}>
                <PushPinIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Button size="small" onClick={logout}>
              Log out
            </Button>
          </Stack>
        </Toolbar>
      </AppBar>

      <Container maxWidth="xl" sx={{ py: 3 }}>
        {store.error && (
          <Alert severity="error" variant="outlined" sx={{ mb: 2 }}>
            {store.error}
          </Alert>
        )}

        {store.isEmpty && (
          <Alert
            severity="info"
            variant="outlined"
            sx={{ mb: 2 }}
            action={
              <Button size="small" onClick={() => void store.seedBundled()} disabled={store.isLoading}>
                Save to ConfigHub
              </Button>
            }
          >
            These four dashboards are bundled with the app. Saving them writes one
            <code> AppConfig/YAML </code> unit each to the <code>configboard</code> Space, after
            which they are editable, versioned config like anything else.
          </Alert>
        )}

        {store.missingBundled.length > 0 && (
          <Alert
            severity="info"
            variant="outlined"
            sx={{ mb: 2 }}
            action={
              <Button size="small" onClick={() => void store.seedBundled()} disabled={store.isLoading}>
                Add
              </Button>
            }
          >
            {store.missingBundled.length === 1
              ? `A starter dashboard is not saved here yet: ${store.missingBundled[0]}.`
              : `Starter dashboards not saved here yet: ${store.missingBundled.join(', ')}.`}{' '}
            Adding them leaves your existing dashboards untouched.
          </Alert>
        )}

        {store.viewsMissing && !store.isEmpty && (
          <Alert
            severity="info"
            variant="outlined"
            sx={{ mb: 2 }}
            action={
              <Button size="small" onClick={() => void store.seedViews()} disabled={store.isLoading}>
                Create Views
              </Button>
            }
          >
            The Version Skew dashboard reads values out of configuration data through saved
            ConfigHub <strong>Views</strong>. Creating them adds two Views to the{' '}
            <code>configboard</code> Space — they work for any resource type, including
            Crossplane and ACK resources, and are reusable from{' '}
            <code>cub unit list --view</code>.
          </Alert>
        )}

        {store.isLoading && dashboards.length === 0 ? (
          <Centered>
            <CircularProgress size={22} />
          </Centered>
        ) : current ? (
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

      {sourceEntry && (
        <SourceDialog
          // Keyed by the unit being edited: the dialog holds the document text in state,
          // and without a key React would keep one entry's text while the props pointed
          // at another — which is how an edit to one dashboard overwrote a different one.
          key={sourceEntry.stored?.unitId ?? sourceEntry.dashboard.slug}
          entry={sourceEntry}
          open
          onClose={() => setSourceEntry(null)}
          onSave={(yaml) => store.saveSource(sourceEntry, yaml)}
        />
      )}

      {pinning && (
        <PinnedDimensionsDialog spaceSlugs={store.spaceSlugs} onClose={() => setPinning(false)} />
      )}

      {building && (
        <PanelBuilderDialog
          key={building.stored?.unitId ?? building.dashboard.slug}
          dashboard={building.dashboard}
          baseUrl={BASE_URL}
          scope={{}}
          onClose={() => setBuilding(null)}
          onAdd={(panel) => store.addPanel(building, panel)}
        />
      )}

      {duplicating && (
        <DuplicateDialog
          entry={duplicating}
          existingSlugs={dashboards.map((d) => d.dashboard.slug)}
          onClose={() => setDuplicating(null)}
          onConfirm={async (slug, title) => {
            await store.duplicate(duplicating, slug, title);
            setDuplicating(null);
          }}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title={`Delete “${deleting.dashboard.title}”?`}
          body={`This deletes the ${deleting.stored?.slug} unit from the configboard Space. The configuration it charts is untouched.`}
          confirmLabel="Delete"
          onClose={() => setDeleting(null)}
          onConfirm={async () => {
            await store.remove(deleting);
            setDeleting(null);
            setActive(0);
          }}
        />
      )}
    </Box>
  );
}
