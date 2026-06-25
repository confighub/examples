import { Box, Chip, Paper, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography } from '@mui/material';

import { type BudgetStatus, type CostRow, statusColor, usd } from '../cost/model';

const STATUSES: BudgetStatus[] = ['OVER', 'WARN', 'OK', 'UNKNOWN'];

function sum(rows: CostRow[]): number {
  return rows.reduce((t, r) => t + (r.monthlyUsd ?? 0), 0);
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Paper variant="outlined" sx={{ p: 2, flex: 1, minWidth: 160 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="h5" sx={{ fontWeight: 600 }}>
        {value}
      </Typography>
      {sub && (
        <Typography variant="caption" color="text.secondary">
          {sub}
        </Typography>
      )}
    </Paper>
  );
}

export function DashboardPage({ rows }: { rows: CostRow[] }) {
  const byStatus = (s: BudgetStatus) => rows.filter((r) => r.budgetStatus === s);
  const gated = rows.filter((r) => r.gates.length > 0);

  // Monthly cost rolled up by environment.
  const envs = [...new Set(rows.map((r) => r.environment || '(none)'))].sort();
  const byEnv = envs.map((e) => ({
    env: e,
    total: sum(rows.filter((r) => (r.environment || '(none)') === e)),
    count: rows.filter((r) => (r.environment || '(none)') === e).length,
  }));

  const topSpenders = [...rows].filter((r) => r.monthlyUsd != null).slice(0, 5);

  return (
    <Box sx={{ p: 3 }}>
      <Stack direction="row" spacing={2} sx={{ mb: 3, flexWrap: 'wrap' }} useFlexGap>
        <StatCard label="Fleet monthly cost" value={usd(sum(rows))} sub={`${rows.length} workloads`} />
        <StatCard label="Over budget" value={String(byStatus('OVER').length)} sub="blocked by within-budget" />
        <StatCard label="Gated workloads" value={String(gated.length)} sub="apply blocked by a guardrail" />
        <StatCard
          label="Uncosted"
          value={String(byStatus('UNKNOWN').length)}
          sub="missing requests / no budget"
        />
      </Stack>

      <Stack direction="row" spacing={3} sx={{ flexWrap: 'wrap' }} useFlexGap>
        <Paper variant="outlined" sx={{ p: 2, flex: 1, minWidth: 280 }}>
          <Typography variant="subtitle2" gutterBottom>
            Budget status
          </Typography>
          <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap' }} useFlexGap>
            {STATUSES.map((s) => (
              <Chip key={s} label={`${s} · ${byStatus(s).length}`} color={statusColor(s)} size="small" />
            ))}
          </Stack>
        </Paper>

        <Paper variant="outlined" sx={{ p: 2, flex: 1, minWidth: 280 }}>
          <Typography variant="subtitle2" gutterBottom>
            Monthly cost by environment
          </Typography>
          <Table size="small">
            <TableBody>
              {byEnv.map((e) => (
                <TableRow key={e.env}>
                  <TableCell>{e.env}</TableCell>
                  <TableCell align="right">{usd(e.total)}</TableCell>
                  <TableCell align="right" sx={{ color: 'text.secondary' }}>
                    {e.count}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>

        <Paper variant="outlined" sx={{ p: 2, flex: 1, minWidth: 280 }}>
          <Typography variant="subtitle2" gutterBottom>
            Top spenders
          </Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Workload</TableCell>
                <TableCell>Space</TableCell>
                <TableCell align="right">Monthly</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {topSpenders.map((r) => (
                <TableRow key={`${r.space}/${r.unit}`}>
                  <TableCell>{r.unit}</TableCell>
                  <TableCell sx={{ color: 'text.secondary' }}>{r.space}</TableCell>
                  <TableCell align="right">{usd(r.monthlyUsd)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>
      </Stack>
    </Box>
  );
}
