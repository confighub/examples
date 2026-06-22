import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Container,
  Divider,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { ComponentVariantPicker } from '../components/ComponentVariantPicker';
import { useCatalog } from '../data/catalog';
import { useStorage, WorkflowEntry } from '../data/storage';
import { ComponentChoice, Stage, Workflow } from '../model/workflow';

export function StageEditPage() {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const storage = useStorage();
  const catalog = useCatalog();

  const [entry, setEntry] = useState<WorkflowEntry | null>(null);
  const [draft, setDraft] = useState<Workflow | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const e = await storage.loadWorkflow(slug);
        if (cancelled) return;
        if (!e) {
          setLoadError('Workflow not found.');
          return;
        }
        setEntry(e);
        setDraft(e.workflow);
      } catch {
        if (!cancelled) setLoadError('Failed to load workflow.');
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug]);

  const update = useCallback((fn: (wf: Workflow) => Workflow) => {
    setDraft((cur) => (cur ? fn(cur) : cur));
  }, []);

  const addStage = () =>
    update((wf) => ({
      ...wf,
      stages: [...wf.stages, { name: `stage-${wf.stages.length + 1}`, components: [] }],
    }));

  const setStage = (i: number, stage: Stage) =>
    update((wf) => ({ ...wf, stages: wf.stages.map((s, j) => (j === i ? stage : s)) }));

  const removeStage = (i: number) =>
    update((wf) => ({ ...wf, stages: wf.stages.filter((_, j) => j !== i) }));

  const moveStage = (i: number, dir: -1 | 1) =>
    update((wf) => {
      const j = i + dir;
      if (j < 0 || j >= wf.stages.length) return wf;
      const stages = [...wf.stages];
      [stages[i], stages[j]] = [stages[j], stages[i]];
      return { ...wf, stages };
    });

  const save = async () => {
    if (!entry || !draft) return;
    setSaving(true);
    setSaveError(null);
    try {
      await storage.saveWorkflow(entry, draft, `Edit workflow ${draft.name}`);
      navigate(`/workflow/${slug}`);
    } catch {
      setSaveError('Save failed. The workflow may have changed elsewhere — reload and retry.');
      setSaving(false);
    }
  };

  if (loadError) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity='error'>{loadError}</Alert>
        <Button sx={{ mt: 2 }} onClick={() => navigate('/')}>
          Back to workflows
        </Button>
      </Container>
    );
  }

  if (!draft) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Container sx={{ mt: 4, mb: 8 }} maxWidth='md'>
      <Stack direction='row' alignItems='center' justifyContent='space-between' sx={{ mb: 2 }}>
        <Typography variant='h4'>Edit “{draft.name}”</Typography>
        <Stack direction='row' spacing={1}>
          <Button onClick={() => navigate(`/workflow/${slug}`)}>Cancel</Button>
          <Button variant='contained' onClick={save} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </Stack>
      </Stack>

      {catalog.error && (
        <Alert severity='warning' sx={{ mb: 2 }}>
          {catalog.error}
        </Alert>
      )}
      {saveError && (
        <Alert severity='error' sx={{ mb: 2 }}>
          {saveError}
        </Alert>
      )}
      <Alert severity='info' sx={{ mb: 3 }}>
        Stages run top to bottom; each stage promotes from the one above it. For each stage, choose
        which variant of each component it deploys.
      </Alert>

      <Stack spacing={2}>
        {draft.stages.map((stage, i) => (
          <StageEditor
            key={i}
            stage={stage}
            index={i}
            total={draft.stages.length}
            catalog={catalog}
            onChange={(s) => setStage(i, s)}
            onRemove={() => removeStage(i)}
            onMove={(dir) => moveStage(i, dir)}
          />
        ))}
      </Stack>

      <Button sx={{ mt: 2 }} variant='outlined' onClick={addStage}>
        Add stage
      </Button>
    </Container>
  );
}

function StageEditor({
  stage,
  index,
  total,
  catalog,
  onChange,
  onRemove,
  onMove,
}: {
  stage: Stage;
  index: number;
  total: number;
  catalog: ReturnType<typeof useCatalog>;
  onChange: (s: Stage) => void;
  onRemove: () => void;
  onMove: (dir: -1 | 1) => void;
}) {
  const setComponent = (i: number, choice: ComponentChoice) =>
    onChange({ ...stage, components: stage.components.map((c, j) => (j === i ? choice : c)) });
  const addComponent = () =>
    onChange({ ...stage, components: [...stage.components, { component: '', variant: '' }] });
  const removeComponent = (i: number) =>
    onChange({ ...stage, components: stage.components.filter((_, j) => j !== i) });

  return (
    <Card variant='outlined'>
      <CardContent>
        <Stack direction='row' alignItems='center' spacing={1} sx={{ mb: 1 }}>
          <Typography variant='subtitle2' color='text.secondary'>
            Stage {index + 1}
          </Typography>
          <Box sx={{ flexGrow: 1 }} />
          <Tooltip title='Move up'>
            <span>
              <IconButton size='small' disabled={index === 0} onClick={() => onMove(-1)}>
                ↑
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title='Move down'>
            <span>
              <IconButton size='small' disabled={index === total - 1} onClick={() => onMove(1)}>
                ↓
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title='Remove stage'>
            <IconButton size='small' color='error' onClick={onRemove}>
              🗑
            </IconButton>
          </Tooltip>
        </Stack>

        <TextField
          size='small'
          label='Stage name'
          value={stage.name}
          onChange={(e) => onChange({ ...stage, name: e.target.value })}
          sx={{ mb: 2 }}
        />

        <Divider sx={{ mb: 2 }} />
        <Typography variant='body2' color='text.secondary' sx={{ mb: 1 }}>
          Components in this stage
        </Typography>

        <Stack spacing={1.5}>
          {stage.components.map((choice, i) => (
            <Stack key={i} direction='row' alignItems='flex-start' spacing={1}>
              <ComponentVariantPicker
                catalog={catalog}
                value={choice}
                onChange={(c) => setComponent(i, c)}
              />
              <Tooltip title='Remove component'>
                <IconButton size='small' color='error' onClick={() => removeComponent(i)}>
                  ✕
                </IconButton>
              </Tooltip>
            </Stack>
          ))}
        </Stack>

        <Button size='small' sx={{ mt: 1 }} onClick={addComponent} disabled={catalog.isLoading}>
          Add component
        </Button>
      </CardContent>
    </Card>
  );
}
