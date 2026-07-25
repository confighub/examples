import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import Divider from '@mui/material/Divider';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useEffect, useMemo, useState } from 'react';

import {
  type DiscoveredTrigger,
  RECORDABLE,
  type Recordable,
  attachCommands,
  usePinnedDimensions,
  valuesDimension,
  valuesKey,
} from '../storage/pinnedDimensions';
import { STORAGE_SPACE_SLUG, useDashboardStorage } from '../storage/dashboards';

export interface PinnedDimensionsDialogProps {
  /** Space slugs in the org, for the attach-command generator. */
  spaceSlugs: string[];
  onClose: () => void;
}

/**
 * Pinned dimensions turn a config value into filterable metadata.
 *
 * The dialog does two things and refuses a third: it creates the recording Trigger in
 * configboard's own Space, it shows which recording Triggers already exist anywhere in
 * the org, and it *generates* — rather than runs — the commands that attach a Trigger to
 * a workload Space, because that edits a Space this app does not own.
 */
export function PinnedDimensionsDialog({ spaceSlugs, onClose }: PinnedDimensionsDialogProps) {
  const pinned = usePinnedDimensions();
  const storage = useDashboardStorage();

  const [discovered, setDiscovered] = useState<DiscoveredTrigger[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [selected, setSelected] = useState<Recordable>(RECORDABLE[0]);
  const [targetSpace, setTargetSpace] = useState(spaceSlugs[0] ?? '');

  useEffect(() => {
    let cancelled = false;
    pinned
      .discover()
      .then((d) => {
        if (!cancelled) setDiscovered(d);
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : String(e));
          setDiscovered([]);
        }
      });
    return () => {
      cancelled = true;
    };
    // discover is stable for the hook's lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const existingKeys = useMemo(
    () => new Set((discovered ?? []).map((d) => d.key)),
    [discovered],
  );

  const create = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const spaceId = await storage.ensureSpace();
      await pinned.createTrigger(spaceId, selected);
      setDiscovered(await pinned.discover());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const commands = attachCommands(targetSpace || '<space>', selected).join('\n');

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        Pinned dimensions
        <Typography variant="caption" color="text.secondary" display="block">
          Record a config value into <code>Unit.Values</code>, where it becomes filterable
          in <code>where</code> instead of projection-only.
        </Typography>
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <Alert severity="info" variant="outlined">
            A View's data-path dimension can be grouped and charted, but never filtered
            server-side, and every query reparses config to read it. A value recorded by a
            Mutation Trigger is indexed metadata — <code>Values."Image/container-image"</code>
            works in a <code>where</code> clause and comes back with the plain unit list.
          </Alert>

          <Alert severity="info" variant="outlined">
            Recording depends on <strong>selection</strong>, not just existence. A Space
            selects Triggers through its <code>WhereTrigger</code> and{' '}
            <code>TriggerFilterID</code>, and by default a Space selects only the Triggers
            defined in itself. A recording Trigger that no unit-holding Space selects
            produces nothing — and looks exactly like a broken feature.
          </Alert>

          <Box>
            <Typography variant="subtitle2" gutterBottom>
              Recording Triggers already in this organization
            </Typography>
            {discovered === null ? (
              <Typography variant="body2" color="text.secondary">
                Looking…
              </Typography>
            ) : discovered.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                None. Nothing is recording config values into <code>Unit.Values</code> yet.
              </Typography>
            ) : (
              <Stack spacing={0.75}>
                {discovered.map((d) => {
                  const spaces = d.selectedBy.length;
                  const units = d.selectedBy.reduce((sum, s) => sum + s.units, 0);
                  // A Trigger that no Space with units selects records nothing. That is
                  // the failure mode that reads as a broken feature, so name it.
                  const dead = units === 0;
                  return (
                    <Stack key={`${d.space}/${d.slug}`} direction="row" spacing={1} alignItems="center">
                      <Chip size="small" label={`${d.space}/${d.slug}`} />
                      <Typography variant="caption" color="text.secondary">
                        {d.fn} → <code>Unit.Values.{d.key}</code>
                      </Typography>
                      <Chip
                        size="small"
                        variant="outlined"
                        color={dead ? 'warning' : 'default'}
                        label={
                          spaces === 0
                            ? 'no Space selects it'
                            : dead
                              ? `selected by ${spaces} Space${spaces === 1 ? '' : 's'} holding 0 units`
                              : `${spaces} Space${spaces === 1 ? '' : 's'}, ${units} units — fires on ${d.toolchain || 'any toolchain'}`
                        }
                      />
                    </Stack>
                  );
                })}
              </Stack>
            )}
          </Box>

          <Divider />

          <TextField
            select
            label="Value to record"
            size="small"
            value={selected.slug}
            onChange={(e) =>
              setSelected(RECORDABLE.find((r) => r.slug === e.target.value) ?? RECORDABLE[0])
            }
            helperText={selected.description}
          >
            {RECORDABLE.map((r) => (
              <MenuItem key={r.slug} value={r.slug}>
                {r.label} — {r.fn}
              </MenuItem>
            ))}
          </TextField>

          <Typography variant="body2" color="text.secondary">
            Dimension id once recorded: <code>{valuesDimension(selected)}</code>
          </Typography>

          <Stack direction="row" spacing={1} alignItems="center">
            <Button variant="contained" size="small" onClick={() => void create()} disabled={busy}>
              Create trigger in {STORAGE_SPACE_SLUG}
            </Button>
            {existingKeys.has(valuesKey(selected)) && (
              <Typography variant="caption" color="text.secondary">
                A Trigger producing <code>{valuesKey(selected)}</code> already exists.
              </Typography>
            )}
          </Stack>

          <Alert severity="warning" variant="outlined">
            A Trigger applies to a <strong>Space</strong>, not to the organization. Created
            here it records values for configboard's own units, which are dashboards.
            Recording across the fleet means attaching it to the Spaces that hold your
            workloads — editing Spaces this app does not own — so configboard generates the
            commands instead of running them.
          </Alert>

          <TextField
            select
            label="Generate attach commands for"
            size="small"
            value={targetSpace}
            onChange={(e) => setTargetSpace(e.target.value)}
          >
            {spaceSlugs.map((s) => (
              <MenuItem key={s} value={s}>
                {s}
              </MenuItem>
            ))}
          </TextField>

          <Box
            component="pre"
            sx={{
              p: 1,
              m: 0,
              fontSize: 11,
              overflow: 'auto',
              maxHeight: 240,
              bgcolor: 'action.hover',
              borderRadius: 1,
            }}
          >
            {commands}
          </Box>

          {error && (
            <Alert severity="error" variant="outlined">
              {error}
            </Alert>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => void navigator.clipboard?.writeText(commands)}>Copy commands</Button>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
