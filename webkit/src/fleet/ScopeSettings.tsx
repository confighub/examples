// Scope settings: the filter expressions selecting which Targets (clusters) and Spaces
// (for untargeted base Units) an app analyzes. Defaults to everything the user can view.

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

import type { FleetScope, ScopeStore } from './scope';

export interface ScopeSettingsProps {
  open: boolean;
  store: ScopeStore;
  /** `changed` is true when the scope was saved, so the caller can rebuild. */
  onClose: (changed: boolean) => void;
}

export function ScopeSettings({ open, store, onClose }: ScopeSettingsProps) {
  const [scope, setScope] = useState<FleetScope>(() => store.load());

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
            <Link href='https://docs.confighub.com' target='_blank' rel='noreferrer'>
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
            store.save(scope);
            onClose(true);
          }}
        >
          Save &amp; reload
        </Button>
      </DialogActions>
    </Dialog>
  );
}
