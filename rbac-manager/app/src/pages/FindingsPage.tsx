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
import { Link } from 'react-router-dom';

import { useSnapshot } from '../fleet/SnapshotContext';
import { Severity, analyzeFleet } from '../rbac/findings';

const SEVERITY_COLOR: Record<Severity, 'error' | 'warning' | 'default'> = {
  high: 'error',
  medium: 'warning',
  low: 'default',
};

/** Fleet hygiene audit: wildcards, escalation, cluster-admin, orphans. */
export function FindingsPage() {
  const { snapshot, isLoading, error } = useSnapshot();
  const [cluster, setCluster] = useState('');

  const findings = useMemo(
    () => (snapshot ? analyzeFleet(snapshot.clusters) : []),
    [snapshot],
  );
  const filtered = cluster === '' ? findings : findings.filter((f) => f.cluster === cluster);

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

  const clusters = [...snapshot.clusters.keys()].sort();
  const high = filtered.filter((f) => f.severity === 'high').length;

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' spacing={2} alignItems='center' sx={{ mb: 2 }}>
        <Typography variant='h5'>Findings</Typography>
        <FormControl size='small' sx={{ minWidth: 180 }}>
          <InputLabel>Cluster</InputLabel>
          <Select label='Cluster' value={cluster} onChange={(e) => setCluster(e.target.value)}>
            <MenuItem value=''>All clusters</MenuItem>
            {clusters.map((c) => (
              <MenuItem key={c} value={c}>
                {c}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <Chip
          label={`${filtered.length} findings, ${high} high`}
          color={high > 0 ? 'error' : 'success'}
          variant='outlined'
        />
      </Stack>

      {filtered.length === 0 && (
        <Alert severity='success'>No findings. The fleet&apos;s RBAC is clean.</Alert>
      )}

      {filtered.length > 0 && (
        <Table size='small'>
          <TableHead>
            <TableRow>
              <TableCell>Severity</TableCell>
              <TableCell>Analyzer</TableCell>
              <TableCell>Cluster</TableCell>
              <TableCell>Resource</TableCell>
              <TableCell>Unit</TableCell>
              <TableCell>Detail</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filtered.map((f) => (
              <TableRow key={f.id} hover>
                <TableCell>
                  <Chip size='small' label={f.severity} color={SEVERITY_COLOR[f.severity]} />
                </TableCell>
                <TableCell>{f.analyzer}</TableCell>
                <TableCell>{f.cluster}</TableCell>
                <TableCell>
                  {f.resourceKind}/{f.namespace !== undefined ? `${f.namespace}/` : ''}
                  {f.resourceName}
                </TableCell>
                <TableCell>
                  <Link to={`/unit/${f.origin.spaceId}/${f.origin.unitId}`}>{f.origin.unitSlug}</Link>
                </TableCell>
                <TableCell>{f.message}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Container>
  );
}
