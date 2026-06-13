// Create a new persona Unit by cloning an existing Unit in a chosen Space.
// The clone (upstream/downstream) keeps provenance: the new persona can
// later be diffed against and upgraded from its template.

import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
} from '@mui/material';
import { useMemo, useState } from 'react';

import { useSnapshot } from '../fleet/SnapshotContext';
import { useCreateUnitMutation } from '../sdk/confighubapi.gen';

const SLUG_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

export interface NewPersonaDialogProps {
  open: boolean;
  onClose: (created: boolean) => void;
}

export function NewPersonaDialog({ open, onClose }: NewPersonaDialogProps) {
  const { snapshot } = useSnapshot();
  const [createUnit, createState] = useCreateUnitMutation();
  const [name, setName] = useState('');
  const [spaceId, setSpaceId] = useState('');
  const [templateId, setTemplateId] = useState('');
  const [error, setError] = useState<string | null>(null);

  // Spaces and template units come from the snapshot (in-scope units only).
  const spaces = useMemo(() => {
    const byId = new Map<string, string>();
    for (const eu of snapshot?.units.values() ?? []) {
      const id = eu.Unit?.SpaceID;
      const slug = eu.Space?.Slug;
      if (id !== undefined && slug !== undefined) byId.set(id, slug);
    }
    return [...byId.entries()].sort((a, b) => a[1].localeCompare(b[1]));
  }, [snapshot]);

  const templates = useMemo(
    () =>
      [...(snapshot?.units.values() ?? [])].filter((eu) => eu.Unit?.SpaceID === spaceId),
    [snapshot, spaceId],
  );

  const nameValid = SLUG_RE.test(name);

  const create = async () => {
    setError(null);
    const result = await createUnit({
      spaceId,
      upstreamSpaceId: spaceId,
      upstreamUnitId: templateId,
      unit: {
        Slug: name,
        ToolchainType: 'Kubernetes/YAML',
        Labels: { persona: name },
        LastChangeDescription: `Create persona ${name} from template`,
      },
    });
    if ('error' in result && result.error) {
      setError('Create failed — the slug may already exist in that space.');
      return;
    }
    setName('');
    onClose(true);
  };

  return (
    <Dialog open={open} onClose={() => onClose(false)} maxWidth='sm' fullWidth>
      <DialogTitle>New persona</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {error !== null && <Alert severity='error'>{error}</Alert>}
          <TextField
            autoFocus
            size='small'
            label='Persona name (slug)'
            value={name}
            error={name !== '' && !nameValid}
            helperText='lowercase letters, digits, hyphens'
            onChange={(e) => setName(e.target.value)}
          />
          <FormControl size='small'>
            <InputLabel>Space</InputLabel>
            <Select
              label='Space'
              value={spaceId}
              onChange={(e) => {
                setSpaceId(e.target.value);
                setTemplateId('');
              }}
            >
              {spaces.map(([id, slug]) => (
                <MenuItem key={id} value={id}>
                  {slug}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControl size='small' disabled={spaceId === ''}>
            <InputLabel>Clone from unit</InputLabel>
            <Select
              label='Clone from unit'
              value={templateId}
              onChange={(e) => setTemplateId(e.target.value)}
            >
              {templates.map((eu) => (
                <MenuItem key={eu.Unit?.UnitID} value={eu.Unit?.UnitID ?? ''}>
                  {eu.Unit?.Slug}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <Alert severity='info'>
            Creates the persona as a tracked clone of the template unit, in the same Space. Roll
            it out to clusters from the Fleet ops page.
          </Alert>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose(false)}>Cancel</Button>
        <Button
          variant='contained'
          disabled={!nameValid || spaceId === '' || templateId === '' || createState.isLoading}
          onClick={() => void create()}
        >
          {createState.isLoading ? 'Creating…' : 'Create'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
