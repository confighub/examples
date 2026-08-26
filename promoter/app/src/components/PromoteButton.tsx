import {
  Alert,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItem,
  ListItemText,
  Tooltip,
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { VariantRef } from '../data/catalog';
import { PromotabilityReport, ReleaseReadiness, usePromotion } from '../data/promote';

/**
 * What publishing would actually capture. A Release is whole-Space: it bundles every
 * Unit assigned to the Space's Release Target, so the set can be wider than the one
 * just promoted. Showing the difference is the point of this block — an approval to
 * promote is not an approval to publish something larger.
 */
function ReleaseScope({
  target,
  release,
}: {
  target: VariantRef | undefined;
  release: ReleaseReadiness;
}) {
  if (!release.publishable) {
    return <Alert severity='warning'>{release.reason}</Alert>;
  }
  return (
    <>
      <Typography variant='body2' color='text.secondary' sx={{ mb: 1 }}>
        Publishing bundles all {release.members.length} Unit(s) in{' '}
        <code>{target?.spaceSlug}</code> assigned to Release Target{' '}
        <code>{release.targetSlug}</code>, each at its current head, as one immutable
        Release.
      </Typography>
      {release.alsoCaptured.length > 0 && (
        <Alert severity='warning' sx={{ mb: 1 }}>
          {release.alsoCaptured.length} of them {release.alsoCaptured.length === 1 ? 'was' : 'were'}{' '}
          not part of this promotion and will be published at whatever head they are on:{' '}
          {release.alsoCaptured.map((m) => m.slug).join(', ')}.
        </Alert>
      )}
      <List dense>
        {release.members.map((m) => (
          <ListItem key={m.unitId}>
            <ListItemText primary={m.slug} />
            <Chip
              size='small'
              label={m.promoted ? 'promoted' : 'already in the Space'}
              color={m.promoted ? 'primary' : 'default'}
              variant='outlined'
            />
          </ListItem>
        ))}
      </List>
    </>
  );
}

/**
 * The manual promotion gate. Promotes a component into a stage by upgrading
 * its variant-Space units from the upstream stage's variant. It is disabled
 * (with a reason) when the upstream stage isn't ready yet, when the chosen
 * variants aren't actually linked upstream, or when a target is missing — we
 * never silently copy data. The app changes desired state here; ConfigHub then
 * reports the resulting live status back via the Space label.
 *
 * Upgrading changes desired state and delivers nothing. Publishing a Release is
 * offered as a second, separately confirmed step, because it has a wider subject:
 * a Release captures every Unit assigned to the Space's Release Target, which can
 * include Units this promotion never touched. Those are listed before the button
 * is offered.
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
  const { inspect, promote, inspectRelease, publish } = usePromotion();
  const [open, setOpen] = useState(false);
  const [report, setReport] = useState<PromotabilityReport | null>(null);
  const [inspecting, setInspecting] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Set once the upgrade lands, which is when publishing becomes a meaningful offer.
  const [promoted, setPromoted] = useState(false);
  const [release, setRelease] = useState<ReleaseReadiness | null>(null);
  const [published, setPublished] = useState<string | null>(null);

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
    setRelease(null);
    setPromoted(false);
    setPublished(null);
    setInspecting(true);
    setReport(await inspect(target, upstream));
    setInspecting(false);
  };

  const doPromote = async () => {
    if (!target || !report?.promotable) return;
    setBusy(true);
    setError(null);
    try {
      await promote(target, report, `Promote ${target.component} to ${target.variant}`);
      // Stay open: the upgrade is done, and publishing it is the next decision.
      // Reading release readiness only now means the gate states reflect the
      // Revisions the upgrade just created.
      setPromoted(true);
      setRelease(await inspectRelease(target, report));
      setBusy(false);
      onPromoted();
    } catch {
      setBusy(false);
      setError('Promotion failed.');
    }
  };

  const doPublish = async () => {
    if (!target || !release?.publishable) return;
    setBusy(true);
    setError(null);
    try {
      const result = await publish(target);
      setPublished(`Release ${result.releaseNum} published — ${result.unitCount} Unit(s) bundled.`);
      setBusy(false);
      onPromoted();
    } catch {
      setBusy(false);
      setError('Publishing the Release failed.');
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
              {!promoted && (
                <Alert severity='info'>
                  Upgrading changes desired state in ConfigHub and delivers nothing. After it
                  lands you can publish a Release, which is what a cluster pulls.
                </Alert>
              )}
            </>
          )}

          {promoted && !published && (
            <>
              <Alert severity='success' sx={{ mb: 2 }}>
                Upgraded {report?.units.length} Unit(s). Nothing is delivered yet.
              </Alert>
              {release === null && (
                <Typography
                  color='text.secondary'
                  sx={{ display: 'flex', alignItems: 'center', gap: 1 }}
                >
                  <CircularProgress size={16} /> Checking what a Release would capture…
                </Typography>
              )}
              {release && <ReleaseScope target={target} release={release} />}
            </>
          )}

          {published && <Alert severity='success'>{published}</Alert>}

          {error && <Alert severity='error' sx={{ mt: 2 }}>{error}</Alert>}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={busy}>
            {promoted ? 'Close' : 'Cancel'}
          </Button>
          {!promoted && (
            <Button
              variant='contained'
              onClick={doPromote}
              disabled={busy || inspecting || !report?.promotable}
            >
              {busy ? 'Promoting…' : 'Promote'}
            </Button>
          )}
          {promoted && !published && (
            <Button
              variant='contained'
              onClick={doPublish}
              disabled={busy || release === null || !release.publishable}
            >
              {busy
                ? 'Publishing…'
                : release
                  ? `Publish Release (${release.members.length} Unit${release.members.length === 1 ? '' : 's'})`
                  : 'Publish Release'}
            </Button>
          )}
        </DialogActions>
      </Dialog>
    </>
  );
}
