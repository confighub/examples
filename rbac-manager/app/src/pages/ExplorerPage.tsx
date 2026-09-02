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
  TextField,
} from '@mui/material';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';

import { ResourceDetail, resourceIdentity } from '../components/ResourceDetail';
import { useSnapshot } from '../fleet/snapshot';
import { FleetResource } from '@confighub/examples-webkit/rbac';

interface ResourceRow {
  resource: FleetResource;
  kind: string;
  name: string;
  namespace: string;
}

function toRow(resource: FleetResource): ResourceRow | null {
  const { kind, name, namespace } = resourceIdentity(resource.doc);
  if (kind === '') return null;
  return { resource, kind, name, namespace };
}

/** Fleet-wide RBAC resource inventory with cluster/kind/text filtering. */
export function ExplorerPage() {
  const { snapshot, isLoading, error } = useSnapshot();
  // The cluster lives in the URL so the Dashboard and the Findings page can link
  // straight to one cluster's inventory.
  const [params, setParams] = useSearchParams();
  const cluster = params.get('cluster') ?? '';
  const [kind, setKind] = useState('');
  const [text, setText] = useState('');
  const [selected, setSelected] = useState<ResourceRow | null>(null);

  const setCluster = (value: string) => {
    const next = new URLSearchParams(params);
    if (value === '') next.delete('cluster');
    else next.set('cluster', value);
    setParams(next, { replace: true });
  };

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

  // Every group a resource can belong to, not just the clusters analysis runs on, so the
  // base Spaces the Dashboard lists are selectable here too.
  const clusters = [...new Set(snapshot.resources.map((r) => r.origin.cluster))].sort();
  const kinds = ['ClusterRole', 'Role', 'ClusterRoleBinding', 'RoleBinding', 'ServiceAccount'];

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Stack direction='row' spacing={2} sx={{ mb: 2 }}>
        <FormControl size='small' sx={{ minWidth: 200 }}>
          <InputLabel>Cluster/Space</InputLabel>
          <Select
            label='Cluster/Space'
            value={cluster}
            onChange={(e) => setCluster(e.target.value)}
          >
            <MenuItem value=''>All clusters and spaces</MenuItem>
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
            <TableCell>Cluster/Space</TableCell>
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

      <ResourceDetail
        resource={selected?.resource ?? null}
        onClose={() => setSelected(null)}
      />
    </Container>
  );
}
