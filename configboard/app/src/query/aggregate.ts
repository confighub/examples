// The GROUP BY the server does not do. Pure functions over Rows: bin, group,
// aggregate, fold the tail into "Other". The same output feeds the chart and the
// table view, so what you see plotted is what the table shows.

import type {
  AggregateFn,
  BinUnit,
  Frame,
  Point,
  Row,
  RowValue,
  Series,
  Transform,
} from '../model/types';

export const OTHER = 'Other';
export const NONE = '(none)';

const DAY = 24 * 60 * 60 * 1000;

export function binTimestamp(iso: string, unit: BinUnit): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return NONE;
  d.setUTCHours(0, 0, 0, 0);
  if (unit === 'week') {
    // ISO weeks start Monday; getUTCDay() is 0 for Sunday.
    const dow = (d.getUTCDay() + 6) % 7;
    d.setTime(d.getTime() - dow * DAY);
  } else if (unit === 'month') {
    d.setUTCDate(1);
  }
  return d.toISOString().slice(0, 10);
}

function numeric(value: RowValue): number | null {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null;
  if (typeof value === 'string' && value.trim() !== '') {
    const n = Number(value);
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

function percentile(sorted: number[], p: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length));
  return sorted[idx];
}

export function applyAggregate(rows: Row[], fn: AggregateFn, field?: string): number {
  if (fn === 'count') return rows.length;

  if (fn === 'distinctCount') {
    const seen = new Set<RowValue>();
    for (const r of rows) seen.add(field ? r.values[field] : r.id);
    return seen.size;
  }

  if (!field) return rows.length;
  const nums = rows.map((r) => numeric(r.values[field])).filter((n): n is number => n !== null);
  if (nums.length === 0) return 0;

  switch (fn) {
    case 'sum':
      return nums.reduce((a, b) => a + b, 0);
    case 'avg':
      return nums.reduce((a, b) => a + b, 0) / nums.length;
    case 'min':
      return Math.min(...nums);
    case 'max':
      return Math.max(...nums);
    case 'p50':
      return percentile([...nums].sort((a, b) => a - b), 50);
    case 'p95':
      return percentile([...nums].sort((a, b) => a - b), 95);
    default:
      return nums.length;
  }
}

function keyOf(row: Row, field: string, bin?: Transform['bin']): string | null {
  const raw = row.values[field];
  if (raw === null || raw === undefined || raw === '') return null;
  if (bin && bin.field === field) return binTimestamp(String(raw), bin.unit);
  return String(raw);
}

function sortCategories(
  categories: string[],
  totals: Map<string, number>,
  sort: Transform['sort'],
): string[] {
  const rest = categories.filter((c) => c !== OTHER);
  const hasOther = categories.length !== rest.length;

  switch (sort) {
    case 'key-asc':
      rest.sort((a, b) => a.localeCompare(b));
      break;
    case 'key-desc':
      rest.sort((a, b) => b.localeCompare(a));
      break;
    case 'value-asc':
      rest.sort((a, b) => (totals.get(a) ?? 0) - (totals.get(b) ?? 0));
      break;
    default:
      rest.sort((a, b) => (totals.get(b) ?? 0) - (totals.get(a) ?? 0));
      break;
  }
  // "Other" is a residue, not a rank — it always sits at the end.
  return hasOther ? [...rest, OTHER] : rest;
}

/**
 * Rows -> Frame. With no `groupBy` the whole set is one point, which is what a stat
 * tile wants; with two keys the second becomes the series dimension (stacked bars,
 * multi-line).
 */
export function aggregate(rows: Row[], transform: Transform | undefined): Frame {
  const t = transform ?? {};
  const fn = t.aggregate?.fn ?? 'count';
  const field = t.aggregate?.field;

  const groupKeys = typeof t.groupBy === 'string' ? [t.groupBy] : (t.groupBy ?? []);
  if (groupKeys.length === 0) {
    return {
      categories: ['total'],
      series: [{ name: 'total', points: [{ key: 'total', value: applyAggregate(rows, fn, field), rows }] }],
      total: applyAggregate(rows, fn, field),
      excluded: 0,
      omittedCategories: 0,
    };
  }

  const [primary, secondary] = groupKeys;
  const buckets = new Map<string, Map<string, Row[]>>();
  let excluded = 0;

  for (const row of rows) {
    const rawKey = keyOf(row, primary, t.bin);
    if (rawKey === null) {
      if (t.dropEmpty) {
        excluded++;
        continue;
      }
    }
    const key = rawKey ?? NONE;
    const seriesKey = secondary ? (keyOf(row, secondary, t.bin) ?? NONE) : 'value';

    let bySeries = buckets.get(key);
    if (!bySeries) {
      bySeries = new Map();
      buckets.set(key, bySeries);
    }
    const list = bySeries.get(seriesKey);
    if (list) list.push(row);
    else bySeries.set(seriesKey, [row]);
  }

  // Category totals drive both sorting and the topN cut.
  const totals = new Map<string, number>();
  for (const [key, bySeries] of buckets) {
    const all = [...bySeries.values()].flat();
    totals.set(key, applyAggregate(all, fn, field));
  }

  let categories = sortCategories([...buckets.keys()], totals, t.sort);

  // Trim the tail. Time bins are never trimmed — a missing day is not "other", it is
  // a gap in the axis.
  let omittedCategories = 0;
  if (t.topN && categories.length > t.topN && !t.bin) {
    const keep = categories.slice(0, t.topN);
    const tail = categories.slice(t.topN);

    if (t.tail === 'drop') {
      for (const key of tail) buckets.delete(key);
      omittedCategories = tail.length;
      categories = keep;
    } else {
      const merged = new Map<string, Row[]>();
      for (const key of tail) {
        for (const [seriesKey, rows_] of buckets.get(key) ?? []) {
          const list = merged.get(seriesKey);
          if (list) list.push(...rows_);
          else merged.set(seriesKey, [...rows_]);
        }
        buckets.delete(key);
      }
      if (merged.size > 0) buckets.set(OTHER, merged);
      categories = [...keep, ...(merged.size > 0 ? [OTHER] : [])];
    }
  }

  // A time axis is ordered by time, not by magnitude.
  if (t.bin) categories = [...categories].sort((a, b) => a.localeCompare(b));

  const seriesNames = secondary
    ? [...new Set([...buckets.values()].flatMap((m) => [...m.keys()]))].sort((a, b) =>
        a.localeCompare(b),
      )
    : ['value'];

  const series: Series[] = seriesNames.map((name) => ({
    name,
    points: categories.map((category): Point => {
      const rowsFor = buckets.get(category)?.get(name) ?? [];
      return { key: category, value: applyAggregate(rowsFor, fn, field), rows: rowsFor };
    }),
  }));

  const total = series.reduce((sum, s) => sum + s.points.reduce((a, p) => a + p.value, 0), 0);
  return { categories, series, total, excluded, omittedCategories };
}

/** Single headline number, for stat tiles and meters. */
export function scalar(frame: Frame): number {
  return frame.total;
}
