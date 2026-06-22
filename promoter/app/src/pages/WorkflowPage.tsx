import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  Chip,
  CircularProgress,
  Container,
  Link as MuiLink,
  Stack,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { StageColumn, stageState } from '../components/StageColumn';
import { useCatalog } from '../data/catalog';
import { useStorage, WorkflowEntry } from '../data/storage';

/** Live status is re-read from Space labels on this interval. */
const POLL_MS = 5000;

export function WorkflowPage() {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const storage = useStorage();
  const catalog = useCatalog(POLL_MS);

  const [entry, setEntry] = useState<WorkflowEntry | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);

  const load = useCallback(async () => {
    try {
      const e = await storage.loadWorkflow(slug);
      if (!e) {
        setNotFound(true);
        return;
      }
      setEntry(e);
    } catch {
      setError('Failed to load workflow.');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug]);

  useEffect(() => {
    void load();
  }, [load]);

  if (notFound) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity='error'>Workflow not found.</Alert>
        <Button sx={{ mt: 2 }} onClick={() => navigate('/')}>
          Back to workflows
        </Button>
      </Container>
    );
  }

  if (!entry) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  const wf = entry.workflow;
  const readyCount = wf.stages.filter((s) => stageState(s, catalog, wf.statusLabel) === 'succeeded')
    .length;

  return (
    <Container sx={{ mt: 4, mb: 8 }} maxWidth={false}>
      <Breadcrumbs sx={{ mb: 1 }}>
        <MuiLink component={Link} to='/' underline='hover'>
          Workflows
        </MuiLink>
        <Typography color='text.primary'>{wf.name}</Typography>
      </Breadcrumbs>

      <Stack direction='row' alignItems='center' justifyContent='space-between' sx={{ mb: 1 }}>
        <Stack direction='row' alignItems='baseline' spacing={2}>
          <Typography variant='h4'>{wf.name}</Typography>
          {wf.stages.length > 0 && (
            <Typography color='text.secondary'>
              {readyCount}/{wf.stages.length} stages ready
            </Typography>
          )}
        </Stack>
        <Stack direction='row' spacing={1} alignItems='center'>
          <Chip size='small' variant='outlined' label={`auto-refresh ${POLL_MS / 1000}s`} />
          <Button size='small' onClick={() => catalog.refetch()}>
            Refresh
          </Button>
          <Button variant='outlined' onClick={() => navigate(`/workflow/${slug}/edit`)}>
            Edit stages
          </Button>
        </Stack>
      </Stack>

      <Typography variant='body2' color='text.secondary' sx={{ mb: 2 }}>
        Status is read live from each variant Space’s <code>{wf.statusLabel}</code> label. Simulate a
        change: <code>cub space update --patch &lt;variant-space&gt; --label "{wf.statusLabel}=Ready"</code>
      </Typography>

      {error && (
        <Alert severity='error' sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      {catalog.error && (
        <Alert severity='warning' sx={{ mb: 2 }}>
          {catalog.error}
        </Alert>
      )}

      {wf.stages.length === 0 ? (
        <Alert severity='info'>
          This workflow has no stages yet.{' '}
          <MuiLink component={Link} to={`/workflow/${slug}/edit`}>
            Add stages
          </MuiLink>
          .
        </Alert>
      ) : (
        <Box sx={{ display: 'flex', gap: 1, overflowX: 'auto', pb: 2, alignItems: 'stretch' }}>
          {wf.stages.map((stage, i) => (
            <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              {i > 0 && (
                <Typography color='text.disabled' sx={{ fontSize: 28, userSelect: 'none' }}>
                  →
                </Typography>
              )}
              <StageColumn
                stage={stage}
                stageIndex={i}
                prevStage={i > 0 ? wf.stages[i - 1] : undefined}
                statusLabel={wf.statusLabel}
                catalog={catalog}
                onPromoted={() => catalog.refetch()}
              />
            </Box>
          ))}
        </Box>
      )}
    </Container>
  );
}
