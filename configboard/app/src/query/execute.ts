// Runs a compiled RequestSpec. RTK Query supplies caching and request dedup, so two
// panels compiling to the same request share one fetch — which is how a KPI row of
// seven tiles costs zero extra calls over the Space fetch the dashboard already made.

import {
  useListAllRevisionsQuery,
  useListAllTargetsQuery,
  useListAllUnitsQuery,
  useListAllViewsQuery,
  useListSpacesQuery,
} from '@confighub/rtk-query';
import { useMemo } from 'react';

import type { Row } from '../model/types';
import type { RequestSpec } from './compile';
import { resourceRows, useResourceListQuery } from './resources';
import { findingRows, revisionRow, spaceRow, targetRow, unitRow } from './rows';

/** Config changes on human timescales — a minute of staleness is not a lie. */
export const STALE_SECONDS = 60;

/**
 * `GET /unit` has no pagination: a query returns its whole result set. These bounds
 * turn that into a stated limit rather than a stalled tab.
 */
export const SOFT_ROW_LIMIT = 5_000;
export const HARD_ROW_LIMIT = 25_000;

export interface QueryResult {
  rows: Row[];
  /** True when the panel is waiting to be run rather than loading. */
  held: boolean;
  isLoading: boolean;
  isFetching: boolean;
  error?: string;
  /** True when the result set is large enough to be worth narrowing. */
  overSoftLimit: boolean;
  /** True when the result set was refused. `rows` is empty in that case. */
  overHardLimit: boolean;
}

function errorMessage(error: unknown): string | undefined {
  if (!error) return undefined;
  if (typeof error === 'object' && error !== null) {
    const e = error as { status?: number | string; data?: unknown; error?: string };
    if (typeof e.data === 'object' && e.data !== null) {
      const d = e.data as { Message?: string; message?: string };
      if (d.Message || d.message) return d.Message ?? d.message;
    }
    if (e.error) return e.error;
    if (e.status) return `HTTP ${e.status}`;
  }
  return 'Request failed';
}

/**
 * One hook per source, all four called unconditionally and three of them skipped —
 * hooks cannot be called conditionally, and `skip` is RTK Query's supported way to
 * express "not this one".
 */
export function useQueryResult(
  spec: RequestSpec,
  baseUrl: string,
  /** True once the user has asked a `manual` panel to run. */
  runRequested = false,
): QueryResult {
  // A manual panel stays idle until asked. Its cost is seconds of someone else's
  // server time, which is not a thing to spend on a tab someone happened to open.
  const held = Boolean(spec.manual) && !runRequested;
  const common = { where: spec.where, include: spec.include, filter: spec.filter };

  // A panel names a View as `space/slug`, but the org-wide unit list resolves `view`
  // only as a UUID — a slug has no space to be resolved in. One cheap lookup, cached
  // like everything else, and the unit query waits for it.
  const viewSlug = spec.view?.includes('/') ? spec.view.split('/').pop() : spec.view;
  const viewLookup = useListAllViewsQuery(
    { where: `Slug = '${viewSlug ?? ''}'` },
    { skip: !viewSlug },
  );
  const viewId = viewLookup.data?.[0]?.View?.ViewID;
  const viewPending = Boolean(viewSlug) && !viewId;
  const viewMissing = Boolean(viewSlug) && !viewLookup.isFetching && !viewId && viewLookup.isSuccess;

  const units = useListAllUnitsQuery(
    {
      ...common,
      select: spec.select,
      view: viewId,
      resourceType: spec.resourceType,
      whereTrigger: spec.triggerWhere,
      triggersPassed: spec.triggersPassed,
    },
    // Running the unit query before the View resolves would return unprojected rows and
    // the panel would render an empty dimension rather than waiting.
    { skip: (spec.source !== 'Unit' && spec.source !== 'Finding') || viewPending || held },
  );
  const spaces = useListSpacesQuery(
    { ...common, summary: spec.summary },
    { skip: spec.source !== 'Space' || held },
  );
  const revisions = useListAllRevisionsQuery({ ...common }, { skip: spec.source !== 'Revision' || held });
  const targets = useListAllTargetsQuery({ ...common }, { skip: spec.source !== 'Target' || held });

  const isResource = spec.source === 'Resource';
  const resources = useResourceListQuery(
    { where: spec.where, filter: spec.filter, resourceType: spec.resourceType },
    { skip: !isResource || held },
  );
  // A resource row knows its TargetID but not the Target's slug; the Target list is
  // small and already cached for the scope bar, so the lookup is nearly free.
  const targetsForNames = useListAllTargetsQuery({}, { skip: !isResource });

  const active =
    spec.source === 'Unit' || spec.source === 'Finding'
      ? units
      : spec.source === 'Space'
        ? spaces
        : spec.source === 'Revision'
          ? revisions
          : spec.source === 'Resource'
            ? resources
            : targets;

  const targetSlugs = useMemo((): Record<string, string> => {
    const map: Record<string, string> = {};
    for (const t of targetsForNames.data ?? []) {
      if (t.Target?.TargetID && t.Target.Slug) map[t.Target.TargetID] = t.Target.Slug;
    }
    return map;
  }, [targetsForNames.data]);

  const rows = useMemo((): Row[] => {
    switch (spec.source) {
      case 'Unit':
        return (units.data ?? []).map((u) => unitRow(u, baseUrl));
      case 'Space':
        return (spaces.data ?? []).map(spaceRow);
      case 'Revision':
        return (revisions.data ?? []).map(revisionRow);
      case 'Target':
        return (targets.data ?? []).map(targetRow);
      case 'Finding':
        return (units.data ?? []).flatMap((u) => findingRows(u, baseUrl));
      case 'Resource':
        return resourceRows(resources.data ?? [], targetSlugs, baseUrl);
      default:
        return [];
    }
  }, [
    spec.source,
    units.data,
    spaces.data,
    revisions.data,
    targets.data,
    resources.data,
    targetSlugs,
    baseUrl,
  ]);

  const overHardLimit = rows.length > HARD_ROW_LIMIT;
  return {
    rows: overHardLimit ? [] : rows,
    held,
    isLoading: active.isLoading || (viewPending && !viewMissing),
    isFetching: active.isFetching || viewLookup.isFetching,
    error:
      viewMissing && spec.view
        ? `View "${spec.view}" not found — seed the configboard Views, or fix the panel's view reference.`
        : errorMessage(active.error) ?? errorMessage(viewLookup.error),
    overSoftLimit: rows.length > SOFT_ROW_LIMIT && !overHardLimit,
    overHardLimit,
  };
}
