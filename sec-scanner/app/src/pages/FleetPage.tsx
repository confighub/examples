// The fleet image inventory: every workload, its image(s), the scan verdict,
// and its gate state — one row per Unit, filterable by severity and free text.
// This is the "which clusters run a vulnerable image" view that a per-image
// scan can't give you.

import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  MenuItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';

import { useSnapshot } from '../fleet/SnapshotContext';
import { isStale, Workload } from '../sec/model';
import { Severity, severityColor, severityLabel, severityRank } from '../sec/severity';

const FILTERS: { value: string; label: string }[] = [
  { value: 'all', label: 'All severities' },
  { value: 'CRITICAL', label: 'Critical only' },
  { value: 'HIGH', label: 'High and above' },
  { value: 'MEDIUM', label: 'Medium and above' },
  { value: 'gated', label: 'Gated only' },
  { value: 'unscanned', label: 'Unscanned only' },
  { value: 'stale', label: 'Stale only' },
];

export function FleetPage() {
  const { snapshot, isLoading, error, refresh } = useSnapshot();
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');

  const rows = useMemo(() => {
    let ws: Workload[] = snapshot?.workloads ?? [];
    if (filter === 'gated') ws = ws.filter((w) => w.gates.length > 0);
    else if (filter === 'unscanned') ws = ws.filter((w) => !w.scanned);
    else if (filter === 'stale') ws = ws.filter((w) => isStale(w, snapshot?.cvedb ?? null));
    else if (filter !== 'all') ws = ws.filter((w) => severityRank(w.maxSeverity) >= severityRank(filter as Severity));
    const q = query.trim().toLowerCase();
    if (q) {
      ws = ws.filter(
        (w) =>
          w.unitSlug.toLowerCase().includes(q) ||
          w.cluster.toLowerCase().includes(q) ||
          w.images.some((i) => i.toLowerCase().includes(q)),
      );
    }
    return [...ws].sort(
      (a, b) =>
        severityRank(b.maxSeverity) - severityRank(a.maxSeverity) ||
        a.cluster.localeCompare(b.cluster) ||
        a.unitSlug.localeCompare(b.unitSlug),
    );
  }, [snapshot, filter, query]);

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' alignItems='center' spacing={2} sx={{ mb: 2 }}>
        <Typography variant='h5' sx={{ flexGrow: 1 }}>
          Image inventory
        </Typography>
        {isLoading && <CircularProgress size={20} />}
        <Button variant='outlined' size='small' onClick={() => void refresh()} disabled={isLoading}>
          Refresh
        </Button>
      </Stack>

      {error && (
        <Alert severity='error' sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <Stack direction='row' spacing={2} sx={{ mb: 2 }}>
        <TextField
          select
          size='small'
          label='Filter'
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          sx={{ minWidth: 200 }}
        >
          {FILTERS.map((f) => (
            <MenuItem key={f.value} value={f.value}>
              {f.label}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          size='small'
          label='Search image / unit / cluster'
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          sx={{ flexGrow: 1 }}
        />
      </Stack>

      <Typography variant='caption' color='text.secondary'>
        {rows.length} workload(s)
      </Typography>

      <Table size='small' sx={{ mt: 1 }}>
        <TableHead>
          <TableRow>
            <TableCell>Severity</TableCell>
            <TableCell>CVEs</TableCell>
            <TableCell>Cluster</TableCell>
            <TableCell>Workload</TableCell>
            <TableCell>Image(s)</TableCell>
            <TableCell>Gates / warnings</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((w) => (
            <TableRow key={w.unitId} hover>
              <TableCell>
                <Stack direction='row' spacing={0.5} alignItems='center'>
                  <Chip
                    size='small'
                    color={severityColor(w.maxSeverity)}
                    variant={w.scanned ? 'filled' : 'outlined'}
                    label={severityLabel(w.maxSeverity)}
                  />
                  {isStale(w, snapshot?.cvedb ?? null) && (
                    <Chip size='small' color='warning' variant='outlined' label='stale' title='scanned against an older CVE DB' />
                  )}
                </Stack>
              </TableCell>
              <TableCell>{w.scanned ? w.cveCount : '—'}</TableCell>
              <TableCell>
                {w.cluster}
                {w.canonical && (
                  <Chip size='small' variant='outlined' label='base' sx={{ ml: 1 }} />
                )}
              </TableCell>
              <TableCell>
                <Link to={`/unit/${w.spaceId}/${w.unitId}`}>{w.unitSlug}</Link>
              </TableCell>
              <TableCell>
                <Box component='code' sx={{ fontSize: 12 }}>
                  {w.images.join(', ')}
                </Box>
              </TableCell>
              <TableCell>
                <Stack direction='row' spacing={0.5} flexWrap='wrap' useFlexGap>
                  {w.gates.map((g) => (
                    <Chip key={g} size='small' color='error' label={g.split('/').slice(-2, -1)[0] ?? g} />
                  ))}
                  {w.warnings.map((wn) => (
                    <Chip
                      key={wn}
                      size='small'
                      color='warning'
                      variant='outlined'
                      label={wn.split('/').slice(-2, -1)[0] ?? wn}
                    />
                  ))}
                </Stack>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {rows.length === 0 && !isLoading && (
        <Alert severity='info' sx={{ mt: 2 }}>
          No workloads match. If everything is unscanned, run{' '}
          <Box component='code'>secscan scan-fleet --space "sec-demo-*" --write-back</Box>.
        </Alert>
      )}
    </Container>
  );
}
