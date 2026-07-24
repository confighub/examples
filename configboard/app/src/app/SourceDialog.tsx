import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useMemo, useState } from 'react';

import { parseDashboard } from '../model/parse';
import { unknownDimensions } from '../query/compile';
import type { DashboardEntry } from './useDashboards';

export interface SourceDialogProps {
  entry: DashboardEntry;
  open: boolean;
  onClose: () => void;
  onSave: (yaml: string) => Promise<void>;
}

/**
 * The dashboard document, editable. This is the whole point of storing dashboards as
 * data: the thing you edit is the thing that runs, and it validates before it saves —
 * the same validation the test suite runs over the bundled documents.
 */
export function SourceDialog({ entry, open, onClose, onSave }: SourceDialogProps) {
  const [text, setText] = useState(entry.yaml);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | undefined>();

  const problems = useMemo((): string[] => {
    const { dashboard, errors } = parseDashboard(text);
    if (!dashboard) return errors;
    const unknown = dashboard.panels.flatMap((p) =>
      unknownDimensions(p).map((d) => `panel ${p.id}: unknown dimension ${d}`),
    );
    return [...errors, ...unknown];
  }, [text]);

  const dirty = text !== entry.yaml;
  const canSave = dirty && problems.length === 0 && Boolean(entry.stored);

  const handleSave = async () => {
    setSaving(true);
    setSaveError(undefined);
    try {
      await onSave(text);
      onClose();
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        {entry.dashboard.title}
        <Typography variant="caption" color="text.secondary" display="block">
          {entry.stored
            ? `configboard/${entry.stored.slug} — AppConfig/YAML unit at revision ${entry.stored.headRevision}`
            : 'bundled document — not stored in ConfigHub'}
        </Typography>
      </DialogTitle>
      <DialogContent>
        <TextField
          value={text}
          onChange={(e) => setText(e.target.value)}
          multiline
          fullWidth
          minRows={18}
          maxRows={28}
          slotProps={{ htmlInput: { spellCheck: false } }}
          sx={{ '& textarea': { fontFamily: 'monospace', fontSize: 12, lineHeight: 1.5 } }}
        />
        <Box sx={{ mt: 1 }}>
          {problems.length > 0 ? (
            <Alert severity="warning" variant="outlined">
              {problems.slice(0, 8).map((p) => (
                <div key={p}>{p}</div>
              ))}
              {problems.length > 8 && <div>…and {problems.length - 8} more.</div>}
            </Alert>
          ) : (
            <Typography variant="caption" color="text.secondary">
              {dirty ? 'Document is valid.' : 'No changes.'}
            </Typography>
          )}
          {saveError && (
            <Alert severity="error" variant="outlined" sx={{ mt: 1 }}>
              {saveError}
            </Alert>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
        <Button variant="contained" onClick={() => void handleSave()} disabled={!canSave || saving}>
          Save revision
        </Button>
      </DialogActions>
    </Dialog>
  );
}
