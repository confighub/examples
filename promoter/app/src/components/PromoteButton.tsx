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

/**
 * The manual promotion gate. Promotes a component into a stage by upgrading
 * its variant-Space units from the upstream stage's variant. It is disabled
 * (with a reason) when the upstream stage isn't ready yet, when the chosen
 * variants aren't actually linked upstream, or when a target is missing — we
 * never silently copy data. The app changes desired state here; ConfigHub then
 * reports the resulting live status back via the Space label.
 */
export function PromoteButton({
  target,
  upstream,
  blockedReason,
  onPromoted,
}: {
  target: VariantRef | undefined;
  upstream: VariantRef | undefined;
  /** When set, the gate is closed for this reason (e.g. upstream not ready). */
  blockedReason?: string;
  /** Called after a successful upgrade so the parent can re-read status. */
  onPromoted: () => void;
}) {
  const { inspect, promote } = usePromotion();
  const [open, setOpen] = useState(false);
  const [report, setReport] = useState<PromotabilityReport | null>(null);
  const [inspecting, setInspecting] = useState(false);
  const [apply, setApply] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const staticReason =
    blockedReason ??
    (!upstream
      ? 'No upstream stage promotes this component.'
      : !target
        ? 'This stage’s variant no longer exists.'
        : null);

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
      await promote(target, report, `Promote ${target.component} to ${target.variant}`, apply);
      setOpen(false);
      setBusy(false);
      onPromoted();
    } catch {
      setBusy(false);
      setError('Promotion failed.');
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
