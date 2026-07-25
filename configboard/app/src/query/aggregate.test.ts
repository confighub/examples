import { describe, expect, it } from 'vitest';

import type { Row } from '../model/types';
import { NONE, OTHER, aggregate, applyAggregate, binTimestamp } from './aggregate';

const row = (id: string, values: Record<string, string | number | null>): Row => ({ id, values });

describe('binTimestamp', () => {
  it('buckets to the UTC day', () => {
    expect(binTimestamp('2026-07-24T18:32:11Z', 'day')).toBe('2026-07-24');
  });

  it('buckets to the Monday of the ISO week', () => {
    // 2026-07-24 is a Friday.
    expect(binTimestamp('2026-07-24T00:00:00Z', 'week')).toBe('2026-07-20');
    // A Sunday belongs to the week that started the previous Monday.
    expect(binTimestamp('2026-07-26T23:59:00Z', 'week')).toBe('2026-07-20');
  });

  it('buckets to the first of the month', () => {
    expect(binTimestamp('2026-07-24T00:00:00Z', 'month')).toBe('2026-07-01');
  });

  it('does not throw on unparseable input', () => {
    expect(binTimestamp('not-a-date', 'day')).toBe(NONE);
  });
});

describe('applyAggregate', () => {
  const rows = [
    row('a', { n: 1 }),
    row('b', { n: 3 }),
    row('c', { n: 5 }),
    row('d', { n: null }),
  ];

  it('counts rows regardless of field', () => {
    expect(applyAggregate(rows, 'count')).toBe(4);
  });

  it('ignores non-numeric values in numeric aggregates', () => {
    expect(applyAggregate(rows, 'sum', 'n')).toBe(9);
    expect(applyAggregate(rows, 'avg', 'n')).toBe(3);
    expect(applyAggregate(rows, 'max', 'n')).toBe(5);
  });

  it('coerces numeric strings, which is what Unit.Values holds', () => {
    expect(applyAggregate([row('a', { n: '4' }), row('b', { n: '2' })], 'sum', 'n')).toBe(6);
  });

  it('counts distinct values', () => {
    expect(applyAggregate([row('a', { k: 'x' }), row('b', { k: 'x' }), row('c', { k: 'y' })], 'distinctCount', 'k')).toBe(2);
  });
});

describe('aggregate', () => {
  const rows = [
    row('1', { env: 'prod', kind: 'Deployment' }),
    row('2', { env: 'prod', kind: 'Service' }),
    row('3', { env: 'dev', kind: 'Deployment' }),
    row('4', { env: 'dev', kind: 'Deployment' }),
    row('5', { env: null, kind: 'Service' }),
  ];

  it('returns one point when nothing is grouped', () => {
    const frame = aggregate(rows, {});
    expect(frame.total).toBe(5);
    expect(frame.series).toHaveLength(1);
  });

  it('groups by one key, sorted by value', () => {
    const frame = aggregate(rows, { groupBy: 'kind', aggregate: { fn: 'count' } });
    expect(frame.categories).toEqual(['Deployment', 'Service']);
    expect(frame.series[0].points.map((p) => p.value)).toEqual([3, 2]);
  });

  it('buckets missing values rather than dropping them silently', () => {
    const frame = aggregate(rows, { groupBy: 'env', aggregate: { fn: 'count' } });
    expect(frame.categories).toContain(NONE);
    expect(frame.excluded).toBe(0);
  });

  it('drops and counts missing values when asked', () => {
    const frame = aggregate(rows, { groupBy: 'env', aggregate: { fn: 'count' }, dropEmpty: true });
    expect(frame.categories).not.toContain(NONE);
    expect(frame.excluded).toBe(1);
  });

  it('folds the tail into Other and keeps it last', () => {
    const many = ['a', 'b', 'c', 'd', 'e'].flatMap((k, i) =>
      Array.from({ length: 5 - i }, (_, j) => row(`${k}${j}`, { k })),
    );
    const frame = aggregate(many, { groupBy: 'k', aggregate: { fn: 'count' }, topN: 2 });
    expect(frame.categories).toEqual(['a', 'b', OTHER]);
    // Nothing is lost in the fold: 5+4+3+2+1 = 15.
    expect(frame.total).toBe(15);
    const other = frame.series[0].points.find((p) => p.key === OTHER);
    expect(other?.value).toBe(6);
  });

  it('drops the tail and reports the count when asked', () => {
    // The failure this guards, seen with real data: a "top 10 Spaces" chart summed the
    // other 46 Spaces into "Other", which became the largest bar and flattened the ten
    // the panel existed to show.
    const many = Array.from({ length: 12 }, (_, i) =>
      Array.from({ length: 12 - i }, (_, j) => row(`k${i}-${j}`, { k: `k${i}` })),
    ).flat();

    const folded = aggregate(many, { groupBy: 'k', aggregate: { fn: 'count' }, topN: 3 });
    expect(folded.categories).toContain(OTHER);
    expect(folded.omittedCategories).toBe(0);

    const dropped = aggregate(many, {
      groupBy: 'k',
      aggregate: { fn: 'count' },
      topN: 3,
      tail: 'drop',
    });
    expect(dropped.categories).toEqual(['k0', 'k1', 'k2']);
    expect(dropped.categories).not.toContain(OTHER);
    expect(dropped.omittedCategories).toBe(9);
    // The total reflects only what is charted, so the table and the chart agree.
    expect(dropped.total).toBe(12 + 11 + 10);
  });

  it('produces one series per value of a second key', () => {
    const frame = aggregate(rows, {
      groupBy: ['env', 'kind'],
      aggregate: { fn: 'count' },
    });
    expect(frame.series.map((s) => s.name).sort()).toEqual(['Deployment', 'Service']);
    // Every series carries a point for every category, so stacks line up.
    for (const s of frame.series) {
      expect(s.points.map((p) => p.key)).toEqual(frame.categories);
    }
  });

  it('orders a binned axis by time, not by magnitude', () => {
    const timed = [
      row('1', { t: '2026-07-20T00:00:00Z' }),
      row('2', { t: '2026-07-22T00:00:00Z' }),
      row('3', { t: '2026-07-22T06:00:00Z' }),
      row('4', { t: '2026-07-21T00:00:00Z' }),
    ];
    const frame = aggregate(timed, {
      bin: { field: 't', unit: 'day' },
      groupBy: 't',
      aggregate: { fn: 'count' },
    });
    expect(frame.categories).toEqual(['2026-07-20', '2026-07-21', '2026-07-22']);
  });

  it('never folds a time axis into Other', () => {
    const timed = Array.from({ length: 10 }, (_, i) =>
      row(`${i}`, { t: `2026-07-${String(10 + i).padStart(2, '0')}T00:00:00Z` }),
    );
    const frame = aggregate(timed, {
      bin: { field: 't', unit: 'day' },
      groupBy: 't',
      aggregate: { fn: 'count' },
      topN: 3,
    });
    expect(frame.categories).toHaveLength(10);
    expect(frame.categories).not.toContain(OTHER);
  });
});
