import {
  CartesianGrid,
  Legend,
  Line,
  LineChart as ReLineChart,
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

export interface LineChartProps {
  frame: Frame;
  spec: ChartSpec;
  height?: number;
}

/** Trend over time. One y-axis, always — two measures of different scale get two charts. */
export function LineChart({ frame, spec, height = 260 }: LineChartProps) {
  const mode = useMode();
  const axis = useAxisProps();
  const grid = useGridStroke();
  const legendFormatter = useLegendFormatter();
  const data = frameToData(frame);

  const seriesNames = frame.series.map((s) => s.name);
  const singleSeries = seriesNames.length === 1 && seriesNames[0] === 'value';

  return (
    <ResponsiveContainer width="100%" height={height}>
      <ReLineChart data={data} margin={{ top: 8, right: 16, bottom: 4, left: 4 }}>
        <CartesianGrid stroke={grid} vertical={false} strokeDasharray="2 4" />
        <XAxis dataKey="key" {...axis} tickFormatter={formatCategory} minTickGap={24} />
        <YAxis {...axis} tickFormatter={(v: number) => formatValue(v)} />
        <Tooltip
          cursor={{ stroke: grid, strokeWidth: 1 }}
          content={<ChartTooltip unit={spec.unit} hideSeriesNames={singleSeries} />}
        />
        {!singleSeries && <Legend iconType="line" iconSize={12} formatter={legendFormatter} />}
        {seriesNames.map((name, i) => (
          <Line
            key={name}
            // Straight segments between observations. A spline through daily counts
            // invents values on the way — including dips below what was measured.
            type="linear"
            dataKey={name}
            stroke={colorFor(name, i, seriesNames.length, spec.color, mode, spec.emphasize)}
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
            isAnimationActive={false}
          />
        ))}
      </ReLineChart>
    </ResponsiveContainer>
  );
}
