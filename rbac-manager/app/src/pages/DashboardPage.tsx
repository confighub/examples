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
  Typography,
} from '@mui/material';
import { useMemo, useState } from 'react';

import { NewPersonaDialog } from '../components/NewPersonaDialog';
import { useSnapshot } from '../fleet/SnapshotContext';
import { ExtendedUnitRead } from '../sdk/confighubapi.gen';

interface GroupSummary {
  key: string;
  /** Target slug for clusters; Space slug for untargeted base groups. */
  label: string;
  unitCount: number;
  gatedCount: number;
  warnedCount: number;
  resourceCount: number;
  spaces: Set<string>;
}

function summarize(
  units: Iterable<ExtendedUnitRead>,
  resourceCountByCluster: Map<string, number>,
): { clusters: GroupSummary[]; bases: GroupSummary[] } {
  const groups = new Map<string, GroupSummary & { targeted: boolean }>();
  for (const eu of units) {
    const unit = eu.Unit;
    if (!unit) continue;
    const targeted = eu.Target?.Slug !== undefined;
    const label = eu.Target?.Slug ?? eu.Space?.Slug ?? 'unknown';
    const key = `${targeted ? 't' : 's'}:${label}`;
    let g = groups.get(key);
    if (!g) {
      g = {
        key,
        label,
        unitCount: 0,
        gatedCount: 0,
        warnedCount: 0,
        resourceCount: resourceCountByCluster.get(label) ?? 0,
        spaces: new Set(),
        targeted,
      };
      groups.set(key, g);
    }
    g.unitCount += 1;
    if (Object.keys(unit.ApplyGates ?? {}).length > 0) g.gatedCount += 1;
    if (Object.keys(unit.ApplyWarnings ?? {}).length > 0) g.warnedCount += 1;
    if (eu.Space?.Slug !== undefined) g.spaces.add(eu.Space.Slug);
  }
  const all = [...groups.values()].sort((a, b) => a.label.localeCompare(b.label));
  return {
    clusters: all.filter((g) => g.targeted),
    bases: all.filter((g) => !g.targeted),
  };
}

function GroupCard({ group, kind }: { group: GroupSummary; kind: 'cluster' | 'space' }) {
  return (
    <Card variant='outlined' sx={{ minWidth: 270 }}>
      <CardContent>
        <Stack direction='row' spacing={1} alignItems='center' sx={{ mb: 1 }}>
          <Typography variant='h6'>{group.label}</Typography>
          <Chip size='small' variant='outlined' label={kind === 'cluster' ? 'target' : 'space'} />
        </Stack>
        <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap'>
          <Typography variant='body2'>{group.unitCount} units</Typography>
          <Typography variant='body2'>{group.resourceCount} RBAC resources</Typography>
          <Typography
            variant='body2'
            color={group.gatedCount > 0 ? 'error' : 'text.secondary'}
            fontWeight={group.gatedCount > 0 ? 600 : 400}
          >
            {group.gatedCount} gated
          </Typography>
          <Typography
            variant='body2'
            color={group.warnedCount > 0 ? 'warning.main' : 'text.secondary'}
            fontWeight={group.warnedCount > 0 ? 600 : 400}
          >
            {group.warnedCount} warned
          </Typography>
        </Stack>
        {kind === 'cluster' && group.spaces.size > 1 && (
          <Typography variant='caption' color='text.secondary'>
            units from {group.spaces.size} spaces
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}

/**
 * Fleet dashboard: clusters (Targets) and base Spaces in scope, with unit,
 * gate, and warning summaries computed from the snapshot.
 */
export function DashboardPage() {
  const { snapshot, isLoading, error, refresh } = useSnapshot();
  const [personaOpen, setPersonaOpen] = useState(false);

  const resourceCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const r of snapshot?.resources ?? []) {
      counts.set(r.origin.cluster, (counts.get(r.origin.cluster) ?? 0) + 1);
    }
    return counts;
  }, [snapshot]);

  const { clusters, bases } = useMemo(
    () => summarize(snapshot?.units.values() ?? [], resourceCounts),
    [snapshot, resourceCounts],
  );

  if (error !== null) {
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

  const scoped = snapshot.scope.targetWhere !== '' || snapshot.scope.spaceWhere !== '';

  return (
    <Container sx={{ mt: 4 }}>
      <Stack direction='row' spacing={2} alignItems='center' sx={{ mb: 3 }}>
        <Typography variant='body2' color='text.secondary'>
          {snapshot.units.size} Kubernetes/YAML unit(s) in scope ·{' '}
          {snapshot.resources.length} RBAC resource(s)
          {scoped ? ' · custom scope active' : ''}
        </Typography>
        <Button size='small' onClick={() => void refresh()}>
          Refresh
        </Button>
        <Button size='small' variant='outlined' onClick={() => setPersonaOpen(true)}>
          New persona…
        </Button>
      </Stack>

      {snapshot.units.size === 0 && (
        <Alert severity='info'>
          No Kubernetes/YAML units in scope. Widen the scope (Scope button, top right), or seed
          the demo fleet with the example&apos;s demo-setup.sh.
        </Alert>
      )}

      {clusters.length > 0 && (
        <>
          <Typography variant='h5' sx={{ mb: 2 }}>
            Clusters
          </Typography>
          <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap' sx={{ mb: 4 }}>
            {clusters.map((g) => (
              <GroupCard key={g.key} group={g} kind='cluster' />
            ))}
          </Stack>
        </>
      )}

      {bases.length > 0 && (
        <>
          <Typography variant='h5' sx={{ mb: 2 }}>
            Spaces (units without a Target)
          </Typography>
          <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap'>
            {bases.map((g) => (
              <GroupCard key={g.key} group={g} kind='space' />
            ))}
          </Stack>
        </>
      )}

      <NewPersonaDialog
        open={personaOpen}
        onClose={(created) => {
          setPersonaOpen(false);
          if (created) void refresh();
        }}
      />
    </Container>
  );
}
