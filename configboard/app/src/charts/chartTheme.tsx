import { useTheme } from '@mui/material/styles';

import type { Frame } from '../model/types';
import type { ColorRole } from '../model/types';
import { GRID, type Mode, categorical, emphasisColor, keyColor, statusColor } from './palette';

export function useMode(): Mode {
  return useTheme().palette.mode === 'dark' ? 'dark' : 'light';
}

export function useAxisProps() {
  const theme = useTheme();
  const mode = useMode();
  return {
    tick: { fill: theme.palette.text.secondary, fontSize: 11 },
    axisLine: { stroke: GRID[mode] },
    tickLine: false as const,
  };
}

export function useGridStroke(): string {
  return GRID[useMode()];
}

/**
 * Legend labels wear a text token, never the series colour — Recharts colours them to
 * match the mark by default, which puts identity on colour alone and leaves the label
 * unreadable for the low-contrast slots. The swatch beside it carries the identity.
 */
export function useLegendFormatter() {
  const theme = useTheme();
  return (value: unknown) => (
    <span style={{ color: theme.palette.text.secondary, fontSize: 12 }}>{String(value)}</span>
  );
}

/** Recharts wants row objects keyed by series name. */
export interface ChartDatum {
  key: string;
  [series: string]: string | number;
}

export function frameToData(frame: Frame): ChartDatum[] {
  return frame.categories.map((category) => {
    const datum: ChartDatum = { key: category };
    for (const series of frame.series) {
      const point = series.points.find((p) => p.key === category);
      datum[series.name] = point?.value ?? 0;
    }
    return datum;
  });
}

/**
 * Colour for a series or category. Indices come from the full stable key list, so a
 * filter that removes series never repaints the ones that remain.
 */
export function colorFor(
  key: string,
  index: number,
  count: number,
  role: ColorRole | undefined,
  mode: Mode,
  emphasize?: string,
): string {
  switch (role) {
    case 'status':
      return statusColor(key, mode);
    case 'emphasis':
      return emphasisColor(key, emphasize, mode);
    case 'sequential':
      return keyColor(key, index, mode, 'sequential', count);
    case 'categorical':
      return keyColor(key, index, mode, 'categorical', count);
    default:
      return categorical(index, mode);
  }
}

/** Compact number formatting for axes and labels. */
export function formatValue(value: number, unit?: string): string {
  const abs = Math.abs(value);
  let text: string;
  if (abs >= 1_000_000) text = `${(value / 1_000_000).toFixed(1)}M`;
  else if (abs >= 1_000) text = `${(value / 1_000).toFixed(1)}k`;
  else if (!Number.isInteger(value)) text = value.toFixed(1);
  else text = String(value);
  return unit ? `${text} ${unit}` : text;
}

/** Longest category label an axis renders before eliding the middle. */
const MAX_LABEL = 26;

/**
 * Time-bin keys are ISO dates; show them shortened on a dense axis. Long identifiers
 * (`CustomResourceDefinition`, deep space slugs) get elided in the middle rather than
 * clipped at the edge — the tail of a resource kind or a slug is usually the part that
 * distinguishes it.
 */
export function formatCategory(key: string): string {
  const isoDate = /^\d{4}-\d{2}-\d{2}$/;
  if (isoDate.test(key)) {
    const d = new Date(`${key}T00:00:00Z`);
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
  }
  if (key.length <= MAX_LABEL) return key;
  const head = Math.ceil((MAX_LABEL - 1) / 2);
  const tail = Math.floor((MAX_LABEL - 1) / 2);
  return `${key.slice(0, head)}…${key.slice(key.length - tail)}`;
}
