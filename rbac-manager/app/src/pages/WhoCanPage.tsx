import {
  Alert,
  Box,
  Button,
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
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { useSnapshot } from '../fleet/SnapshotContext';
import { Grant, whoCan } from '../rbac/whocan';

const VERBS = ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete', 'deletecollection'];

/** "Who can VERB RESOURCE [in NAMESPACE]?" across every cluster. */
export function WhoCanPage() {
  const { snapshot, isLoading, error } = useSnapshot();
  const [verb, setVerb] = useState('get');
  const [resource, setResource] = useState('secrets');
  const [apiGroup, setApiGroup] = useState('');
  const [namespace, setNamespace] = useState('');
  const [grants, setGrants] = useState<Grant[] | null>(null);

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

  const run = () => {
    setGrants(
      whoCan(snapshot.clusters, {
        verb,
        resource: resource.trim(),
        apiGroup: apiGroup.trim(),
        namespace: namespace.trim() === '' ? undefined : namespace.trim(),
      }),
    );
  };

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Typography variant='h5' gutterBottom>
        Who can…
      </Typography>
      <Stack direction='row' spacing={2} sx={{ mb: 3 }}>
        <FormControl size='small' sx={{ minWidth: 140 }}>
          <InputLabel>Verb</InputLabel>
          <Select label='Verb' value={verb} onChange={(e) => setVerb(e.target.value)}>
            {VERBS.map((v) => (
              <MenuItem key={v} value={v}>
                {v}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField
          size='small'
          label='Resource (e.g. secrets, pods/exec)'
          value={resource}
          onChange={(e) => setResource(e.target.value)}
        />
        <TextField
          size='small'
          label='API group (blank = core)'
          value={apiGroup}
          onChange={(e) => setApiGroup(e.target.value)}
        />
        <TextField
          size='small'
          label='Namespace (blank = anywhere)'
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
        />
        <Button variant='contained' onClick={run} disabled={resource.trim() === ''}>
          Ask the fleet
        </Button>
      </Stack>

      {grants !== null && grants.length === 0 && (
        <Alert severity='success'>
          Nobody. No subject in the fleet can {verb} {resource}
          {namespace !== '' ? ` in ${namespace}` : ''}.
        </Alert>
      )}

      {grants !== null && grants.length > 0 && (
        <>
          <Typography variant='body2' color='text.secondary' sx={{ mb: 1 }}>
            {grants.length} grant(s) across the fleet
          </Typography>
          <Table size='small'>
            <TableHead>
              <TableRow>
                <TableCell>Cluster</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Via role</TableCell>
                <TableCell>Via binding</TableCell>
                <TableCell>Scope</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {grants.map((g, i) => (
                <TableRow key={i}>
                  <TableCell>{g.cluster}</TableCell>
                  <TableCell>{g.subjectKey}</TableCell>
                  <TableCell>
                    {g.roleRefName}
                    {g.viaBuiltinRole && (
                      <Chip size='small' label='builtin' sx={{ ml: 1 }} variant='outlined' />
                    )}
                  </TableCell>
                  <TableCell>
                    {g.binding.kind}/{g.binding.name}
                  </TableCell>
                  <TableCell>{g.scope ?? 'cluster-wide'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </>
      )}
    </Container>
  );
}
