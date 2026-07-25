import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import { useState } from 'react';

import type { DashboardEntry } from './useDashboards';

/** Slugs are the Unit's identity, so they follow the same rule ConfigHub enforces. */
const SLUG_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export interface DuplicateDialogProps {
  entry: DashboardEntry;
  existingSlugs: string[];
  onClose: () => void;
  onConfirm: (slug: string, title: string) => Promise<void>;
}

export function DuplicateDialog({
  entry,
  existingSlugs,
  onClose,
  onConfirm,
}: DuplicateDialogProps) {
  const [title, setTitle] = useState(`${entry.dashboard.title} copy`);
  const [slug, setSlug] = useState(`${entry.dashboard.slug}-copy`);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>();

  const slugProblem = !SLUG_PATTERN.test(slug)
    ? 'lowercase letters, digits, and hyphens'
    : existingSlugs.includes(slug)
      ? 'already taken'
      : undefined;

  const submit = async () => {
    setBusy(true);
    setError(undefined);
    try {
      await onConfirm(slug, title);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Duplicate dashboard</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField
            label="Title"
            size="small"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            fullWidth
          />
          <TextField
            label="Slug"
            size="small"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            error={Boolean(slugProblem)}
            helperText={slugProblem ?? 'Identifies the unit in the configboard Space.'}
            fullWidth
          />
          {error && (
            <Alert severity="error" variant="outlined">
              {error}
            </Alert>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          onClick={() => void submit()}
          disabled={busy || Boolean(slugProblem) || title.trim().length === 0}
        >
          Duplicate
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export interface ConfirmDialogProps {
  title: string;
  body: string;
  confirmLabel: string;
  onClose: () => void;
  onConfirm: () => Promise<void>;
}

export function ConfirmDialog({
  title,
  body,
  confirmLabel,
  onClose,
  onConfirm,
}: ConfirmDialogProps) {
  const [busy, setBusy] = useState(false);

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <DialogContentText>{body}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          color="error"
          variant="contained"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void onConfirm().finally(() => setBusy(false));
          }}
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
