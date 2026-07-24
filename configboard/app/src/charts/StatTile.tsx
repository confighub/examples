import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';

import { formatValue, useMode } from './chartTheme';
import { NEUTRAL, STATUS, type StatusLevel } from './palette';

export interface StatTileProps {
  value: number;
  label?: string;
  unit?: string;
  level?: StatusLevel;
}

/** A single current value. Not a one-bar bar chart. */
export function StatTile({ value, label, unit, level }: StatTileProps) {
  const mode = useMode();
  // A headline count is shown in full: "1.4k" hides whether the fleet grew by 40
  // resources. Axis ticks still abbreviate — there, precision is noise.
  const text = Number.isInteger(value) ? value.toLocaleString() : formatValue(value);

  return (
    <Box sx={{ py: 1 }}>
      <Typography
        sx={{
          fontSize: 40,
          fontWeight: 600,
          lineHeight: 1.1,
          // The number wears a text token unless it reports a state worth flagging.
          color: level ? STATUS[mode][level] : 'text.primary',
        }}
      >
        {unit ? `${text} ${unit}` : text}
      </Typography>
      {label && (
        <Typography variant="body2" color="text.secondary">
          {label}
        </Typography>
      )}
    </Box>
  );
}

export interface MeterProps {
  value: number;
  total: number;
  label?: string;
  /** Lower is better — a full track is bad news rather than good. */
  invert?: boolean;
}

/** A single ratio against a limit. Same-hue track; never a two-slice pie. */
export function Meter({ value, total, label, invert }: MeterProps) {
  const mode = useMode();
  const ratio = total > 0 ? Math.min(1, Math.max(0, value / total)) : 0;
  const pct = Math.round(ratio * 100);

  const level: StatusLevel = invert
    ? ratio > 0.25
      ? 'critical'
      : ratio > 0.05
        ? 'warning'
        : 'good'
    : ratio > 0.9
      ? 'good'
      : ratio > 0.6
        ? 'warning'
        : 'critical';

  return (
    <Box sx={{ py: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
        <Typography sx={{ fontSize: 40, fontWeight: 600, lineHeight: 1.1 }}>{pct}%</Typography>
        <Typography variant="body2" color="text.secondary">
          {formatValue(value)} of {formatValue(total)}
        </Typography>
      </Box>
      <Box
        sx={{
          mt: 1,
          height: 8,
          borderRadius: 1,
          bgcolor: mode === 'dark' ? '#2f2f2c' : '#e6e5e1',
          overflow: 'hidden',
        }}
      >
        <Box
          sx={{
            width: `${pct}%`,
            height: '100%',
            bgcolor: total > 0 ? STATUS[mode][level] : NEUTRAL[mode],
            borderRadius: 1,
          }}
        />
      </Box>
      {label && (
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          {label}
        </Typography>
      )}
    </Box>
  );
}
