import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import { useMemo, useState } from 'react';

import { BarChart } from '../charts/BarChart';
import { FrameTable, RowTable } from '../charts/DataTable';
import { DonutChart } from '../charts/DonutChart';
import { Heatmap } from '../charts/Heatmap';
import { LineChart } from '../charts/LineChart';
import { Meter, StatTile } from '../charts/StatTile';
import type { Frame, Panel, Row } from '../model/types';
import { NONE, OTHER, aggregate } from '../query/aggregate';
import { ALL_VALUE, type Scope, compilePanel, cubCommand, unknownDimensions } from '../query/compile';
import {
  type CrossFilter,
  applicableFilters,
  applyCrossFilters,
} from '../query/crossFilter';
import { lookupDimension } from '../query/dimensions';
import { HARD_ROW_LIMIT, SOFT_ROW_LIMIT, useQueryResult } from '../query/execute';
import { useDashboardStorage } from '../storage/dashboards';
import { canSaveAsFilter, useFilterStorage } from '../storage/filters';
import { PanelFrame } from './PanelFrame';

export interface PanelRendererProps {
  panel: Panel;
  scope: Scope;
  baseUrl: string;
  crossFilters?: CrossFilter[];
  onCrossFilter?: (filter: CrossFilter) => void;
}

/** Below this, a whole-org resource scan is fast enough not to nag about. */
const RESOURCE_HINT_THRESHOLD = 500;

/** Applies a panel's `excludes` clause client-side and reports what it removed. */
function applyExcludes(panel: Panel, rows: Row[]): { kept: Row[]; note?: string } {
  const excludes = panel.query.excludes;
  if (!excludes) return { kept: rows };

  const kept = rows.filter((r) => {
    const value = r.values[excludes.field];
    return excludes.isNull ? value !== null && value !== undefined && value !== '' : true;
  });
  const removed = rows.length - kept.length;
  return {
    kept,
    note: removed > 0 ? `${removed} ${excludes.label}` : undefined,
  };
}

function dimensionLabel(panel: Panel): string {
  const groupBy = panel.transform?.groupBy;
  const id = typeof groupBy === 'string' ? groupBy : groupBy?.[0];
  if (!id) return 'Category';
  return lookupDimension(panel.query.source, id)?.label ?? id;
}

/** Columns for the drill-down table: the grouped dimension plus useful identity. */
function drillColumns(panel: Panel): { id: string; label: string }[] {
  const source = panel.query.source;
  const identity =
    source === 'Unit'
      ? [
          { id: 'Unit.Slug', label: 'Unit' },
          { id: 'Space.Slug', label: 'Space' },
        ]
      : source === 'Space'
        ? [{ id: 'Space.Slug', label: 'Space' }]
        : source === 'Revision'
          ? [
              { id: 'Unit.Slug', label: 'Unit' },
              { id: 'Revision.Num', label: 'Rev' },
              { id: 'Revision.CreatedAt', label: 'Created' },
            ]
          : source === 'Resource'
            ? [
                { id: 'Resource.Name', label: 'Resource' },
                { id: 'Resource.Type', label: 'Type' },
                { id: 'Resource.Scope', label: 'Scope' },
                { id: 'Unit.Slug', label: 'Unit' },
              ]
            : [{ id: 'Target.Slug', label: 'Target' }];

  const groupBy = panel.transform?.groupBy;
  const grouped = typeof groupBy === 'string' ? [groupBy] : (groupBy ?? []);
  const extra = grouped
    .filter((id) => !identity.some((c) => c.id === id))
    .map((id) => ({ id, label: lookupDimension(panel.query.source, id)?.label ?? id }));

  return [...identity, ...extra];
}

function renderChart(panel: Panel, frame: Frame, onSelect?: (c: string, s?: string) => void) {
  const { chart } = panel;
  switch (chart.form) {
    case 'statTile': {
      const level = chart.color === 'status' && frame.total > 0 ? 'critical' : undefined;
      return <StatTile value={frame.total} unit={chart.unit} level={level} />;
    }
    case 'meter': {
      // The meter's denominator is a second measure over the same rows.
      const rows = frame.series[0]?.points[0]?.rows ?? [];
      const total = chart.totalField
        ? rows.reduce((sum, r) => sum + (Number(r.values[chart.totalField!]) || 0), 0)
        : frame.total;
      return <Meter value={frame.total} total={total} invert={chart.invert} />;
    }
    case 'donut':
      return <DonutChart frame={frame} spec={chart} onSelect={onSelect} />;
    case 'line':
      // A time axis is a scale, not a set of categories — clicking one day to filter the
      // dashboard to that day is rarely what anyone means, so lines do not cross-filter.
      return <LineChart frame={frame} spec={chart} />;
    case 'stackedBar':
      return <BarChart frame={frame} spec={chart} stacked onSelect={onSelect} />;
    case 'bar':
      return <BarChart frame={frame} spec={chart} onSelect={onSelect} />;
    case 'histogram':
      // A histogram is a bar chart over ordered buckets: bars touch conceptually, and
      // the axis is a scale, so it never reorders by magnitude.
      return <BarChart frame={frame} spec={{ ...chart, orientation: 'vertical' }} />;
    case 'heatmap':
      return (
        <Heatmap
          frame={frame}
          spec={chart}
          dimensionLabel={dimensionLabel(panel)}
          onSelect={onSelect}
        />
      );
    case 'table':
      return null;
    default:
      return null;
  }
}

export function PanelRenderer({
  panel,
  scope,
  baseUrl,
  crossFilters = [],
  onCrossFilter,
}: PanelRendererProps) {
  const spec = useMemo(() => compilePanel(panel, scope), [panel, scope]);
  const [runRequested, setRunRequested] = useState(false);
  const [savedFilter, setSavedFilter] = useState<string | undefined>();
  const filters = useFilterStorage();
  const storage = useDashboardStorage();
  const result = useQueryResult(spec, baseUrl, runRequested);

  const { kept: afterExcludes, note: excludeNote } = useMemo(
    () => applyExcludes(panel, result.rows),
    [panel, result.rows],
  );

  // Only filters on dimensions this source actually has: a resource-kind chip must not
  // blank the Space-grain panels beside it.
  const active = useMemo(
    () => applicableFilters(crossFilters, panel.query.source),
    [crossFilters, panel.query.source],
  );
  const kept = useMemo(() => applyCrossFilters(afterExcludes, active), [afterExcludes, active]);

  const frame = useMemo(() => aggregate(kept, panel.transform), [kept, panel.transform]);

  const groupKeys = panel.transform?.groupBy;
  const primaryKey = typeof groupKeys === 'string' ? groupKeys : groupKeys?.[0];
  const secondaryKey = Array.isArray(groupKeys) ? groupKeys[1] : undefined;

  const onSelect = useMemo(() => {
    if (!onCrossFilter || !primaryKey) return undefined;
    return (category: string, series?: string) => {
      // Clicking a stack segment filters on the series dimension when there is one:
      // the segment's identity is the series, and the category is already the axis.
      const field = series && secondaryKey ? secondaryKey : primaryKey;
      const value = series && secondaryKey ? series : category;
      if (value === OTHER || value === NONE) return; // a residue bucket is not a value
      onCrossFilter({
        field,
        value,
        label: `${lookupDimension(panel.query.source, field)?.label ?? field}: ${value}`,
      });
    };
  }, [onCrossFilter, primaryKey, secondaryKey, panel.query.source]);

  // Promoting a panel's `where` to a Filter makes the query an org entity: usable from
  // `cub unit list --filter`, a bulk patch, or a Trigger's scope.
  const saveAsFilter =
    canSaveAsFilter(panel.query.source, spec.where) && !savedFilter
      ? async () => {
          try {
            const spaceId = await storage.ensureSpace();
            const slug = `cb-${panel.id}`.slice(0, 60);
            await filters.save({
              spaceId,
              slug,
              source: panel.query.source,
              where: spec.where!,
              resourceType: spec.resourceType,
            });
            setSavedFilter(slug);
          } catch (e) {
            setSavedFilter(`error: ${e instanceof Error ? e.message : String(e)}`);
          }
        }
      : undefined;

  const notes: string[] = [];
  if (savedFilter) {
    notes.push(
      savedFilter.startsWith('error:')
        ? `Save as Filter ${savedFilter}`
        : `Saved as Filter configboard/${savedFilter} — usable with cub unit list --filter.`,
    );
  }
  if (excludeNote) notes.push(`Excluded: ${excludeNote}.`);
  if (frame.excluded > 0) notes.push(`${frame.excluded} rows had no value for this dimension.`);
  if (frame.omittedCategories > 0) {
    const label = dimensionLabel(panel).toLowerCase();
    notes.push(`${frame.omittedCategories} more ${label} values not shown.`);
  }
  if (result.overHardLimit) {
    notes.push(
      `Result set exceeds ${HARD_ROW_LIMIT.toLocaleString()} rows and was not loaded — narrow the scope.`,
    );
  } else if (result.overSoftLimit) {
    notes.push(
      `${result.rows.length.toLocaleString()} rows (over ${SOFT_ROW_LIMIT.toLocaleString()}) — consider narrowing the scope.`,
    );
  }

  const unknown = unknownDimensions(panel);
  if (unknown.length > 0) notes.push(`Unknown dimension(s): ${unknown.join(', ')}.`);

  // A chip that this panel cannot honour is stated, so a panel showing unfiltered
  // numbers next to filtered ones is never a mystery.
  const ignored = crossFilters.length - active.length;
  if (ignored > 0) {
    notes.push(
      `${ignored} dashboard filter${ignored === 1 ? '' : 's'} not applicable to ${panel.query.source} rows.`,
    );
  }

  // Resource counting invokes a function per Unit, and the invoke response carries each
  // Unit's config data whether or not the function asked for it — org-wide that is tens
  // of megabytes. Every resource panel shares one invocation, so the advice belongs
  // where it is actionable: only when the scope is still wide open. Repeating it under
  // eleven panels is noise, not information.
  const scopeIsWideOpen = Object.values(scope).every((v) => v === undefined || v === ALL_VALUE);
  if (
    panel.query.source === 'Resource' &&
    !result.isLoading &&
    kept.length > RESOURCE_HINT_THRESHOLD &&
    scopeIsWideOpen
  ) {
    notes.push('Whole-org resource scan — pick an Environment or Cluster to speed this up.');
  }

  const isEmpty = !result.isLoading && !result.error && kept.length === 0;

  const chart = isEmpty ? (
    <Box sx={{ py: 5, textAlign: 'center' }}>
      <Typography variant="body2" color="text.secondary">
        No matching rows.
      </Typography>
    </Box>
  ) : (
    renderChart(panel, frame, onSelect)
  );

  const table =
    panel.chart.form === 'statTile' || panel.chart.form === 'meter' ? (
      <RowTable rows={kept} columns={drillColumns(panel)} />
    ) : (
      <FrameTable frame={frame} dimensionLabel={dimensionLabel(panel)} />
    );

  return (
    <PanelFrame
      title={panel.title}
      description={panel.description}
      query={cubCommand(spec)}
      isLoading={result.isLoading}
      isFetching={result.isFetching}
      error={result.error}
      notes={notes}
      chart={chart}
      table={table}
      onSaveAsFilter={saveAsFilter ? () => void saveAsFilter() : undefined}
      held={
        result.held
          ? {
              label:
                panel.query.triggerWhere
                  ? 'This runs a validator against every candidate Unit on the server — about 20 seconds.'
                  : 'This query is expensive enough to be worth asking for.',
              onRun: () => setRunRequested(true),
            }
          : undefined
      }
    />
  );
}
