import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  Container,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';

import { ResourceDetail } from '../components/ResourceDetail';
import { resourceForFinding } from '../fleet/enrichment';
import { useSnapshot } from '../fleet/snapshot';
import { Finding, Severity, countBySeverity } from '@confighub/examples-webkit/rbac';

const SEVERITY_COLOR: Record<Severity, 'error' | 'warning' | 'default'> = {
  high: 'error',
  medium: 'warning',
  low: 'default',
};

/** The finding, restated above its resource in the detail panel. */
function FindingContext({ finding }: { finding: Finding }) {
  return (
    <Alert severity={finding.severity === 'low' ? 'info' : finding.severity === 'high' ? 'error' : 'warning'} sx={{ mt: 1 }}>
      <Typography variant='subtitle2'>{finding.analyzer}</Typography>
      {finding.message}
    </Alert>
  );
}

/** Fleet hygiene audit: wildcards, escalation, cluster-admin, orphans. */
export function FindingsPage() {
  const { snapshot, isLoading, error } = useSnapshot();
  // The cluster filter lives in the URL, so the Dashboard's cluster cards link straight
  // to that cluster's findings and the filtered view is shareable.
  const [params, setParams] = useSearchParams();
  const cluster = params.get('cluster') ?? '';
  const severity = params.get('severity') ?? '';
  const [selected, setSelected] = useState<Finding | null>(null);

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value === '') next.delete(key);
    else next.set(key, value);
    setParams(next, { replace: true });
  };

  const findings = snapshot?.findings ?? [];
  const filtered = useMemo(
    () =>
      findings
        .filter((f) => cluster === '' || f.cluster === cluster)
        .filter((f) => severity === '' || f.severity === severity),
    [findings, cluster, severity],
  );
  const selectedResource = useMemo(
    () => (selected === null ? null : resourceForFinding(snapshot, selected)),
    [snapshot, selected],
  );

  if (error) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity='error'>{error}</Alert>
      </Container>
    );
  }
  if (isLoading || !snapshot) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  // Clusters plus the base/policy Spaces, which carry definition-local findings of
  // their own.
  const clusters = [...snapshot.clusters.keys(), ...snapshot.definitions.keys()].sort();
  const counts = countBySeverity(filtered);

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' spacing={2} alignItems='center' sx={{ mb: 2 }}>
        <Typography variant='h5'>Findings</Typography>
        <FormControl size='small' sx={{ minWidth: 200 }}>
          <InputLabel>Cluster/Space</InputLabel>
          <Select
            label='Cluster/Space'
            value={cluster}
            onChange={(e) => setParam('cluster', e.target.value)}
          >
            <MenuItem value=''>All clusters and spaces</MenuItem>
            {clusters.map((c) => (
              <MenuItem key={c} value={c}>
                {c}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 140 }}>
          <InputLabel>Severity</InputLabel>
          <Select
            label='Severity'
            value={severity}
            onChange={(e) => setParam('severity', e.target.value)}
          >
            <MenuItem value=''>All severities</MenuItem>
            <MenuItem value='high'>high</MenuItem>
            <MenuItem value='medium'>medium</MenuItem>
            <MenuItem value='low'>low</MenuItem>
          </Select>
        </FormControl>
        <Chip
          label={`${counts.total} findings, ${counts.high} high`}
          color={counts.high > 0 ? 'error' : 'success'}
          variant='outlined'
        />
      </Stack>

      {filtered.length === 0 && findings.length === 0 && (
        <Alert severity='success'>No findings. The fleet&apos;s RBAC is clean.</Alert>
      )}
      {filtered.length === 0 && findings.length > 0 && (
        <Alert severity='info'>
          No findings match this filter. {findings.length} finding(s) elsewhere in the fleet.
        </Alert>
      )}

      {filtered.length > 0 && (
        <Table size='small'>
          <TableHead>
            <TableRow>
              <TableCell>Severity</TableCell>
              <TableCell>Analyzer</TableCell>
              <TableCell>Cluster/Space</TableCell>
              <TableCell>Resource</TableCell>
              <TableCell>Unit</TableCell>
              <TableCell>Detail</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filtered.map((f) => (
              <TableRow
                key={f.id}
                hover
                sx={{ cursor: 'pointer' }}
                onClick={() => setSelected(f)}
              >
                <TableCell>
                  <Chip size='small' label={f.severity} color={SEVERITY_COLOR[f.severity]} />
                </TableCell>
                <TableCell>{f.analyzer}</TableCell>
                <TableCell>{f.cluster}</TableCell>
                <TableCell>
                  {f.resourceKind}/{f.namespace !== undefined ? `${f.namespace}/` : ''}
                  {f.resourceName}
                </TableCell>
                {/* The unit link is the whole Unit; the row itself opens this resource. */}
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <Link to={`/unit/${f.origin.spaceId}/${f.origin.unitId}`}>{f.origin.unitSlug}</Link>
                </TableCell>
                <TableCell>{f.message}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {selected !== null && selectedResource === null && (
        <Alert severity='warning' sx={{ mt: 2 }} onClose={() => setSelected(null)}>
          The resource behind this finding is no longer in the snapshot. Refresh the
          Dashboard to re-read the fleet.
        </Alert>
      )}
      <ResourceDetail
        resource={selectedResource}
        onClose={() => setSelected(null)}
        context={selected !== null ? <FindingContext finding={selected} /> : undefined}
      />
    </Container>
  );
}
