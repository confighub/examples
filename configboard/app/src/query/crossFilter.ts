// Cross-filtering: clicking a mark narrows the whole dashboard.
//
// Filters are applied client-side, after the fetch. That is deliberate. Many chart
// dimensions are derived rather than stored — `Unit.ReleaseState`, `Unit.Deployable`,
// every `Resource.*` — so there is no `where` clause that could express them, and
// pushing only *some* clicks down would make the same gesture mean different things.
// Uniform client-side filtering keeps a click honest at the cost of not narrowing the
// query, and the row budget already bounds what a query returns.

import type { Row } from '../model/types';
import { lookupDimension } from './dimensions';
import type { SourceName } from '../model/types';

export interface CrossFilter {
  /** Dimension id, e.g. `Resource.Kind`. */
  field: string;
  /** The clicked category value. */
  value: string;
  /** Human label for the chip. */
  label: string;
}

export function crossFilterKey(filter: CrossFilter): string {
  return `${filter.field}=${filter.value}`;
}

export function addCrossFilter(filters: CrossFilter[], filter: CrossFilter): CrossFilter[] {
  const key = crossFilterKey(filter);
  // Clicking the same mark twice is a no-op, not a duplicate chip. Clicking a different
  // value of the *same* dimension replaces it: "prod, and also dev" is not what the
  // gesture means, and an AND of two values of one dimension matches nothing.
  const withoutSameField = filters.filter((f) => f.field !== filter.field);
  if (filters.some((f) => crossFilterKey(f) === key)) return filters;
  return [...withoutSameField, filter];
}

export function removeCrossFilter(filters: CrossFilter[], filter: CrossFilter): CrossFilter[] {
  const key = crossFilterKey(filter);
  return filters.filter((f) => crossFilterKey(f) !== key);
}

/**
 * The filters that apply to a given source. A filter on a dimension the source does not
 * have is ignored for that panel rather than emptying it — clicking a resource kind
 * should not blank the Space-grain panels beside it.
 */
export function applicableFilters(filters: CrossFilter[], source: SourceName): CrossFilter[] {
  return filters.filter((f) => lookupDimension(source, f.field) !== undefined);
}

export function applyCrossFilters(rows: Row[], filters: CrossFilter[]): Row[] {
  if (filters.length === 0) return rows;
  return rows.filter((row) =>
    filters.every((f) => {
      const value = row.values[f.field];
      // A row with no value for the dimension is not a match — the same reading the
      // "(none)" bucket gets in the chart it was clicked from.
      return value !== null && value !== undefined && String(value) === f.value;
    }),
  );
}
