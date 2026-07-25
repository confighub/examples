import {
  Bar,
  CartesianGrid,
  Cell,
  Legend,
  BarChart as ReBarChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

import type { ChartSpec, Frame } from '../model/types';
import { ChartTooltip } from './ChartTooltip';
import {
  colorFor,
  formatCategory,
  formatValue,
  frameToData,
  useAxisProps,
  useGridStroke,
  useLegendFormatter,
  useMode,
} from './chartTheme';
import { SURFACE } from './palette';

export interface BarChartProps {
  frame: Frame;
  spec: ChartSpec;
  stacked?: boolean;
  height?: number;
  /** Cross-filter: called with the clicked category, and the series when stacked. */
  onSelect?: (category: string, series?: string) => void;
}

/**
 * Bar and stacked bar. Horizontal is the default for long category names, which is
 * most fleet dimensions (space slugs, image references, resource types).
 */
export function BarChart({ frame, spec, stacked, height = 260, onSelect }: BarChartProps) {
  const mode = useMode();
  const axis = useAxisProps();
  const grid = useGridStroke();
  const legendFormatter = useLegendFormatter();
  const data = frameToData(frame);
  const horizontal = spec.orientation !== 'vertical';

  const seriesNames = frame.series.map((s) => s.name);
  const singleSeries = seriesNames.length === 1 && seriesNames[0] === 'value';

  // With one series the identity lives on the category axis, so colour encodes
  // magnitude; with several, colour carries series identity.
  const colorOf = (key: string, index: number, count: number) =>
    colorFor(key, index, count, spec.color, mode, spec.emphasize, singleSeries);

  // A horizontal bar chart must label every band. When Recharts thins the ticks out to
  // fit, the surviving labels sit beside bars they do not belong to and the chart reads
  // as a straightforwardly wrong ranking — so give each category room instead.
  const plotHeight = horizontal ? Math.max(height, data.length * 26 + 44) : height;

  return (
    <ResponsiveContainer width="100%" height={plotHeight}>
      <ReBarChart
        data={data}
        layout={horizontal ? 'vertical' : 'horizontal'}
        margin={{ top: 4, right: 16, bottom: 4, left: 4 }}
        barCategoryGap="20%"
        // Without a cap, a chart with one or two categories renders bars as thick as
        // their band — a slab that reads as a filled panel rather than a measurement.
        maxBarSize={34}
      >
        <CartesianGrid
          stroke={grid}
          horizontal={!horizontal}
          vertical={horizontal}
          strokeDasharray="2 4"
        />
        {horizontal ? (
          <>
            <XAxis type="number" {...axis} tickFormatter={(v: number) => formatValue(v)} />
            <YAxis
              type="category"
              dataKey="key"
              // Wide enough for an elided 26-character label; formatCategory keeps
              // anything longer from being clipped by the plot edge.
              width={186}
              interval={0}
              {...axis}
              tickFormatter={formatCategory}
            />
          </>
        ) : (
          <>
            <XAxis type="category" dataKey="key" {...axis} tickFormatter={formatCategory} />
            <YAxis type="number" {...axis} tickFormatter={(v: number) => formatValue(v)} />
          </>
        )}
        <Tooltip
          cursor={{ fill: grid, fillOpacity: 0.4 }}
          content={<ChartTooltip unit={spec.unit} hideSeriesNames={singleSeries} />}
        />
        {!singleSeries && <Legend iconType="square" iconSize={10} formatter={legendFormatter} />}

        {singleSeries ? (
          <Bar
            dataKey="value"
            radius={horizontal ? [0, 4, 4, 0] : [4, 4, 0, 0]}
            isAnimationActive={false}
            cursor={onSelect ? 'pointer' : undefined}
            onClick={(datum: unknown) => {
              const key = (datum as { key?: string } | undefined)?.key;
              if (key && onSelect) onSelect(key);
            }}
          >
            {data.map((datum, i) => (
              <Cell key={datum.key} fill={colorOf(datum.key, i, data.length)} />
            ))}
          </Bar>
        ) : (
          seriesNames.map((name, i) => (
            <Bar
              key={name}
              dataKey={name}
              stackId={stacked ? 'stack' : undefined}
              cursor={onSelect ? 'pointer' : undefined}
              onClick={(datum: unknown) => {
                const key = (datum as { key?: string } | undefined)?.key;
                if (key && onSelect) onSelect(key, name);
              }}
              fill={colorOf(name, i, seriesNames.length)}
              // A 2px surface-coloured edge keeps adjacent stack segments from
              // reading as one block.
              stroke={SURFACE[mode]}
              strokeWidth={stacked ? 2 : 0}
              radius={stacked ? 0 : horizontal ? [0, 4, 4, 0] : [4, 4, 0, 0]}
              isAnimationActive={false}
            />
          ))
        )}
      </ReBarChart>
    </ResponsiveContainer>
  );
}
