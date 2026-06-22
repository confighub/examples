import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  CircularProgress,
  Container,
  Link as MuiLink,
  Stack,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { StageColumn } from '../components/StageColumn';
import { useCatalog } from '../data/catalog';
import { useStorage, WorkflowEntry } from '../data/storage';
import { statusProvider } from '../model/status';
import { PromotionState } from '../model/workflow';
import { useGetMeQuery } from '../sdk/confighubapi.gen';

export function WorkflowPage() {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const storage = useStorage();
  const catalog = useCatalog();
  const { data: me } = useGetMeQuery();

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

  // Record a status change (manual or promotion outcome) and persist it.
  const writeStatus = useCallback(
    async (
      stageName: string,
      component: string,
      state: PromotionState,
      revision?: number,
    ) => {
      if (!entry || !statusProvider.set) return;
      const next = statusProvider.set(entry.workflow, stageName, component, state, {
        promotedRevision: revision,
        by: me?.DisplayName ?? undefined,
      });
      try {
        await storage.saveWorkflow(entry, next, `Set ${component} status in ${stageName} to ${state}`);
        await load();
      } catch {
        setError('Failed to save status. Reload and retry.');
      }
    },
    [entry, me, storage, load],
  );

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

  return (
    <Container sx={{ mt: 4, mb: 8 }} maxWidth={false}>
      <Breadcrumbs sx={{ mb: 1 }}>
        <MuiLink component={Link} to='/' underline='hover'>
          Workflows
        </MuiLink>
        <Typography color='text.primary'>{wf.name}</Typography>
      </Breadcrumbs>

      <Stack direction='row' alignItems='center' justifyContent='space-between' sx={{ mb: 3 }}>
        <Typography variant='h4'>{wf.name}</Typography>
        <Button variant='outlined' onClick={() => navigate(`/workflow/${slug}/edit`)}>
          Edit stages
        </Button>
      </Stack>

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
        <Box sx={{ display: 'flex', gap: 2, overflowX: 'auto', pb: 2, alignItems: 'flex-start' }}>
          {wf.stages.map((stage, i) => (
            <StageColumn
              key={i}
              stage={stage}
              stageIndex={i}
              prevStage={i > 0 ? wf.stages[i - 1] : undefined}
              workflow={wf}
              catalog={catalog}
              onPromoted={(component, state, revision) =>
                writeStatus(stage.name, component, state, revision)
              }
              onSetStatus={(component, state) => writeStatus(stage.name, component, state)}
            />
          ))}
        </Box>
      )}
    </Container>
  );
}
