import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';

import type { ChartSpec, Frame } from '../model/types';
import { ChartTooltip } from './ChartTooltip';
import { colorFor, formatValue, useMode } from './chartTheme';
import { SURFACE } from './palette';

export interface DonutChartProps {
  frame: Frame;
  spec: ChartSpec;
  height?: number;
}

/**
 * Part-to-whole for a small number of slices. Anything with a long tail is a bar
 * chart instead — the panel author picks, but the legend below carries every label so
 * the reading never depends on distinguishing two arcs by hue.
 */
export function DonutChart({ frame, spec, height = 260 }: DonutChartProps) {
  const mode = useMode();
  const series = frame.series[0];
  const points = (series?.points ?? []).filter((p) => p.value > 0);
  const total = points.reduce((sum, p) => sum + p.value, 0);

  const data = points.map((p) => ({ key: p.key, value: p.value }));
  const colorOf = (key: string, index: number) =>
    colorFor(key, index, data.length, spec.color ?? 'categorical', mode, spec.emphasize);

  return (
    <Box>
      <Box sx={{ position: 'relative' }}>
        <ResponsiveContainer width="100%" height={height}>
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              nameKey="key"
              innerRadius="58%"
              outerRadius="82%"
              paddingAngle={1}
              stroke={SURFACE[mode]}
              strokeWidth={2}
              isAnimationActive={false}
            >
              {data.map((datum, i) => (
                <Cell key={datum.key} fill={colorOf(datum.key, i)} />
              ))}
            </Pie>
            <Tooltip content={<ChartTooltip unit={spec.unit} />} />
          </PieChart>
        </ResponsiveContainer>
        <Box
          sx={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            pointerEvents: 'none',
          }}
        >
          <Typography variant="h5" sx={{ fontWeight: 600, lineHeight: 1 }}>
            {formatValue(total)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            total
          </Typography>
        </Box>
      </Box>

      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5, mt: 1, justifyContent: 'center' }}>
        {data.map((datum, i) => (
          <Box key={datum.key} sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
            <Box
              sx={{
                width: 10,
                height: 10,
                borderRadius: '2px',
                bgcolor: colorOf(datum.key, i),
              }}
            />
            <Typography variant="caption" color="text.secondary">
              {datum.key}
            </Typography>
            <Typography variant="caption" color="text.primary" sx={{ fontWeight: 600 }}>
              {formatValue(datum.value)}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}
