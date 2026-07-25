import { describe, expect, it } from 'vitest';

import type { Row } from '../model/types';
import {
  type CrossFilter,
  addCrossFilter,
  applicableFilters,
  applyCrossFilters,
  removeCrossFilter,
} from './crossFilter';

const f = (field: string, value: string): CrossFilter => ({
  field,
  value,
  label: `${field}: ${value}`,
});

const row = (id: string, values: Record<string, string | null>): Row => ({ id, values });

describe('addCrossFilter', () => {
  it('adds a filter', () => {
    expect(addCrossFilter([], f('Resource.Kind', 'Deployment'))).toHaveLength(1);
  });

  it('is idempotent for the same mark', () => {
    const one = addCrossFilter([], f('Resource.Kind', 'Deployment'));
    expect(addCrossFilter(one, f('Resource.Kind', 'Deployment'))).toEqual(one);
  });

  it('replaces rather than ANDs a second value of the same dimension', () => {
    // "Kind = Deployment AND Kind = Service" matches nothing, which is never what
    // clicking a second bar means.
    const filters = addCrossFilter(
      addCrossFilter([], f('Resource.Kind', 'Deployment')),
      f('Resource.Kind', 'Service'),
    );
    expect(filters).toHaveLength(1);
    expect(filters[0].value).toBe('Service');
  });

  it('keeps filters on different dimensions', () => {
    const filters = addCrossFilter(
      addCrossFilter([], f('Resource.Kind', 'Deployment')),
      f('Space.Slug', 'prod-platform'),
    );
    expect(filters).toHaveLength(2);
  });
});

describe('removeCrossFilter', () => {
  it('removes only the named filter', () => {
    const filters = [f('Resource.Kind', 'Deployment'), f('Space.Slug', 'prod')];
    expect(removeCrossFilter(filters, filters[0])).toEqual([filters[1]]);
  });
});

describe('applicableFilters', () => {
  it('keeps filters the source has a dimension for', () => {
    expect(applicableFilters([f('Space.Slug', 'prod')], 'Unit')).toHaveLength(1);
  });

  it('drops filters the source cannot answer', () => {
    // Clicking a resource kind must not blank the Space-grain panels beside it.
    expect(applicableFilters([f('Resource.Kind', 'Deployment')], 'Space')).toHaveLength(0);
  });

  it('recognizes label dimensions by prefix', () => {
    expect(applicableFilters([f('Space.Labels.Environment', 'prod')], 'Unit')).toHaveLength(1);
  });
});

describe('applyCrossFilters', () => {
  const rows = [
    row('1', { kind: 'Deployment', space: 'prod' }),
    row('2', { kind: 'Service', space: 'prod' }),
    row('3', { kind: 'Deployment', space: 'dev' }),
    row('4', { kind: null, space: 'dev' }),
  ];

  it('returns everything when there are no filters', () => {
    expect(applyCrossFilters(rows, [])).toHaveLength(4);
  });

  it('ANDs filters across dimensions', () => {
    const out = applyCrossFilters(rows, [f('kind', 'Deployment'), f('space', 'prod')]);
    expect(out.map((r) => r.id)).toEqual(['1']);
  });

  it('does not match rows missing the dimension', () => {
    expect(applyCrossFilters(rows, [f('kind', 'Deployment')]).map((r) => r.id)).toEqual(['1', '3']);
  });

  it('compares numbers as their string form, matching the chart key', () => {
    const numeric = [row('a', { n: '3' }), row('b', { n: '4' })];
    expect(applyCrossFilters(numeric, [f('n', '3')]).map((r) => r.id)).toEqual(['a']);
  });
});
