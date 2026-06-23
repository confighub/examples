// Every CVE the scanner found, flattened across the fleet: one row per
// (workload, finding). Filter by severity, search by CVE / package / image.
// The detail comes from the findings the scanner recorded on each Unit.

import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Link as MuiLink,
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
import { Finding } from '../sec/model';
import { Severity, severityColor, severityLabel, severityRank } from '../sec/severity';

interface FindingRow extends Finding {
  unitId: string;
  unitSlug: string;
  spaceId: string;
  cluster: string;
}

const FILTERS: { value: string; label: string }[] = [
  { value: 'CRITICAL', label: 'Critical only' },
  { value: 'HIGH', label: 'High and above' },
  { value: 'MEDIUM', label: 'Medium and above' },
  { value: 'LOW', label: 'All severities' },
];

/** Link a CVE/GHSA id to its advisory page. */
function advisoryUrl(id: string): string | undefined {
  if (id.startsWith('CVE-')) return `https://nvd.nist.gov/vuln/detail/${id}`;
  if (id.startsWith('GHSA-')) return `https://github.com/advisories/${id}`;
  return undefined;
}

export function FindingsPage() {
  const { snapshot, isLoading, error, refresh } = useSnapshot();
  const [filter, setFilter] = useState('HIGH');
  const [query, setQuery] = useState('');

  const all = useMemo<FindingRow[]>(() => {
    const out: FindingRow[] = [];
    for (const w of snapshot?.workloads ?? []) {
      for (const f of w.findings) {
        out.push({ ...f, unitId: w.unitId, unitSlug: w.unitSlug, spaceId: w.spaceId, cluster: w.cluster });
      }
    }
    return out;
  }, [snapshot]);

  const rows = useMemo(() => {
    let fs = all.filter((f) => severityRank(f.severity) >= severityRank(filter as Severity));
    const q = query.trim().toLowerCase();
    if (q) {
      fs = fs.filter(
        (f) =>
          f.advisory.toLowerCase().includes(q) ||
          f.package.toLowerCase().includes(q) ||
          f.unitSlug.toLowerCase().includes(q) ||
          f.cluster.toLowerCase().includes(q),
      );
    }
    return fs.sort(
      (a, b) =>
        severityRank(b.severity) - severityRank(a.severity) ||
        (b.cvss_score ?? 0) - (a.cvss_score ?? 0) ||
        a.package.localeCompare(b.package),
    );
  }, [all, filter, query]);

  const distinctCves = useMemo(() => new Set(rows.map((r) => r.advisory)).size, [rows]);

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' alignItems='center' spacing={2} sx={{ mb: 2 }}>
        <Typography variant='h5' sx={{ flexGrow: 1 }}>
          Findings
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
        <TextField select size='small' label='Severity' value={filter} onChange={(e) => setFilter(e.target.value)} sx={{ minWidth: 200 }}>
          {FILTERS.map((f) => (
            <MenuItem key={f.value} value={f.value}>
              {f.label}
            </MenuItem>
          ))}
        </TextField>
        <TextField size='small' label='Search CVE / package / unit' value={query} onChange={(e) => setQuery(e.target.value)} sx={{ flexGrow: 1 }} />
      </Stack>

      <Typography variant='caption' color='text.secondary'>
        {rows.length} finding(s), {distinctCves} distinct CVE(s)
      </Typography>

      {all.length === 0 && !isLoading ? (
        <Alert severity='info' sx={{ mt: 2 }}>
          No findings recorded yet. Run{' '}
          <Box component='code'>secscan scan-fleet --space "sec-demo-*" --write-back</Box> to populate them.
        </Alert>
      ) : (
        <Table size='small' sx={{ mt: 1 }}>
          <TableHead>
            <TableRow>
              <TableCell>Severity</TableCell>
              <TableCell>Score</TableCell>
              <TableCell>CVE</TableCell>
              <TableCell>Package</TableCell>
              <TableCell>Installed</TableCell>
              <TableCell>Fixed</TableCell>
              <TableCell>Workload</TableCell>
              <TableCell>Cluster</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map((f, i) => {
              const url = advisoryUrl(f.advisory);
              return (
                <TableRow key={`${f.unitId}-${f.advisory}-${f.package}-${i}`} hover>
                  <TableCell>
                    <Chip size='small' color={severityColor(f.severity)} label={severityLabel(f.severity)} />
                  </TableCell>
                  <TableCell>{f.cvss_score ? f.cvss_score.toFixed(1) : '—'}</TableCell>
                  <TableCell>
                    {url ? (
                      <MuiLink href={url} target='_blank' rel='noreferrer'>
                        {f.advisory}
                      </MuiLink>
                    ) : (
                      f.advisory
                    )}
                  </TableCell>
                  <TableCell>{f.package}</TableCell>
                  <TableCell>
                    <Box component='code' sx={{ fontSize: 12 }}>
                      {f.version}
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Box component='code' sx={{ fontSize: 12 }}>
                      {f.fixed_version ?? '—'}
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Link to={`/unit/${f.spaceId}/${f.unitId}`}>{f.unitSlug}</Link>
                  </TableCell>
                  <TableCell>{f.cluster}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </Container>
  );
}
