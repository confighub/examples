import { Box, Chip, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography } from '@mui/material';

import { type CostRow, num, statusColor, usd } from '../cost/model';

export function FleetPage({ rows, onSelect }: { rows: CostRow[]; onSelect: (r: CostRow) => void }) {
  return (
    <Box sx={{ p: 3, overflow: 'auto' }}>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
        {rows.length} workloads · estimates and gates read from ConfigHub
      </Typography>
      <Table size="small" stickyHeader>
        <TableHead>
          <TableRow>
            {['Space', 'Workload', 'Env', 'Kind', 'CPU', 'Mem (GB)', 'Stg (GB)', 'Monthly', 'Status', 'Gates'].map(
              (h) => (
                <TableCell key={h} sx={{ fontWeight: 600 }} align={['CPU', 'Mem (GB)', 'Stg (GB)', 'Monthly'].includes(h) ? 'right' : 'left'}>
                  {h}
                </TableCell>
              ),
            )}
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={`${r.space}/${r.unit}`} hover sx={{ cursor: 'pointer' }} onClick={() => onSelect(r)}>
              <TableCell sx={{ color: 'text.secondary' }}>{r.space}</TableCell>
              <TableCell>{r.unit}</TableCell>
              <TableCell>{r.environment || '—'}</TableCell>
              <TableCell sx={{ color: 'text.secondary' }}>{r.kind}</TableCell>
              <TableCell align="right">{num(r.cpuCores)}</TableCell>
              <TableCell align="right">{num(r.memoryGb)}</TableCell>
              <TableCell align="right">{num(r.storageGb, 0)}</TableCell>
              <TableCell align="right">{usd(r.monthlyUsd)}</TableCell>
              <TableCell>
                <Chip label={r.budgetStatus} color={statusColor(r.budgetStatus)} size="small" />
              </TableCell>
              <TableCell>
                <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }} useFlexGap>
                  {r.gates.map((g) => (
                    <Chip key={g} label={g} size="small" variant="outlined" color="warning" />
                  ))}
                </Stack>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  );
}
