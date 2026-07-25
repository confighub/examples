import { describe, expect, it } from 'vitest';

import type { Row } from '../model/types';
import { aggregate, applyDerive, bucketLabel, bucketSize, dominantValue } from './aggregate';

const row = (id: string, values: Record<string, string | number | null>): Row => ({ id, values });

describe('applyDerive with coalesce', () => {
  // Crossplane, ACK-on-spec, and ACK-on-annotation each spell the same concept
  // differently. One derived dimension has to cover all three.
  const derive = [
    { name: 'Derived.Region', coalesce: ['View.Region', 'View.AckRegion', 'View.AnnoRegion'] },
  ];

  it('takes the first non-empty source in order', () => {
    const [out] = applyDerive(
      [row('1', { 'View.Region': 'us-east-1', 'View.AckRegion': 'eu-west-1' })],
      derive,
    );
    expect(out.values['Derived.Region']).toBe('us-east-1');
  });

  it('falls through empty and null sources', () => {
    const [out] = applyDerive(
      [row('1', { 'View.Region': null, 'View.AckRegion': '', 'View.AnnoRegion': 'us-west-2' })],
      derive,
    );
    expect(out.values['Derived.Region']).toBe('us-west-2');
  });

  it('yields null when no source has a value', () => {
    const [out] = applyDerive([row('1', { 'View.Region': null })], derive);
    expect(out.values['Derived.Region']).toBeNull();
  });

  it('does not mutate the input rows', () => {
    const rows = [row('1', { 'View.Region': 'us-east-1' })];
    applyDerive(rows, derive);
    expect(rows[0].values['Derived.Region']).toBeUndefined();
  });

  it('is a no-op without a derive spec', () => {
    const rows = [row('1', { a: 1 })];
    expect(applyDerive(rows, undefined)).toBe(rows);
  });

  it('groups by the derived dimension end to end', () => {
    const rows = [
      row('1', { 'View.Region': 'us-east-1' }),
      row('2', { 'View.AckRegion': 'us-east-1' }),
      row('3', { 'View.AnnoRegion': 'us-west-2' }),
      row('4', {}),
    ];
    const frame = aggregate(rows, {
      derive,
      groupBy: 'Derived.Region',
      aggregate: { fn: 'count' },
      dropEmpty: true,
    });
    expect(frame.categories).toEqual(['us-east-1', 'us-west-2']);
    expect(frame.series[0].points[0].value).toBe(2);
    expect(frame.excluded).toBe(1);
  });
});

describe('dominantValue', () => {
  it('returns the most common value and how many distinct ones there are', () => {
    const rows = [
      row('1', { img: 'app:v2' }),
      row('2', { img: 'app:v2' }),
      row('3', { img: 'app:v1' }),
    ];
    expect(dominantValue(rows, 'img')).toEqual({ label: 'app:v2', distinct: 2 });
  });

  it('breaks ties deterministically, so a matrix does not shuffle between renders', () => {
    const a = dominantValue([row('1', { v: 'b' }), row('2', { v: 'a' })], 'v');
    const b = dominantValue([row('1', { v: 'a' }), row('2', { v: 'b' })], 'v');
    expect(a).toEqual(b);
    expect(a.label).toBe('a');
  });

  it('ignores empty values', () => {
    expect(dominantValue([row('1', { v: null }), row('2', { v: '' })], 'v')).toEqual({
      label: undefined,
      distinct: 0,
    });
  });
});

describe('numeric binning', () => {
  it('picks a round bucket width', () => {
    expect(bucketSize([0, 100])).toBe(10);
    expect(bucketSize([])).toBe(1);
    expect(bucketSize([5, 5])).toBe(1);
  });

  it('never gives integer data a fractional bucket', () => {
    // Replica counts binned at 0.2 produce "1.0–1.2": an edge nothing can fall in.
    expect(bucketSize([1, 2, 3])).toBe(1);
    expect(bucketSize([1, 1, 2, 5])).toBe(1);
    for (const values of [[1, 2], [0, 3], [2, 9], [1, 11]]) {
      expect(Number.isInteger(bucketSize(values)), String(values)).toBe(true);
    }
  });

  it('still allows fractional buckets for fractional data', () => {
    // Lead time in hours is genuinely continuous.
    expect(bucketSize([0.1, 0.2, 0.35])).toBeLessThan(1);
  });

  it('labels a bucket by its bounds', () => {
    expect(bucketLabel(3, 1)).toBe('3–4');
    expect(bucketLabel(37, 10)).toBe('30–40');
  });

  it('orders histogram buckets numerically, not lexically', () => {
    const rows = [
      row('1', { n: 5 }),
      row('2', { n: 25 }),
      row('3', { n: 105 }),
    ];
    const frame = aggregate(rows, {
      numericBin: { field: 'n', size: 10 },
      groupBy: 'n',
      aggregate: { fn: 'count' },
    });
    // Lexical order would put "100–110" before "20–30".
    expect(frame.categories).toEqual(['0–10', '20–30', '100–110']);
  });
});
