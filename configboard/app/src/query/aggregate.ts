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

/**
 * The dominant value in a group and how many distinct values it holds. A count above 1
 * is the signal a skew matrix exists to show: this cell does not agree with itself.
 */
export function dominantValue(
  rows: Row[],
  field: string,
): { label: string | undefined; distinct: number } {
  const counts = new Map<string, number>();
  for (const r of rows) {
    const v = r.values[field];
    if (v === null || v === undefined || v === '') continue;
    const key = String(v);
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  if (counts.size === 0) return { label: undefined, distinct: 0 };

  let best = '';
  let bestCount = -1;
  for (const [value, count] of counts) {
    // Ties break on the lexically smaller value so the same data always renders the
    // same cell — a matrix that shuffles on refresh cannot be compared.
    if (count > bestCount || (count === bestCount && value < best)) {
      best = value;
      bestCount = count;
    }
  }
  return { label: best, distinct: counts.size };
}

export function applyAggregate(rows: Row[], fn: AggregateFn, field?: string): number {
  if (fn === 'count') return rows.length;

  if (fn === 'distinctCount') {
    const seen = new Set<RowValue>();
    for (const r of rows) seen.add(field ? r.values[field] : r.id);
    return seen.size;
  }

  // The measure is "how many distinct values does this group hold"; the value itself
  // rides along as the point's label.
  if (fn === 'value') return field ? dominantValue(rows, field).distinct : 0;

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

/** A round bucket width for a numeric range, so histogram edges read as numbers. */
export function bucketSize(values: number[]): number {
  if (values.length === 0) return 1;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  if (span <= 0) return 1;

  // Aim for roughly a dozen buckets, then round up to 1/2/5 × a power of ten.
  const rough = span / 12;
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  let size = 10 * magnitude;
  for (const step of [1, 2, 5, 10]) {
    if (rough <= step * magnitude) {
      size = step * magnitude;
      break;
    }
  }

  // Integer data never gets fractional buckets. Replica counts binned at 0.2 produce
  // "1.0–1.2" — an edge that cannot contain anything and a label that reads as an error.
  const allIntegers = values.every((v) => Number.isInteger(v));
  return allIntegers ? Math.max(1, Math.round(size)) : size;
}

/** Label for the bucket a value falls in, e.g. `10–20`. */
export function bucketLabel(value: number, size: number): string {
  const lower = Math.floor(value / size) * size;
  const upper = lower + size;
  const fmt = (n: number) => (Number.isInteger(n) ? String(n) : n.toFixed(2));
  return `${fmt(lower)}–${fmt(upper)}`;
}

function keyOf(
  row: Row,
  field: string,
  bin?: Transform['bin'],
  numericBin?: { field: string; size: number },
): string | null {
  const raw = row.values[field];
  if (raw === null || raw === undefined || raw === '') return null;
  if (bin && bin.field === field) return binTimestamp(String(raw), bin.unit);
  if (numericBin && numericBin.field === field) {
    const n = numeric(raw);
    return n === null ? null : bucketLabel(n, numericBin.size);
  }
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
 * Adds derived dimensions to each row. `coalesce` picks the first non-empty source, so a
 * concept that different providers spell differently becomes one dimension.
 */
export function applyDerive(rows: Row[], derive: Transform['derive']): Row[] {
  if (!derive || derive.length === 0) return rows;
  return rows.map((row) => {
    const values = { ...row.values };
    for (const { name, coalesce } of derive) {
      let picked: RowValue = null;
      for (const field of coalesce) {
        const v = values[field];
        if (v !== null && v !== undefined && v !== '') {
          picked = v;
          break;
        }
      }
      values[name] = picked;
    }
    return { ...row, values };
  });
}

/**
 * Rows -> Frame. With no `groupBy` the whole set is one point, which is what a stat
 * tile wants; with two keys the second becomes the series dimension (stacked bars,
 * multi-line).
 */
export function aggregate(inputRows: Row[], transform: Transform | undefined): Frame {
  const t = transform ?? {};
  const rows = applyDerive(inputRows, t.derive);
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

  // Bucket width is derived from the data, once, so every row lands in the same grid.
  const numericBin = t.numericBin
    ? {
        field: t.numericBin.field,
        size:
          t.numericBin.size ??
          bucketSize(
            rows
              .map((r) => numeric(r.values[t.numericBin!.field]))
              .filter((n): n is number => n !== null),
          ),
      }
    : undefined;

  const buckets = new Map<string, Map<string, Row[]>>();
  let excluded = 0;

  for (const row of rows) {
    const rawKey = keyOf(row, primary, t.bin, numericBin);
    if (rawKey === null) {
      if (t.dropEmpty) {
        excluded++;
        continue;
      }
    }
    const key = rawKey ?? NONE;
    const seriesKey = secondary ? (keyOf(row, secondary, t.bin, numericBin) ?? NONE) : 'value';

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
  // A histogram axis is ordered by bucket lower bound, numerically — string sort would
  // put "100–200" before "20–30".
  if (numericBin) {
    const lower = (k: string) => Number.parseFloat(k.split('–')[0]);
    categories = [...categories].sort((a, b) => {
      if (a === NONE) return 1;
      if (b === NONE) return -1;
      return lower(a) - lower(b);
    });
  }

  const seriesNames = secondary
    ? [...new Set([...buckets.values()].flatMap((m) => [...m.keys()]))].sort((a, b) =>
        a.localeCompare(b),
      )
    : ['value'];

  const series: Series[] = seriesNames.map((name) => ({
    name,
    points: categories.map((category): Point => {
      const rowsFor = buckets.get(category)?.get(name) ?? [];
      const point: Point = {
        key: category,
        value: applyAggregate(rowsFor, fn, field),
        rows: rowsFor,
      };
      if (fn === 'value' && field) point.label = dominantValue(rowsFor, field).label;
      return point;
    }),
  }));

  const total = series.reduce((sum, s) => sum + s.points.reduce((a, p) => a + p.value, 0), 0);
  return { categories, series, total, excluded, omittedCategories };
}

/** Single headline number, for stat tiles and meters. */
export function scalar(frame: Frame): number {
  return frame.total;
}
