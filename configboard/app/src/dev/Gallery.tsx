// A dev harness: every chart form rendered against synthetic frames, with no
// ConfigHub instance involved. Open it with `npm run gallery`. It exists so the visual
// layer can be checked — and re-checked after a palette swap — without waiting on real
// fleet data to contain the right shapes.

import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Container from '@mui/material/Container';
import Grid from '@mui/material/Grid2';
import Typography from '@mui/material/Typography';

import { BarChart } from '../charts/BarChart';
import { FrameTable } from '../charts/DataTable';
import { DonutChart } from '../charts/DonutChart';
import { LineChart } from '../charts/LineChart';
import { Meter, StatTile } from '../charts/StatTile';
import type { Frame, Row } from '../model/types';
import { aggregate } from '../query/aggregate';

const row = (id: string, values: Record<string, string | number | null>): Row => ({ id, values });

const KINDS = [
  ['Deployment', 34],
  ['Service', 28],
  ['ConfigMap', 21],
  ['ServiceAccount', 14],
  ['Ingress', 9],
  ['StatefulSet', 6],
  ['CronJob', 4],
  ['NetworkPolicy', 3],
  ['PodDisruptionBudget', 2],
] as const;

const kindRows: Row[] = KINDS.flatMap(([kind, n]) =>
  Array.from({ length: n }, (_, i) => row(`${kind}-${i}`, { kind })),
);

const STATES = [
  ['Applied and current', 62],
  ['Unapplied changes', 18],
  ['Never applied', 7],
  ['Not deployable', 24],
] as const;

const stateRows: Row[] = STATES.flatMap(([state, n]) =>
  Array.from({ length: n }, (_, i) => row(`${state}-${i}`, { state })),
);

const ENVS = ['prod', 'staging', 'dev'] as const;
const timeRows: Row[] = ENVS.flatMap((env, e) =>
  Array.from({ length: 14 }, (_, d) => {
    const count = 2 + ((d * (e + 2)) % 5);
    return Array.from({ length: count }, (_, i) =>
      row(`${env}-${d}-${i}`, {
        env,
        t: `2026-07-${String(10 + d).padStart(2, '0')}T09:00:00Z`,
      }),
    );
  }).flat(),
);

const stackRows: Row[] = ENVS.flatMap((env) =>
  STATES.flatMap(([state, n]) =>
    Array.from({ length: Math.max(1, Math.round(n / (env === 'prod' ? 2 : 3))) }, (_, i) =>
      row(`${env}-${state}-${i}`, { env, state }),
    ),
  ),
);

const frames: { title: string; note: string; frame: Frame; node: (f: Frame) => JSX.Element }[] = [
  {
    title: 'bar — horizontal, sequential, topN fold',
    note: '9 categories folded to 7 + Other; Other is gray, never a 9th hue',
    frame: aggregate(kindRows, {
      groupBy: 'kind',
      aggregate: { fn: 'count' },
      topN: 7,
    }),
    node: (f) => <BarChart frame={f} spec={{ form: 'bar', color: 'sequential' }} />,
  },
  {
    title: 'bar — many categories, every band labelled',
    note: 'no topN fold: the plot grows so no category tick is thinned out and mislabels a bar',
    frame: aggregate(kindRows, { groupBy: 'kind', aggregate: { fn: 'count' } }),
    node: (f) => <BarChart frame={f} spec={{ form: 'bar', color: 'sequential' }} />,
  },
  {
    title: 'bar — status colors',
    note: 'reserved status palette; the label carries the state, not just the hue',
    frame: aggregate(stateRows, { groupBy: 'state', aggregate: { fn: 'count' } }),
    node: (f) => <BarChart frame={f} spec={{ form: 'bar', color: 'status' }} />,
  },
  {
    title: 'bar — vertical, emphasis',
    note: 'one series in the accent hue, the rest recessive',
    frame: aggregate(kindRows, { groupBy: 'kind', aggregate: { fn: 'count' }, topN: 5 }),
    node: (f) => (
      <BarChart
        frame={f}
        spec={{ form: 'bar', orientation: 'vertical', color: 'emphasis', emphasize: 'Deployment' }}
      />
    ),
  },
  {
    title: 'stackedBar — two group keys',
    note: '2px surface gap between segments so stacks do not read as one block',
    frame: aggregate(stackRows, { groupBy: ['env', 'state'], aggregate: { fn: 'count' } }),
    node: (f) => (
      <BarChart frame={f} spec={{ form: 'stackedBar', color: 'status' }} stacked />
    ),
  },
  {
    title: 'line — binned by day, one series per environment',
    note: 'time axis ordered by time; 3 series, legend present',
    frame: aggregate(timeRows, {
      bin: { field: 't', unit: 'day' },
      groupBy: ['t', 'env'],
      aggregate: { fn: 'count' },
    }),
    node: (f) => <LineChart frame={f} spec={{ form: 'line', color: 'categorical' }} />,
  },
  {
    title: 'donut — 4 slices',
    note: 'part-to-whole with a small slice count; every label in the legend',
    frame: aggregate(stateRows, { groupBy: 'state', aggregate: { fn: 'count' } }),
    node: (f) => <DonutChart frame={f} spec={{ form: 'donut', color: 'categorical' }} />,
  },
  {
    title: 'table view',
    note: 'the relief every panel can switch to',
    frame: aggregate(kindRows, { groupBy: 'kind', aggregate: { fn: 'count' }, topN: 7 }),
    node: (f) => <FrameTable frame={f} dimensionLabel="Resource kind" />,
  },
];

export function Gallery() {
  const total = aggregate(kindRows, {});

  return (
    <Container maxWidth="xl" sx={{ py: 3 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 0.5 }}>
        configboard chart gallery
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        Synthetic data. Verifies the visual layer without an instance.
      </Typography>

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 3 }}>
          <Card variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2">statTile</Typography>
            <StatTile value={total.total} label="units under management" />
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 3 }}>
          <Card variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2">statTile — status</Typography>
            <StatTile value={7} label="blocked by apply gates" level="critical" />
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 3 }}>
          <Card variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2">meter</Typography>
            <Meter value={62} total={87} label="applied and current" />
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 3 }}>
          <Card variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2">meter — inverted</Typography>
            <Meter value={18} total={87} label="unapplied changes" invert />
          </Card>
        </Grid>

        {frames.map(({ title, note, frame, node }) => (
          <Grid key={title} size={{ xs: 12, md: 6 }}>
            <Card variant="outlined" sx={{ p: 2 }}>
              <Typography variant="subtitle2">{title}</Typography>
              <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
                {note}
              </Typography>
              <Box>{node(frame)}</Box>
            </Card>
          </Grid>
        ))}
      </Grid>
    </Container>
  );
}
