// Panel spec + dashboard scope -> a concrete request. Two jobs: substitute the
// dashboard variables into the `where` expression, and work out which `include`
// joins the panel's dimensions require.

import type { Panel, SourceName, Transform } from '../model/types';
import { includesFor, lookupDimension } from './dimensions';

export interface RequestSpec {
  source: SourceName;
  where?: string;
  include?: string;
  select?: string;
  view?: string;
  filter?: string;
  summary?: boolean;
}

/** Current values of the dashboard's variables, keyed by variable name. */
export type Scope = Record<string, string | undefined>;

/**
 * Fields worth asking for on a Unit list. `Data`, `LiveData`, and `LiveState` carry
 * whole config bodies, so they are never selected — `GET /unit` has no pagination and
 * the payload is the only thing standing between a dashboard and a stalled tab.
 */
const UNIT_SELECT = [
  'Slug',
  'DisplayName',
  'Labels',
  'Values',
  'ToolchainType',
  'ProviderType',
  'TargetID',
  'HeadRevisionNum',
  'LiveRevisionNum',
  'UpstreamRevisionNum',
  'ApplyGates',
  'ApplyWarnings',
  'UpdatedAt',
  'LastChangeDescription',
].join(',');

const TIME_RANGE_MS: Record<string, number> = {
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000,
  '90d': 90 * 24 * 60 * 60 * 1000,
};

/** ISO timestamp for the start of a `${window.start}` reference. */
export function windowStart(range: string | undefined, now = Date.now()): string {
  const span = TIME_RANGE_MS[range ?? '30d'] ?? TIME_RANGE_MS['30d'];
  return new Date(now - span).toISOString().replace(/\.\d+Z$/, 'Z');
}

/**
 * Substitutes `${var}` and `${var.start}` references. A variable set to "All" (or
 * left unset) makes its whole conjunct drop out, so an unscoped dashboard issues an
 * unfiltered query rather than one matching the literal string "All".
 */
export function substitute(where: string, scope: Scope, now = Date.now()): string | undefined {
  const conjuncts = splitConjuncts(where);
  const kept: string[] = [];

  for (const conjunct of conjuncts) {
    const refs = [...conjunct.matchAll(/\$\{([\w.]+)\}/g)];
    if (refs.length === 0) {
      kept.push(conjunct);
      continue;
    }

    let resolved = conjunct;
    let drop = false;
    for (const [token, ref] of refs) {
      const [name, prop] = ref.split('.');
      const value = scope[name];

      // A `.start` reference is a *bound*, and dropping a bound is not a neutral act:
      // it turns "revisions in the last 30 days" into "every revision ever", which on
      // a paginationless endpoint is the one query that can hang the tab. An unset or
      // "All" time variable falls back to the default window instead of dropping.
      if (prop === 'start') {
        resolved = resolved.replace(
          token,
          windowStart(value === ALL_VALUE ? undefined : value, now),
        );
        continue;
      }

      if (value === undefined || value === ALL_VALUE) {
        drop = true;
        break;
      }
      resolved = resolved.replace(token, value);
    }
    if (!drop) kept.push(resolved);
  }

  const expr = kept.join(' AND ').trim();
  return expr.length > 0 ? expr : undefined;
}

export const ALL_VALUE = '*';

/**
 * Splits on top-level ` AND `. Quoted literals may legally contain the word, so the
 * split tracks single-quote state rather than using a bare string split.
 */
function splitConjuncts(where: string): string[] {
  const parts: string[] = [];
  let current = '';
  let inQuote = false;

  for (let i = 0; i < where.length; i++) {
    const ch = where[i];
    if (ch === "'") inQuote = !inQuote;
    if (!inQuote && where.slice(i, i + 5).toUpperCase() === ' AND ') {
      parts.push(current.trim());
      current = '';
      i += 4;
      continue;
    }
    current += ch;
  }
  if (current.trim().length > 0) parts.push(current.trim());
  return parts;
}

/** Every dimension id a panel references, so the compiler knows what to join. */
export function referencedDimensions(transform: Transform | undefined): string[] {
  if (!transform) return [];
  const ids: string[] = [];
  const groupBy = transform.groupBy;
  if (typeof groupBy === 'string') ids.push(groupBy);
  else if (Array.isArray(groupBy)) ids.push(...groupBy);
  if (transform.bin?.field) ids.push(transform.bin.field);
  if (transform.aggregate?.field) ids.push(transform.aggregate.field);
  return ids;
}

export function compilePanel(panel: Panel, scope: Scope, now = Date.now()): RequestSpec {
  const { source, where, view, filter } = panel.query;

  const dimensionIds = referencedDimensions(panel.transform);
  if (panel.query.excludes?.field) dimensionIds.push(panel.query.excludes.field);

  // Filtering across a join does *not* require `include` — the predicate resolves
  // either way. We add it anyway when a panel names a joined entity, because a row
  // that cannot name its own Space or Target is a row the drill-down table cannot
  // show. The cost is the joined entity on each row, which is small next to the
  // config bodies `select` already excludes.
  const whereIncludes = new Set<string>();
  const resolvedWhere = where ? substitute(where, scope, now) : undefined;
  if (resolvedWhere) {
    if (/\bSpace\./.test(resolvedWhere) && source !== 'Space') whereIncludes.add('SpaceID');
    if (/\bTarget\./.test(resolvedWhere) && source !== 'Target') whereIncludes.add('TargetID');
    if (/\bUnit\./.test(resolvedWhere) && source === 'Revision') whereIncludes.add('UnitID');
  }

  for (const inc of includesFor(source, dimensionIds)) whereIncludes.add(inc);

  const spec: RequestSpec = {
    source,
    where: resolvedWhere,
    view,
    filter,
  };
  if (whereIncludes.size > 0) spec.include = [...whereIncludes].join(',');

  if (source === 'Unit') {
    // `select` and `view` are alternatives: a view already dictates the projection,
    // and narrowing the selection under it drops the fields the columns read from.
    if (!view) spec.select = UNIT_SELECT;
  }
  if (source === 'Space') spec.summary = true;

  // A resource query selects Units with `where` and then looks inside them. The
  // joined-entity includes above do not apply: the function response carries its own
  // Unit/Space/Target identity, and `select` is not a parameter of function invoke.
  if (source === 'Resource') {
    spec.include = undefined;
    spec.select = undefined;
  }

  return spec;
}

/** Human-readable `cub` equivalent of a compiled request, for "show me the query". */
export function cubCommand(spec: RequestSpec): string {
  const parts: string[] = [];
  switch (spec.source) {
    case 'Unit':
      parts.push('cub unit list --space "*"');
      break;
    case 'Space':
      parts.push('cub space list');
      break;
    case 'Revision':
      parts.push('cub revision list --space "*"');
      break;
    case 'Target':
      parts.push('cub target list --space "*"');
      break;
    case 'Resource':
      parts.push('cub function do --space "*" -o json -- get-resources --body=none');
      break;
  }
  if (spec.where) parts.push(`--where "${spec.where}"`);
  if (spec.view) parts.push(`--view ${spec.view}`);
  if (spec.filter) parts.push(`--filter ${spec.filter}`);
  return parts.join(' ');
}

/** Stable cache key: panels compiling to the same request share one fetch. */
export function requestKey(spec: RequestSpec): string {
  return JSON.stringify([
    spec.source,
    spec.where ?? '',
    spec.include ?? '',
    spec.select ?? '',
    spec.view ?? '',
    spec.filter ?? '',
    spec.summary ?? false,
  ]);
}

/** Dimensions a panel names that the registry does not recognize. */
export function unknownDimensions(panel: Panel): string[] {
  return referencedDimensions(panel.transform).filter(
    (id) => !lookupDimension(panel.query.source, id),
  );
}
