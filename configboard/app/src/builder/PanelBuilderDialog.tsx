import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import Grid from '@mui/material/Grid2';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useMemo, useState } from 'react';

import { serializePanelItem } from '../model/parse';
import type {
  AggregateFn,
  ChartForm,
  Dashboard,
  Panel,
  SourceName,
} from '../model/types';
import { PanelRenderer } from '../panels/PanelRenderer';
import type { Scope } from '../query/compile';
import { type Dimension, dimensionsFor } from '../query/dimensions';
import { VIEW_SEEDS, viewRef } from '../storage/views';

const SOURCES: SourceName[] = ['Unit', 'Space', 'Revision', 'Target', 'Resource'];
const FORMS: ChartForm[] = [
  'bar',
  'stackedBar',
  'line',
  'donut',
  'heatmap',
  'histogram',
  'statTile',
  'meter',
];
const AGGREGATES: AggregateFn[] = [
  'count',
  'sum',
  'avg',
  'min',
  'max',
  'p50',
  'p95',
  'distinctCount',
  'value',
];

const NONE = '';

export interface PanelBuilderDialogProps {
  dashboard: Dashboard;
  baseUrl: string;
  scope: Scope;
  onClose: () => void;
  onAdd: (panel: Panel) => Promise<void>;
}

/**
 * Compose a panel by picking a source, dimensions, a measure, and a form — with the
 * result rendered live against real data before it is saved. The output is a panel
 * stanza appended to the dashboard document, so what the builder produces is the same
 * thing a hand-editor would write.
 */
export function PanelBuilderDialog({
  dashboard,
  baseUrl,
  scope,
  onClose,
  onAdd,
}: PanelBuilderDialogProps) {
  const [title, setTitle] = useState('New panel');
  const [source, setSource] = useState<SourceName>('Unit');
  const [view, setView] = useState<string>(NONE);
  const [groupBy, setGroupBy] = useState<string>(NONE);
  const [groupBy2, setGroupBy2] = useState<string>(NONE);
  const [fn, setFn] = useState<AggregateFn>('count');
  const [field, setField] = useState<string>(NONE);
  const [form, setForm] = useState<ChartForm>('bar');
  const [span, setSpan] = useState(6);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>();

  // A View contributes its columns as extra dimensions; only Unit queries can use one.
  const viewDimensions = useMemo((): Dimension[] => {
    const seed = VIEW_SEEDS.find((v) => viewRef(v.slug) === view);
    if (!seed) return [];
    return seed.dimensions.map((id) => ({
      id,
      label: `${id.replace(/^View\./, '')} (view)`,
      kind: 'string' as const,
      source,
    }));
  }, [view, source]);

  const dimensions = useMemo(
    () => [...dimensionsFor(source), ...viewDimensions],
    [source, viewDimensions],
  );

  const numericDimensions = useMemo(
    () => dimensions.filter((d) => d.kind === 'number' || view !== NONE),
    [dimensions, view],
  );

  const needsField = fn !== 'count';
  const draft = useMemo((): Panel => {
    const groups = [groupBy, groupBy2].filter((g) => g !== NONE);
    return {
      id: slugify(title) || 'new-panel',
      title,
      span,
      query: {
        source,
        ...(view !== NONE ? { view } : {}),
      },
      transform: {
        ...(groups.length > 0 ? { groupBy: groups.length === 1 ? groups[0] : groups } : {}),
        aggregate: { fn, ...(needsField && field !== NONE ? { field } : {}) },
        ...(groups.length > 0 ? { sort: 'value-desc' as const, topN: 12, tail: 'drop' as const } : {}),
        dropEmpty: true,
      },
      chart: {
        form,
        ...(form === 'bar' ? { orientation: 'horizontal' as const } : {}),
        // One series: colour encodes magnitude. Several: colour carries identity.
        ...(groups.length > 1
          ? { color: 'categorical' as const }
          : form === 'bar' || form === 'histogram'
            ? { color: 'sequential' as const }
            : {}),
      },
    };
  }, [title, span, source, view, groupBy, groupBy2, fn, field, needsField, form]);

  const problems = useMemo((): string[] => {
    const out: string[] = [];
    if (title.trim().length === 0) out.push('A title is required.');
    if (needsField && field === NONE) out.push(`Aggregate "${fn}" needs a field.`);
    if (form === 'heatmap' && [groupBy, groupBy2].filter((g) => g !== NONE).length < 2) {
      out.push('A heatmap needs two group-by dimensions: rows and columns.');
    }
    if ((form === 'statTile' || form === 'meter') && groupBy !== NONE) {
      out.push('A stat tile shows one number — remove the group-by, or pick a bar.');
    }
    if (form === 'meter') out.push('A meter needs chart.totalField; add it in the source editor.');
    if (dashboard.panels.some((p) => p.id === draft.id)) {
      out.push(`A panel with id "${draft.id}" already exists — change the title.`);
    }
    return out;
  }, [title, needsField, field, fn, form, groupBy, groupBy2, dashboard.panels, draft.id]);

  const submit = async () => {
    setBusy(true);
    setError(undefined);
    try {
      await onAdd(draft);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="lg" fullWidth>
      <DialogTitle>
        Add a panel to {dashboard.title}
        <Typography variant="caption" color="text.secondary" display="block">
          Rendered live against your data. Saving appends this stanza to the dashboard
          document.
        </Typography>
      </DialogTitle>
      <DialogContent>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 5 }}>
            <Stack spacing={2} sx={{ mt: 1 }}>
              <TextField
                label="Title"
                size="small"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                fullWidth
              />
              <TextField
                select
                label="Source"
                size="small"
                value={source}
                onChange={(e) => {
                  setSource(e.target.value as SourceName);
                  setGroupBy(NONE);
                  setGroupBy2(NONE);
                  setField(NONE);
                  setView(NONE);
                }}
              >
                {SOURCES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {s}
                  </MenuItem>
                ))}
              </TextField>

              {source === 'Unit' && (
                <TextField
                  select
                  label="View (data-path dimensions)"
                  size="small"
                  value={view}
                  onChange={(e) => {
                    setView(e.target.value);
                    setGroupBy(NONE);
                    setGroupBy2(NONE);
                    setField(NONE);
                  }}
                  helperText="Reads values out of the configuration data."
                >
                  <MenuItem value={NONE}>none — metadata only</MenuItem>
                  {VIEW_SEEDS.map((v) => (
                    <MenuItem key={v.slug} value={viewRef(v.slug)}>
                      {v.slug}
                    </MenuItem>
                  ))}
                </TextField>
              )}

              <TextField
                select
                label="Group by"
                size="small"
                value={groupBy}
                onChange={(e) => setGroupBy(e.target.value)}
              >
                <MenuItem value={NONE}>none — a single number</MenuItem>
                {dimensions.map((d) => (
                  <MenuItem key={d.id} value={d.id}>
                    {d.label} — {d.id}
                  </MenuItem>
                ))}
              </TextField>

              <TextField
                select
                label="Then by (series / columns)"
                size="small"
                value={groupBy2}
                onChange={(e) => setGroupBy2(e.target.value)}
                disabled={groupBy === NONE}
              >
                <MenuItem value={NONE}>none</MenuItem>
                {dimensions
                  .filter((d) => d.id !== groupBy)
                  .map((d) => (
                    <MenuItem key={d.id} value={d.id}>
                      {d.label} — {d.id}
                    </MenuItem>
                  ))}
              </TextField>

              <Stack direction="row" spacing={1}>
                <TextField
                  select
                  label="Measure"
                  size="small"
                  value={fn}
                  onChange={(e) => setFn(e.target.value as AggregateFn)}
                  sx={{ flex: 1 }}
                >
                  {AGGREGATES.map((a) => (
                    <MenuItem key={a} value={a}>
                      {a}
                    </MenuItem>
                  ))}
                </TextField>
                <TextField
                  select
                  label="of field"
                  size="small"
                  value={field}
                  onChange={(e) => setField(e.target.value)}
                  disabled={!needsField}
                  sx={{ flex: 1 }}
                >
                  <MenuItem value={NONE}>—</MenuItem>
                  {numericDimensions.map((d) => (
                    <MenuItem key={d.id} value={d.id}>
                      {d.label}
                    </MenuItem>
                  ))}
                </TextField>
              </Stack>

              <Stack direction="row" spacing={1}>
                <TextField
                  select
                  label="Chart"
                  size="small"
                  value={form}
                  onChange={(e) => setForm(e.target.value as ChartForm)}
                  sx={{ flex: 1 }}
                >
                  {FORMS.map((f) => (
                    <MenuItem key={f} value={f}>
                      {f}
                    </MenuItem>
                  ))}
                </TextField>
                <TextField
                  select
                  label="Width"
                  size="small"
                  value={span}
                  onChange={(e) => setSpan(Number(e.target.value))}
                  sx={{ flex: 1 }}
                >
                  {[3, 4, 6, 8, 12].map((s) => (
                    <MenuItem key={s} value={s}>
                      {s} / 12
                    </MenuItem>
                  ))}
                </TextField>
              </Stack>

              {problems.length > 0 && (
                <Alert severity="warning" variant="outlined">
                  {problems.map((p) => (
                    <div key={p}>{p}</div>
                  ))}
                </Alert>
              )}
              {error && (
                <Alert severity="error" variant="outlined">
                  {error}
                </Alert>
              )}
            </Stack>
          </Grid>

          <Grid size={{ xs: 12, md: 7 }}>
            <Typography variant="caption" color="text.secondary">
              Preview
            </Typography>
            <Box sx={{ mt: 0.5 }}>
              <PanelRenderer panel={draft} scope={scope} baseUrl={baseUrl} />
            </Box>
            <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
              Document stanza
            </Typography>
            <Box
              component="pre"
              sx={{
                mt: 0.5,
                p: 1,
                fontSize: 11,
                maxHeight: 180,
                overflow: 'auto',
                bgcolor: 'action.hover',
                borderRadius: 1,
              }}
            >
              {serializePanelItem(draft)}
            </Box>
          </Grid>
        </Grid>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          onClick={() => void submit()}
          disabled={busy || problems.length > 0}
        >
          Add panel
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** Panel ids double as anchors, so they follow the slug rules the rest of the app uses. */
export function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60);
}
