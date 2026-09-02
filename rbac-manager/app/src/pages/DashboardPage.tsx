import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  Collapse,
  Container,
  Divider,
  Stack,
  Typography,
} from '@mui/material';
import { useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';

import { NewPersonaDialog } from '../components/NewPersonaDialog';
import { useSnapshot } from '../fleet/snapshot';
import {
  countBySeverity,
  findingsByCluster,
  type Finding,
  type SeverityCounts,
  type Severity,
} from '@confighub/examples-webkit/rbac';
import { ExtendedUnitRead } from '@confighub/rtk-query';

const SEVERITY_COLOR: Record<Severity, 'error' | 'warning' | 'default'> = {
  high: 'error',
  medium: 'warning',
  low: 'default',
};

/** A Unit carrying at least one gate or warning, and which. */
interface FlaggedUnit {
  unitId: string;
  spaceId: string;
  slug: string;
  gates: string[];
  warnings: string[];
}

interface GroupSummary {
  key: string;
  /** Target slug for clusters; Space slug for untargeted base groups. */
  label: string;
  unitCount: number;
  resourceCount: number;
  spaces: Set<string>;
  flagged: FlaggedUnit[];
  findings: SeverityCounts;
}

function summarize(
  units: Iterable<ExtendedUnitRead>,
  resourceCountByCluster: Map<string, number>,
  findingsByGroup: Map<string, Finding[]>,
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
        resourceCount: resourceCountByCluster.get(label) ?? 0,
        spaces: new Set(),
        flagged: [],
        findings: countBySeverity(findingsByGroup.get(label) ?? []),
        targeted,
      };
      groups.set(key, g);
    }
    g.unitCount += 1;
    const gates = Object.keys(unit.ApplyGates ?? {});
    const warnings = Object.keys(unit.ApplyWarnings ?? {});
    if (gates.length > 0 || warnings.length > 0) {
      g.flagged.push({
        unitId: unit.UnitID ?? '',
        spaceId: unit.SpaceID ?? '',
        slug: unit.Slug ?? '',
        gates,
        warnings,
      });
    }
    if (eu.Space?.Slug !== undefined) g.spaces.add(eu.Space.Slug);
  }
  const all = [...groups.values()].sort((a, b) => a.label.localeCompare(b.label));
  return {
    clusters: all.filter((g) => g.targeted),
    bases: all.filter((g) => !g.targeted),
  };
}

/** Severity chips, sized to whatever is actually present. */
function SeverityChips({ counts }: { counts: SeverityCounts }) {
  if (counts.total === 0) {
    return (
      <Typography variant='body2' color='text.secondary'>
        no findings
      </Typography>
    );
  }
  return (
    <Stack direction='row' spacing={0.5}>
      {(['high', 'medium', 'low'] as const)
        .filter((s) => counts[s] > 0)
        .map((s) => (
          <Chip key={s} size='small' color={SEVERITY_COLOR[s]} label={`${counts[s]} ${s}`} />
        ))}
    </Stack>
  );
}

/**
 * Which Units are gated or warned, and by which policy. The Dashboard is where a blocked
 * or flagged Unit is noticed, so the rule's name and a route to the Unit belong here
 * rather than one navigation away.
 */
function FlaggedUnits({ flagged }: { flagged: FlaggedUnit[] }) {
  return (
    <Stack spacing={0.5} sx={{ mt: 1 }}>
      {flagged.map((f) => (
        <Box key={f.unitId}>
          <Link to={`/unit/${f.spaceId}/${f.unitId}`} onClick={(e) => e.stopPropagation()}>
            {f.slug}
          </Link>
          <Stack direction='row' spacing={0.5} useFlexGap flexWrap='wrap' sx={{ mt: 0.5 }}>
            {f.gates.map((g) => (
              <Chip key={g} size='small' color='error' label={g} />
            ))}
            {f.warnings.map((w) => (
              <Chip key={w} size='small' color='warning' variant='outlined' label={w} />
            ))}
          </Stack>
        </Box>
      ))}
    </Stack>
  );
}

function GroupCard({ group, kind }: { group: GroupSummary; kind: 'cluster' | 'space' }) {
  const navigate = useNavigate();
  const [showFlagged, setShowFlagged] = useState(false);
  const gatedCount = group.flagged.filter((f) => f.gates.length > 0).length;
  const warnedCount = group.flagged.filter((f) => f.warnings.length > 0).length;

  return (
    <Card variant='outlined' sx={{ minWidth: 300 }}>
      <CardActionArea
        onClick={() => navigate(`/findings?cluster=${encodeURIComponent(group.label)}`)}
      >
        <CardContent sx={{ pb: 1 }}>
          <Stack direction='row' spacing={1} alignItems='center' sx={{ mb: 1 }}>
            <Typography variant='h6'>{group.label}</Typography>
            <Chip size='small' variant='outlined' label={kind === 'cluster' ? 'target' : 'space'} />
          </Stack>
          <Typography variant='body2' color='text.secondary'>
            {group.unitCount} units · {group.resourceCount} RBAC resources
          </Typography>
          <Box sx={{ mt: 1 }}>
            <SeverityChips counts={group.findings} />
          </Box>
          {kind === 'cluster' && group.spaces.size > 1 && (
            <Typography variant='caption' color='text.secondary'>
              units from {group.spaces.size} spaces
            </Typography>
          )}
        </CardContent>
      </CardActionArea>
      <Divider />
      <CardContent sx={{ pt: 1 }}>
        {group.flagged.length === 0 ? (
          <Typography variant='body2' color='text.secondary'>
            no apply gates or warnings
          </Typography>
        ) : (
          <>
            <Button
              size='small'
              sx={{ px: 0 }}
              onClick={() => setShowFlagged((v) => !v)}
              color={gatedCount > 0 ? 'error' : 'warning'}
            >
              {gatedCount} gated · {warnedCount} warned {showFlagged ? '▾' : '▸'}
            </Button>
            <Collapse in={showFlagged}>
              <FlaggedUnits flagged={group.flagged} />
            </Collapse>
          </>
        )}
      </CardContent>
    </Card>
  );
}

/**
 * Fleet dashboard: clusters (Targets) and base Spaces in scope, each with its unit,
 * resource, finding, and policy-signal counts. Every count is a route into the surface
 * that explains it.
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

  const byCluster = useMemo(
    () => findingsByCluster(snapshot?.findings ?? []),
    [snapshot],
  );
  const fleetCounts = useMemo(
    () => countBySeverity(snapshot?.findings ?? []),
    [snapshot],
  );

  const { clusters, bases } = useMemo(
    () => summarize(snapshot?.units.values() ?? [], resourceCounts, byCluster),
    [snapshot, resourceCounts, byCluster],
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
      <Stack direction='row' spacing={2} alignItems='center' sx={{ mb: 2 }}>
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

      <Stack direction='row' spacing={1} alignItems='center' sx={{ mb: 3 }} useFlexGap flexWrap='wrap'>
        <Typography variant='subtitle2'>Fleet findings</Typography>
        {fleetCounts.total === 0 ? (
          <Typography variant='body2' color='text.secondary'>
            none
          </Typography>
        ) : (
          (['high', 'medium', 'low'] as const)
            .filter((s) => fleetCounts[s] > 0)
            .map((s) => (
              <Chip
                key={s}
                size='small'
                clickable
                component={Link}
                to={`/findings?severity=${s}`}
                color={SEVERITY_COLOR[s]}
                label={`${fleetCounts[s]} ${s}`}
              />
            ))
        )}
        <Chip
          size='small'
          variant='outlined'
          clickable
          component={Link}
          to='/findings'
          label='all findings'
        />
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
          <Typography variant='h5' sx={{ mb: 1 }}>
            Spaces (units without a Target)
          </Typography>
          <Typography variant='body2' color='text.secondary' sx={{ mb: 2 }}>
            Base and policy definitions are not deployed anywhere, so they are checked for
            what a definition can be judged on alone — over-broad rules, escalation verbs,
            sensitive grants. Cross-reference checks (orphaned bindings, unbound
            ServiceAccounts) need a whole cluster in view and run only on the clusters.
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
