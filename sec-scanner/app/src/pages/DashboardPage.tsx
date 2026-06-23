// Fleet security posture at a glance: a severity rollup across every running
// workload, plus how many are gated and how many are still unscanned. Data is
// the scanner's verdict stored on each Unit — nothing is computed here.

import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Container,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { useMemo } from 'react';
import { Link } from 'react-router-dom';

import { useSnapshot } from '../fleet/SnapshotContext';
import { isStale, severityHistogram, Workload } from '../sec/model';
import { SEVERITIES, severityColor, severityLabel, severityRank } from '../sec/severity';

function StatCard({ label, value, hint }: { label: string; value: number | string; hint?: string }) {
  return (
    <Card variant='outlined' sx={{ minWidth: 150, flex: '1 1 150px' }}>
      <CardContent>
        <Typography variant='h4'>{value}</Typography>
        <Typography color='text.secondary'>{label}</Typography>
        {hint && (
          <Typography variant='caption' color='text.secondary'>
            {hint}
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}

export function DashboardPage() {
  const { snapshot, isLoading, error, refresh } = useSnapshot();

  // "Running" workloads only — canonical base/policy Spaces hold definitions.
  const running = useMemo(
    () => (snapshot?.workloads ?? []).filter((w) => !w.canonical),
    [snapshot],
  );
  const hist = useMemo(() => severityHistogram(running), [running]);
  const gated = running.filter((w) => w.gates.length > 0).length;
  const unscanned = running.filter((w) => !w.scanned).length;
  const stale = running.filter((w) => isStale(w, snapshot?.cvedb ?? null)).length;
  const clusters = new Set(running.map((w) => w.cluster)).size;

  const worst = useMemo(
    () =>
      [...running]
        .filter((w) => severityRank(w.maxSeverity) >= severityRank('MEDIUM'))
        .sort(
          (a, b) =>
            severityRank(b.maxSeverity) - severityRank(a.maxSeverity) || b.cveCount - a.cveCount,
        )
        .slice(0, 15),
    [running],
  );

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' alignItems='center' spacing={2} sx={{ mb: 2 }}>
        <Typography variant='h5' sx={{ flexGrow: 1 }}>
          Fleet security posture
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

      <Stack direction='row' spacing={2} flexWrap='wrap' useFlexGap sx={{ mb: 3 }}>
        <StatCard label='workloads' value={running.length} hint={`${clusters} cluster(s)`} />
        <StatCard label='critical' value={hist.CRITICAL} hint='gated by no-critical-cves' />
        <StatCard label='high' value={hist.HIGH} />
        <StatCard label='gated' value={gated} hint='blocked from apply' />
        <StatCard label='unscanned' value={unscanned} hint='run scan-fleet --write-back' />
        <StatCard label='stale' value={stale} hint='scanned vs an older CVE DB' />
      </Stack>

      {snapshot?.cvedb && (
        <Typography variant='caption' color='text.secondary' sx={{ display: 'block', mb: 2 }}>
          CVE DB version <strong>{snapshot.cvedb.version || 'unknown'}</strong> ·{' '}
          {snapshot.cvedb.advisories.toLocaleString()} advisories
          {stale > 0 && ` · ${stale} workload(s) scanned against an older DB — re-scan`}
        </Typography>
      )}

      <Typography variant='subtitle1' gutterBottom>
        Severity distribution
      </Typography>
      <Stack direction='row' spacing={1} flexWrap='wrap' useFlexGap sx={{ mb: 3 }}>
        {SEVERITIES.map((s) => (
          <Chip
            key={s}
            color={severityColor(s)}
            variant={s === 'CRITICAL' || s === 'HIGH' ? 'filled' : 'outlined'}
            label={`${severityLabel(s)}: ${hist[s]}`}
          />
        ))}
      </Stack>

      <Typography variant='subtitle1' gutterBottom>
        Most vulnerable workloads
      </Typography>
      {worst.length === 0 ? (
        <Alert severity='success'>No workload is MEDIUM or above. Clean fleet (in scope).</Alert>
      ) : (
        <Table size='small'>
          <TableHead>
            <TableRow>
              <TableCell>Severity</TableCell>
              <TableCell>CVEs</TableCell>
              <TableCell>Workload</TableCell>
              <TableCell>Cluster</TableCell>
              <TableCell>Image</TableCell>
              <TableCell>Gated</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {worst.map((w: Workload) => (
              <TableRow key={w.unitId} hover>
                <TableCell>
                  <Chip size='small' color={severityColor(w.maxSeverity)} label={severityLabel(w.maxSeverity)} />
                </TableCell>
                <TableCell>{w.cveCount}</TableCell>
                <TableCell>
                  <Link to={`/unit/${w.spaceId}/${w.unitId}`}>{w.unitSlug}</Link>
                </TableCell>
                <TableCell>{w.cluster}</TableCell>
                <TableCell>
                  <Box component='code' sx={{ fontSize: 12 }}>
                    {w.images.join(', ')}
                  </Box>
                </TableCell>
                <TableCell>{w.gates.length > 0 ? 'yes' : ''}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Container>
  );
}
