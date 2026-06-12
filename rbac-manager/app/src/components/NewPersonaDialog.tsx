// Create a new persona in the base Space by cloning an existing one. The
// clone (upstream/downstream) keeps provenance: the new persona can later
// be diffed against and upgraded from its template.

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
import { useEffect, useState } from 'react';

import {
  ExtendedUnitRead,
  useCreateUnitMutation,
  useLazyListAllUnitsQuery,
} from '../sdk/confighubapi.gen';

const SLUG_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

export interface NewPersonaDialogProps {
  open: boolean;
  /** The Space (labeled role=base) the persona is created in. */
  baseSpaceId: string;
  onClose: (created: boolean) => void;
}

export function NewPersonaDialog({ open, baseSpaceId, onClose }: NewPersonaDialogProps) {
  const [listUnits, { data: units }] = useLazyListAllUnitsQuery();
  const [createUnit, createState] = useCreateUnitMutation();
  const [name, setName] = useState('');
  const [templateId, setTemplateId] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      void listUnits({ where: "Labels.app = 'rbac-manager'", select: 'UnitID,Slug,SpaceID,Labels' });
    }
  }, [open, listUnits]);

  const templates = (units ?? []).filter(
    (eu: ExtendedUnitRead) =>
      eu.Unit?.SpaceID === baseSpaceId && eu.Unit?.Labels?.persona !== undefined,
  );
  const nameValid = SLUG_RE.test(name);

  const create = async () => {
    setError(null);
    const result = await createUnit({
      spaceId: baseSpaceId,
      upstreamSpaceId: baseSpaceId,
      upstreamUnitId: templateId,
      unit: {
        Slug: name,
        ToolchainType: 'Kubernetes/YAML',
        Labels: { app: 'rbac-manager', persona: name },
        LastChangeDescription: `Create persona ${name} from template`,
      },
    });
    if ('error' in result && result.error) {
      setError('Create failed — the slug may already exist.');
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
            <InputLabel>Clone from template</InputLabel>
            <Select
              label='Clone from template'
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
            Creates the persona in the base Space as a tracked clone of the template. Edit it
            there, then roll it out to clusters (fleet operations land in a later milestone).
          </Alert>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose(false)}>Cancel</Button>
        <Button
          variant='contained'
          disabled={!nameValid || templateId === '' || createState.isLoading}
          onClick={() => void create()}
        >
          {createState.isLoading ? 'Creating…' : 'Create'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
