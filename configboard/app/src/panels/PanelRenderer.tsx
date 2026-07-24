import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import { useMemo } from 'react';

import { BarChart } from '../charts/BarChart';
import { FrameTable, RowTable } from '../charts/DataTable';
import { DonutChart } from '../charts/DonutChart';
import { LineChart } from '../charts/LineChart';
import { Meter, StatTile } from '../charts/StatTile';
import type { Frame, Panel, Row } from '../model/types';
import { aggregate } from '../query/aggregate';
import { ALL_VALUE, type Scope, compilePanel, cubCommand, unknownDimensions } from '../query/compile';
import { lookupDimension } from '../query/dimensions';
import { HARD_ROW_LIMIT, SOFT_ROW_LIMIT, useQueryResult } from '../query/execute';
import { PanelFrame } from './PanelFrame';

export interface PanelRendererProps {
  panel: Panel;
  scope: Scope;
  baseUrl: string;
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

function renderChart(panel: Panel, frame: Frame) {
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
      return <DonutChart frame={frame} spec={chart} />;
    case 'line':
      return <LineChart frame={frame} spec={chart} />;
    case 'stackedBar':
      return <BarChart frame={frame} spec={chart} stacked />;
    case 'bar':
      return <BarChart frame={frame} spec={chart} />;
    case 'table':
      return null;
    default:
      return null;
  }
}

export function PanelRenderer({ panel, scope, baseUrl }: PanelRendererProps) {
  const spec = useMemo(() => compilePanel(panel, scope), [panel, scope]);
  const result = useQueryResult(spec, baseUrl);

  const { kept, note: excludeNote } = useMemo(
    () => applyExcludes(panel, result.rows),
    [panel, result.rows],
  );
  const frame = useMemo(() => aggregate(kept, panel.transform), [kept, panel.transform]);

  const notes: string[] = [];
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
    renderChart(panel, frame)
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
    />
  );
}
