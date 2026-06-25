import {
  Alert,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Typography,
} from '@mui/material';

import { type CostRow, num, statusColor, usd } from '../cost/model';

function Row({ k, v }: { k: string; v: string }) {
  return (
    <TableRow>
      <TableCell sx={{ color: 'text.secondary', border: 0, py: 0.5, width: 160 }}>{k}</TableCell>
      <TableCell sx={{ border: 0, py: 0.5, fontFamily: 'monospace' }}>{v}</TableCell>
    </TableRow>
  );
}

export function UnitDialog({ row, onClose }: { row: CostRow | null; onClose: () => void }) {
  if (!row) return null;
  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Stack direction="row" spacing={1} alignItems="center">
          <span>{row.unit}</span>
          <Chip label={row.budgetStatus} color={statusColor(row.budgetStatus)} size="small" />
          <Typography variant="caption" color="text.secondary">
            {row.space}
          </Typography>
        </Stack>
      </DialogTitle>
      <DialogContent>
        <Typography variant="h4" sx={{ fontWeight: 600, mb: 2 }}>
          {usd(row.monthlyUsd)}
          <Typography component="span" variant="body2" color="text.secondary">
            {' '}
            / month
          </Typography>
        </Typography>

        {row.gates.length > 0 && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            Apply blocked by: {row.gates.join(', ')}
          </Alert>
        )}

        <Typography variant="subtitle2" gutterBottom>
          Cost inputs
        </Typography>
        <Table size="small">
          <TableBody>
            <Row k="kind" v={row.kind} />
            <Row k="environment" v={row.environment || '—'} />
            <Row k="region" v={row.region || '—'} />
            <Row k="cpu cores" v={num(row.cpuCores)} />
            <Row k="memory (GB)" v={num(row.memoryGb)} />
            <Row k="storage (GB)" v={num(row.storageGb, 0)} />
          </TableBody>
        </Table>

        <Divider sx={{ my: 2 }} />
        <Typography variant="subtitle2" gutterBottom>
          Provenance
        </Typography>
        <Table size="small">
          <TableBody>
            <Row k="pricing version" v={row.pricingVersion || '—'} />
            <Row k="estimated at" v={row.estimatedAt || '—'} />
          </TableBody>
        </Table>
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
          Read from cost-estimator.confighub.com/* annotations on the Unit.
        </Typography>
      </DialogContent>
    </Dialog>
  );
}
