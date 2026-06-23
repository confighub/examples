// Scope settings: filter expressions selecting which Targets (clusters) and
// Spaces (for untargeted base Units) the app analyzes. Defaults to
// everything the user can view.

import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Link,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { FleetScope, loadScope, saveScope } from '../fleet/scope';

export interface ScopeSettingsProps {
  open: boolean;
  onClose: (changed: boolean) => void;
}

export function ScopeSettings({ open, onClose }: ScopeSettingsProps) {
  const [scope, setScope] = useState<FleetScope>(() => loadScope());

  const set = (patch: Partial<FleetScope>) => setScope({ ...scope, ...patch });

  return (
    <Dialog open={open} onClose={() => onClose(false)} maxWidth='sm' fullWidth>
      <DialogTitle>Analysis scope</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <Typography variant='body2' color='text.secondary'>
            Filter expressions use ConfigHub&apos;s <code>where</code> syntax (AND-combined
            comparisons; <code>LIKE</code>/<code>IN</code> supported), e.g.{' '}
            <code>Slug LIKE &apos;prod-%&apos;</code> or{' '}
            <code>Labels.Environment = &apos;prod&apos;</code>. Leave blank to include everything
            you have permission to view.
          </Typography>
          <TextField
            size='small'
            label='Target filter (clusters)'
            helperText='Which Targets count as clusters under analysis'
            value={scope.targetWhere}
            onChange={(e) => set({ targetWhere: e.target.value })}
          />
          <TextField
            size='small'
            label='Space filter (base units without a Target)'
            helperText='Which Spaces contribute untargeted base Units'
            value={scope.spaceWhere}
            onChange={(e) => set({ spaceWhere: e.target.value })}
          />
          <Alert severity='info'>
            Scope is saved in this browser. See the{' '}
            <Link
              href='https://docs.confighub.com'
              target='_blank'
              rel='noreferrer'
            >
              ConfigHub docs
            </Link>{' '}
            for the full filter-expression syntax.
          </Alert>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose(false)}>Cancel</Button>
        <Button
          variant='contained'
          onClick={() => {
            saveScope(scope);
            onClose(true);
          }}
        >
          Save &amp; reload
        </Button>
      </DialogActions>
    </Dialog>
  );
}
