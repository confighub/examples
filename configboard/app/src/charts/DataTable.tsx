import Box from '@mui/material/Box';
import Link from '@mui/material/Link';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';

import type { Frame, Row } from '../model/types';
import { formatCategory, formatValue } from './chartTheme';

export interface FrameTableProps {
  frame: Frame;
  /** Column header for the category axis. */
  dimensionLabel?: string;
}

/**
 * The table view every panel can switch to. This is the relief the light-mode palette
 * requires — three categorical slots sit below 3:1 against the light surface, so the
 * numbers must be readable without relying on the marks.
 */
export function FrameTable({ frame, dimensionLabel = 'Category' }: FrameTableProps) {
  const multi = frame.series.length > 1 || frame.series[0]?.name !== 'value';

  return (
    <Box sx={{ maxHeight: 300, overflow: 'auto' }}>
      <Table size="small" stickyHeader>
        <TableHead>
          <TableRow>
            <TableCell>{dimensionLabel}</TableCell>
            {multi ? (
              frame.series.map((s) => (
                <TableCell key={s.name} align="right">
                  {s.name}
                </TableCell>
              ))
            ) : (
              <TableCell align="right">Value</TableCell>
            )}
          </TableRow>
        </TableHead>
        <TableBody>
          {frame.categories.map((category) => (
            <TableRow key={category} hover>
              <TableCell>{formatCategory(category)}</TableCell>
              {frame.series.map((s) => {
                const point = s.points.find((p) => p.key === category);
                return (
                  <TableCell key={s.name} align="right">
                    {formatValue(point?.value ?? 0)}
                  </TableCell>
                );
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  );
}

export interface RowTableProps {
  rows: Row[];
  columns: { id: string; label: string }[];
  limit?: number;
}

/** Drill-down: the rows behind a mark, each linking into the ConfigHub UI. */
export function RowTable({ rows, columns, limit = 200 }: RowTableProps) {
  const shown = rows.slice(0, limit);

  return (
    <Box>
      <Box sx={{ maxHeight: 360, overflow: 'auto' }}>
        <Table size="small" stickyHeader>
          <TableHead>
            <TableRow>
              {columns.map((c) => (
                <TableCell key={c.id}>{c.label}</TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {shown.map((row) => (
              <TableRow key={row.id} hover>
                {columns.map((c, i) => {
                  const value = row.values[c.id];
                  const text = value === null || value === undefined ? '—' : String(value);
                  return (
                    <TableCell key={c.id}>
                      {i === 0 && row.href ? (
                        <Link href={row.href} target="_blank" rel="noreferrer" underline="hover">
                          {text}
                        </Link>
                      ) : (
                        text
                      )}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Box>
      {rows.length > shown.length && (
        <Typography variant="caption" color="text.secondary">
          Showing {shown.length} of {rows.length} rows.
        </Typography>
      )}
    </Box>
  );
}
