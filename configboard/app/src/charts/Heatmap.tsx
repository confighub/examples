import Box from '@mui/material/Box';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';

import type { ChartSpec, Frame, Point } from '../model/types';
import { formatCategory, formatValue, useMode } from './chartTheme';
import { GRID, NEUTRAL, STATUS, SURFACE, sequential } from './palette';

export interface HeatmapProps {
  frame: Frame;
  spec: ChartSpec;
  /** Row-axis label, for the corner cell. */
  dimensionLabel?: string;
  onSelect?: (category: string, series?: string) => void;
}

const ROW_LABEL_WIDTH = 168;
const CELL_GAP = 2;

/**
 * A matrix: rows are the primary group, columns the secondary. Two readings:
 *
 * - **Value cells** (`aggregate: value`) show a string — an image tag, a version — and
 *   are coloured by whether the cell agrees with itself. This is the version-skew
 *   matrix: the question "what is where" answered by looking across a row.
 * - **Count cells** colour by magnitude on the sequential ramp.
 *
 * Built as a CSS grid rather than a charting primitive, so cells are real elements with
 * text in them: the labels are readable, which is what a skew matrix is for.
 */
export function Heatmap({ frame, spec, dimensionLabel = '', onSelect }: HeatmapProps) {
  const mode = useMode();
  const columns = frame.series.map((s) => s.name);
  const rows = frame.categories;
  const isValueMode = frame.series.some((s) => s.points.some((p) => p.label !== undefined));

  const maxValue = Math.max(
    1,
    ...frame.series.flatMap((s) => s.points.map((p) => (isValueMode ? 0 : p.value))),
  );

  const cellFor = (row: string, column: string): Point | undefined =>
    frame.series.find((s) => s.name === column)?.points.find((p) => p.key === row);

  function background(point: Point | undefined): string {
    if (!point || point.value === 0) return 'transparent';
    if (isValueMode) {
      // One value: the cell agrees with itself, and reads as unremarkable. More than
      // one: the group disagrees, which is the finding.
      return point.value > 1 ? STATUS[mode].warning : NEUTRAL[mode];
    }
    const rank = Math.max(0, maxValue - point.value);
    return sequential(rank, maxValue, mode);
  }

  const template = `${ROW_LABEL_WIDTH}px repeat(${columns.length}, minmax(72px, 1fr))`;

  return (
    <Box sx={{ overflowX: 'auto' }}>
      <Box sx={{ minWidth: ROW_LABEL_WIDTH + columns.length * 84 }}>
        {/* Column headers */}
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: template,
            gap: `${CELL_GAP}px`,
            mb: `${CELL_GAP}px`,
            alignItems: 'end',
          }}
        >
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
            {dimensionLabel}
          </Typography>
          {columns.map((column) => (
            <Typography
              key={column}
              variant="caption"
              color="text.secondary"
              sx={{ textAlign: 'center', fontWeight: 600, overflowWrap: 'anywhere' }}
            >
              {formatCategory(column)}
            </Typography>
          ))}
        </Box>

        {rows.map((row) => (
          <Box
            key={row}
            sx={{
              display: 'grid',
              gridTemplateColumns: template,
              gap: `${CELL_GAP}px`,
              mb: `${CELL_GAP}px`,
              alignItems: 'stretch',
            }}
          >
            <Typography
              variant="caption"
              sx={{
                display: 'flex',
                alignItems: 'center',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                pr: 1,
              }}
              title={row}
            >
              {formatCategory(row)}
            </Typography>

            {columns.map((column) => {
              const point = cellFor(row, column);
              const empty = !point || point.value === 0;
              // When a cell holds several values, say so in the cell. Colouring it
              // without saying so shows one tag while implying others exist, and the
              // reader has to hover to find out — a chart should not need a tooltip to
              // avoid misleading.
              const extra = !empty && isValueMode && point!.value > 1 ? ` +${point!.value - 1}` : '';
              const text = isValueMode
                ? `${point?.label ?? ''}${extra}`
                : empty
                  ? ''
                  : formatValue(point.value);

              const detail = empty
                ? `${row} × ${column}: no data`
                : isValueMode
                  ? `${row} × ${column}: ${point?.label}${point!.value > 1 ? ` (+${point!.value - 1} other value${point!.value > 2 ? 's' : ''})` : ''}`
                  : `${row} × ${column}: ${formatValue(point!.value)}`;

              return (
                <Tooltip key={column} title={detail} arrow>
                  <Box
                    onClick={() => {
                      if (!empty && onSelect) onSelect(row, column);
                    }}
                    sx={{
                      minHeight: 30,
                      px: 0.75,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      borderRadius: 1,
                      border: `1px solid ${GRID[mode]}`,
                      bgcolor: background(point),
                      cursor: !empty && onSelect ? 'pointer' : 'default',
                      // A 1px surface ring keeps adjacent filled cells from merging.
                      outline: empty ? 'none' : `1px solid ${SURFACE[mode]}`,
                    }}
                  >
                    <Typography
                      variant="caption"
                      sx={{
                        // Cell text wears an ink token: the fill carries magnitude or
                        // disagreement, the text carries the value.
                        color: empty ? 'text.disabled' : mode === 'dark' ? '#fff' : '#111',
                        fontWeight: 600,
                        fontSize: 11,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {text || (empty ? '—' : '')}
                    </Typography>
                  </Box>
                </Tooltip>
              );
            })}
          </Box>
        ))}

        <Box sx={{ display: 'flex', gap: 2, mt: 1, alignItems: 'center', flexWrap: 'wrap' }}>
          {isValueMode ? (
            <>
              <Legend swatch={NEUTRAL[mode]} label="one distinct value" />
              <Legend swatch={STATUS[mode].warning} label="several (dominant shown, +N others)" />
              <Legend swatch="transparent" label="no data" outlined mode={mode} />
            </>
          ) : (
            <>
              <Typography variant="caption" color="text.secondary">
                {spec.unit ? `fewer ${spec.unit}` : 'fewer'}
              </Typography>
              {[4, 3, 2, 1, 0].map((rank) => (
                <Box
                  key={rank}
                  sx={{
                    width: 14,
                    height: 14,
                    borderRadius: 0.5,
                    bgcolor: sequential(rank, 5, mode),
                  }}
                />
              ))}
              <Typography variant="caption" color="text.secondary">
                {spec.unit ? `more ${spec.unit}` : 'more'}
              </Typography>
            </>
          )}
        </Box>
      </Box>
    </Box>
  );
}

function Legend({
  swatch,
  label,
  outlined,
  mode,
}: {
  swatch: string;
  label: string;
  outlined?: boolean;
  mode?: 'light' | 'dark';
}) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
      <Box
        sx={{
          width: 14,
          height: 14,
          borderRadius: 0.5,
          bgcolor: swatch,
          border: outlined && mode ? `1px solid ${GRID[mode]}` : 'none',
        }}
      />
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
    </Box>
  );
}
