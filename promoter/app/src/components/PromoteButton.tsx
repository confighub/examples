import {
  Alert,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  List,
  ListItem,
  ListItemText,
  Tooltip,
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { VariantRef } from '../data/catalog';
import { PromotabilityReport, usePromotion } from '../data/promote';
import { PromotionState } from '../model/workflow';

/**
 * Promote a single component into a stage by upgrading its variant-Space units
 * from the upstream stage's variant. The button is disabled (with a reason)
 * whenever the upstream linkage required for a real upgrade isn't present —
 * we never silently copy data instead.
 */
export function PromoteButton({
  target,
  upstream,
  onPromoted,
}: {
  target: VariantRef | undefined;
  upstream: VariantRef | undefined;
  onPromoted: (state: PromotionState, revision?: number) => Promise<void>;
}) {
  const { inspect, promote } = usePromotion();
  const [open, setOpen] = useState(false);
  const [report, setReport] = useState<PromotabilityReport | null>(null);
  const [inspecting, setInspecting] = useState(false);
  const [apply, setApply] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reasons we can't even attempt a promotion, independent of link topology.
  const staticReason =
    !upstream
      ? 'No upstream stage promotes this component.'
      : !target
        ? 'This stage’s variant no longer exists.'
        : null;

  const openDialog = async () => {
    if (!target || !upstream) return;
    setOpen(true);
    setError(null);
    setReport(null);
    setInspecting(true);
    setReport(await inspect(target, upstream));
    setInspecting(false);
  };

  const doPromote = async () => {
    if (!target || !report?.promotable) return;
    setBusy(true);
    setError(null);
    try {
      const result = await promote(
        target,
        report,
        `Promote ${target.component} to ${target.variant}`,
        apply,
      );
      setOpen(false);
      setBusy(false);
      await onPromoted('succeeded', result.revision);
    } catch {
      setBusy(false);
      setError('Promotion failed. Recording stage as failed.');
      await onPromoted('failed');
    }
  };

  if (staticReason) {
    return (
      <Tooltip title={staticReason}>
        <span>
          <Button size='small' disabled>
            Promote
          </Button>
        </span>
      </Tooltip>
    );
  }

  return (
    <>
      <Button size='small' variant='outlined' onClick={openDialog}>
        Promote
      </Button>
      <Dialog open={open} onClose={() => !busy && setOpen(false)} maxWidth='sm' fullWidth>
        <DialogTitle>
          Promote {target?.component} → {target?.variant}
        </DialogTitle>
        <DialogContent>
          {inspecting && (
            <Typography color='text.secondary' sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <CircularProgress size={16} /> Checking upstream links…
            </Typography>
          )}
          {!inspecting && report && (
            <>
              <Alert severity={report.promotable ? 'success' : 'warning'} sx={{ mb: 2 }}>
                {report.summary}
              </Alert>
              <Typography variant='body2' color='text.secondary'>
                Upgrading from upstream variant <code>{upstream?.spaceSlug}</code>.
              </Typography>
              <List dense>
                {report.units.map((u) => (
                  <ListItem key={u.unitId}>
                    <ListItemText
                      primary={u.slug}
                      secondary={u.ok ? `linked · head rev ${u.headRevisionNum}` : u.reason}
                    />
                    <Typography>{u.ok ? '✅' : '⛔'}</Typography>
                  </ListItem>
                ))}
              </List>
              <FormControlLabel
                control={<Checkbox checked={apply} onChange={(e) => setApply(e.target.checked)} />}
                label='Apply to target after upgrade'
              />
              {error && <Alert severity='error'>{error}</Alert>}
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={busy}>
            Cancel
          </Button>
          <Button
            variant='contained'
            onClick={doPromote}
            disabled={busy || inspecting || !report?.promotable}
          >
            {busy ? 'Promoting…' : 'Promote'}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
