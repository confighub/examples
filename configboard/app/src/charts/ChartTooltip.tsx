import Box from '@mui/material/Box';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';

import { formatCategory, formatValue } from './chartTheme';

export interface TooltipEntry {
  name?: string | number;
  value?: number | string;
  color?: string;
}

export interface ChartTooltipProps {
  active?: boolean;
  label?: string | number;
  payload?: TooltipEntry[];
  unit?: string;
  /** Single-series charts name the measure in the title, not on every row. */
  hideSeriesNames?: boolean;
}

/**
 * Hover layer. Values wear text tokens and the series colour rides a small swatch
 * beside them — a coloured number is unreadable against a light surface and puts
 * identity on colour alone.
 */
export function ChartTooltip({
  active,
  label,
  payload,
  unit,
  hideSeriesNames,
}: ChartTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  return (
    <Paper elevation={3} sx={{ px: 1.25, py: 1, minWidth: 140 }}>
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
        {formatCategory(String(label ?? ''))}
      </Typography>
      {payload.map((entry, i) => (
        <Box
          key={`${entry.name}-${i}`}
          sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0.5 }}
        >
          <Box
            sx={{
              width: 10,
              height: 10,
              borderRadius: '2px',
              bgcolor: entry.color,
              flexShrink: 0,
            }}
          />
          {!hideSeriesNames && (
            <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>
              {String(entry.name ?? '')}
            </Typography>
          )}
          <Typography variant="body2" color="text.primary" sx={{ fontWeight: 600 }}>
            {typeof entry.value === 'number' ? formatValue(entry.value, unit) : entry.value}
          </Typography>
        </Box>
      ))}
    </Paper>
  );
}
