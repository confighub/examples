import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useStorage, WorkflowEntry } from '../data/storage';
import { emptyWorkflow } from '../model/workflow';

const SLUG_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function WorkflowsPage() {
  const storage = useStorage();
  const navigate = useNavigate();
  const [entries, setEntries] = useState<WorkflowEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  const reload = useCallback(async () => {
    setError(null);
    try {
      setEntries(await storage.listWorkflows());
    } catch {
      setError('Failed to load workflows. Check your ConfigHub connection.');
    }
    // storage is recreated each render; intentionally run once on mount + on demand.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  return (
    <Container sx={{ mt: 4 }} maxWidth='md'>
      <Stack direction='row' alignItems='center' justifyContent='space-between' sx={{ mb: 3 }}>
        <Box>
          <Typography variant='h4'>Promotion workflows</Typography>
          <Typography color='text.secondary'>
            Sequential pipelines that promote component variants stage by stage.
          </Typography>
        </Box>
        <Button variant='contained' onClick={() => setCreateOpen(true)}>
          New workflow
        </Button>
      </Stack>

      {error && (
        <Alert severity='error' sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {entries === null && !error && (
        <Box sx={{ display: 'flex', justifyContent: 'center', mt: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {entries !== null && entries.length === 0 && (
        <Alert severity='info'>No workflows yet. Create one to get started.</Alert>
      )}

      <Stack spacing={2}>
        {(entries ?? []).map((entry) => (
          <WorkflowCard key={entry.unitId} entry={entry} onOpen={() => navigate(`/workflow/${entry.slug}`)} onDeleted={reload} />
        ))}
      </Stack>

      <CreateWorkflowDialog
        open={createOpen}
        existingSlugs={(entries ?? []).map((e) => e.slug)}
        onClose={() => setCreateOpen(false)}
        onCreate={async (slug, name) => {
          await storage.createWorkflow(slug, emptyWorkflow(name));
          setCreateOpen(false);
          await reload();
          navigate(`/workflow/${slug}/edit`);
        }}
      />
    </Container>
  );
}

function WorkflowCard({
  entry,
  onOpen,
  onDeleted,
}: {
  entry: WorkflowEntry;
  onOpen: () => void;
  onDeleted: () => Promise<void>;
}) {
  const storage = useStorage();
  const [confirm, setConfirm] = useState(false);
  const [busy, setBusy] = useState(false);
  const stageCount = entry.workflow.stages.length;
  const componentCount = new Set(
    entry.workflow.stages.flatMap((s) => s.components.map((c) => c.component)),
  ).size;

  return (
    <Card variant='outlined'>
      <Stack direction='row' alignItems='center'>
        <CardActionArea onClick={onOpen}>
          <CardContent>
            <Typography variant='h6'>{entry.workflow.name}</Typography>
            <Typography variant='body2' color='text.secondary'>
              {stageCount} stage{stageCount === 1 ? '' : 's'} · {componentCount} component
              {componentCount === 1 ? '' : 's'} · <code>{entry.slug}</code>
            </Typography>
          </CardContent>
        </CardActionArea>
        <Tooltip title='Delete workflow'>
          <IconButton sx={{ mr: 1 }} onClick={() => setConfirm(true)} aria-label='delete'>
            🗑
          </IconButton>
        </Tooltip>
      </Stack>

      <Dialog open={confirm} onClose={() => setConfirm(false)}>
        <DialogTitle>Delete “{entry.workflow.name}”?</DialogTitle>
        <DialogContent>
          <Typography color='text.secondary'>
            This deletes the workflow definition unit. Component and variant config is not touched.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirm(false)}>Cancel</Button>
          <Button
            color='error'
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              await storage.deleteWorkflow(entry);
              setConfirm(false);
              setBusy(false);
              await onDeleted();
            }}
          >
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
}

function CreateWorkflowDialog({
  open,
  existingSlugs,
  onClose,
  onCreate,
}: {
  open: boolean;
  existingSlugs: string[];
  onClose: () => void;
  onCreate: (slug: string, name: string) => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const slug = slugify(name);
  const slugValid = SLUG_RE.test(slug);
  const slugTaken = existingSlugs.includes(slug);

  return (
    <Dialog open={open} onClose={onClose} maxWidth='sm' fullWidth>
      <DialogTitle>New workflow</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {error && <Alert severity='error'>{error}</Alert>}
          <TextField
            autoFocus
            size='small'
            label='Workflow name'
            value={name}
            onChange={(e) => setName(e.target.value)}
            helperText={slug ? `Slug: ${slug}${slugTaken ? ' (already exists)' : ''}` : ' '}
            error={(name !== '' && !slugValid) || slugTaken}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant='contained'
          disabled={!slugValid || slugTaken || busy}
          onClick={async () => {
            setBusy(true);
            setError(null);
            try {
              await onCreate(slug, name.trim());
            } catch {
              setError('Create failed — the slug may already exist.');
              setBusy(false);
            }
          }}
        >
          {busy ? 'Creating…' : 'Create'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
