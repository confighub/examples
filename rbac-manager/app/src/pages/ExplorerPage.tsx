import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  Container,
  Drawer,
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
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material';
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { stringify } from 'yaml';

import { FriendlyResource } from '../components/friendly/RbacFriendly';
import { clusterContextFor } from '../fleet/enrichment';
import { useSnapshot } from '../fleet/SnapshotContext';
import { FleetResource } from '../rbac/model';

interface ResourceRow {
  resource: FleetResource;
  kind: string;
  name: string;
  namespace: string;
}

function toRow(resource: FleetResource): ResourceRow | null {
  const doc = resource.doc as { kind?: string; metadata?: { name?: string; namespace?: string } };
  if (typeof doc?.kind !== 'string') return null;
  return {
    resource,
    kind: doc.kind,
    name: doc.metadata?.name ?? '',
    namespace: doc.metadata?.namespace ?? '',
  };
}

/** Fleet-wide RBAC resource inventory with cluster/kind/text filtering. */
export function ExplorerPage() {
  const { snapshot, isLoading, error } = useSnapshot();
  const [cluster, setCluster] = useState('');
  const [kind, setKind] = useState('');
  const [text, setText] = useState('');
  const [selected, setSelected] = useState<ResourceRow | null>(null);
  const [viewMode, setViewMode] = useState<'friendly' | 'yaml'>('friendly');

  const rows = useMemo(() => {
    if (!snapshot) return [];
    return snapshot.resources
      .map(toRow)
      .filter((r): r is ResourceRow => r !== null)
      .filter((r) => cluster === '' || r.resource.origin.cluster === cluster)
      .filter((r) => kind === '' || r.kind === kind)
      .filter(
        (r) =>
          text === '' ||
          r.name.includes(text) ||
          r.namespace.includes(text) ||
          r.resource.origin.unitSlug.includes(text),
      );
  }, [snapshot, cluster, kind, text]);

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
  const kinds = ['ClusterRole', 'Role', 'ClusterRoleBinding', 'RoleBinding', 'ServiceAccount'];

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' spacing={2} sx={{ mb: 2 }}>
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
        <FormControl size='small' sx={{ minWidth: 180 }}>
          <InputLabel>Kind</InputLabel>
          <Select label='Kind' value={kind} onChange={(e) => setKind(e.target.value)}>
            <MenuItem value=''>All kinds</MenuItem>
            {kinds.map((k) => (
              <MenuItem key={k} value={k}>
                {k}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField
          size='small'
          label='Search name / unit'
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <Chip label={`${rows.length} resources`} sx={{ alignSelf: 'center' }} />
      </Stack>

      <Table size='small'>
        <TableHead>
          <TableRow>
            <TableCell>Kind</TableCell>
            <TableCell>Name</TableCell>
            <TableCell>Namespace</TableCell>
            <TableCell>Cluster</TableCell>
            <TableCell>Unit</TableCell>
            <TableCell>Space</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((r, i) => (
            <TableRow
              key={`${r.resource.origin.unitId}:${i}`}
              hover
              sx={{ cursor: 'pointer' }}
              onClick={() => setSelected(r)}
            >
              <TableCell>{r.kind}</TableCell>
              <TableCell>{r.name}</TableCell>
              <TableCell>{r.namespace}</TableCell>
              <TableCell>{r.resource.origin.cluster}</TableCell>
              <TableCell>{r.resource.origin.unitSlug}</TableCell>
              <TableCell>{r.resource.origin.space}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <Drawer anchor='right' open={selected !== null} onClose={() => setSelected(null)}>
        {selected && (
          <Box sx={{ width: 560, p: 2 }}>
            <Typography variant='h6' gutterBottom>
              {selected.kind} {selected.name}
            </Typography>
            <Typography variant='body2' color='text.secondary' gutterBottom>
              {selected.resource.origin.cluster} · unit{' '}
              <Link to={`/unit/${selected.resource.origin.spaceId}/${selected.resource.origin.unitId}`}>
                {selected.resource.origin.unitSlug}
              </Link>{' '}
              · space {selected.resource.origin.space}
            </Typography>
            <ToggleButtonGroup
              size='small'
              exclusive
              value={viewMode}
              onChange={(_, v: 'friendly' | 'yaml' | null) => v !== null && setViewMode(v)}
              sx={{ mb: 2 }}
            >
              <ToggleButton value='friendly'>Friendly</ToggleButton>
              <ToggleButton value='yaml'>YAML</ToggleButton>
            </ToggleButtonGroup>
            {viewMode === 'friendly' ? (
              <FriendlyResource
                doc={selected.resource.doc}
                cluster={clusterContextFor(
                  snapshot,
                  selected.resource.origin.cluster,
                  selected.resource.origin.unitId,
                )}
              />
            ) : (
              /* Read-only rendering; writes never round-trip through this. */
              <Box
                component='pre'
                sx={{ bgcolor: 'grey.100', p: 1.5, borderRadius: 1, overflow: 'auto', fontSize: 13 }}
              >
                {stringify(selected.resource.doc)}
              </Box>
            )}
          </Box>
        )}
      </Drawer>
    </Container>
  );
}
