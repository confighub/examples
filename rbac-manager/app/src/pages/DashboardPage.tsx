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
import { useState } from 'react';

import { NewPersonaDialog } from '../components/NewPersonaDialog';
import { ExtendedSpaceRead, useListSpacesQuery } from '../sdk/confighubapi.gen';

/** Spaces managed by this app carry the app=rbac-manager label. */
const FLEET_WHERE = "Labels.app = 'rbac-manager'";

interface SpaceCardProps {
  extended: ExtendedSpaceRead;
}

function SpaceCard({ extended }: SpaceCardProps) {
  const space = extended.Space;
  if (!space) return null;
  const labels = space.Labels ?? {};
  const gated = extended.GatedUnitCount ?? 0;
  const units = extended.TotalUnitCount ?? 0;
  const unapplied = extended.UnappliedUnitCount ?? 0;
  const triggers = Object.values(extended.TriggerCountByEventType ?? {}).reduce(
    (a, b) => a + b,
    0,
  );

  return (
    <Card variant='outlined' sx={{ minWidth: 260 }}>
      <CardContent>
        <Stack direction='row' spacing={1} alignItems='center' sx={{ mb: 1 }}>
          <Typography variant='h6'>{space.Slug}</Typography>
          {labels.env !== undefined && <Chip size='small' label={labels.env} />}
          {labels.region !== undefined && (
            <Chip size='small' variant='outlined' label={labels.region} />
          )}
          {labels.role !== undefined && (
            <Chip size='small' color='secondary' label={labels.role} />
          )}
        </Stack>
        <Stack direction='row' spacing={2}>
          <Typography variant='body2'>{units} units</Typography>
          <Typography
            variant='body2'
            color={gated > 0 ? 'error' : 'text.secondary'}
            fontWeight={gated > 0 ? 600 : 400}
          >
            {gated} gated
          </Typography>
          <Typography variant='body2' color='text.secondary'>
            {unapplied} unapplied
          </Typography>
          {triggers > 0 && (
            <Typography variant='body2' color='text.secondary'>
              {triggers} triggers
            </Typography>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}

/**
 * Fleet dashboard: every Space labeled app=rbac-manager, with unit/gate
 * summaries straight off the extended Space read.
 */
export function DashboardPage() {
  // summary=true populates the Total/Gated/Unapplied unit counts.
  const { data, isLoading, isError, refetch } = useListSpacesQuery({
    where: FLEET_WHERE,
    summary: true,
  });
  const [personaOpen, setPersonaOpen] = useState(false);

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 12 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (isError) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity='error'>Failed to load the fleet. Check your connection and token.</Alert>
      </Container>
    );
  }

  const spaces = data ?? [];
  const clusters = spaces.filter((s) => s.Space?.Labels?.env !== undefined);
  const platform = spaces.filter((s) => s.Space?.Labels?.env === undefined);
  const baseSpace = spaces.find((s) => s.Space?.Labels?.role === 'base');

  return (
    <Container sx={{ mt: 4 }}>
      {spaces.length === 0 && (
        <Alert severity='info'>
          No Spaces labeled app=rbac-manager found in this organization. Run the
          example&apos;s setup.sh to seed the demo fleet.
        </Alert>
      )}

      {clusters.length > 0 && (
        <>
          <Typography variant='h5' sx={{ mb: 2 }}>
            Clusters
          </Typography>
          <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap' sx={{ mb: 4 }}>
            {clusters.map((s) => (
              <SpaceCard key={s.Space?.SpaceID} extended={s} />
            ))}
          </Stack>
        </>
      )}

      {platform.length > 0 && (
        <>
          <Stack direction='row' spacing={2} alignItems='center' sx={{ mb: 2 }}>
            <Typography variant='h5'>Base &amp; policy</Typography>
            {baseSpace?.Space?.SpaceID !== undefined && (
              <Button size='small' variant='outlined' onClick={() => setPersonaOpen(true)}>
                New persona…
              </Button>
            )}
          </Stack>
          <Stack direction='row' spacing={2} useFlexGap flexWrap='wrap'>
            {platform.map((s) => (
              <SpaceCard key={s.Space?.SpaceID} extended={s} />
            ))}
          </Stack>
        </>
      )}

      {baseSpace?.Space?.SpaceID !== undefined && (
        <NewPersonaDialog
          open={personaOpen}
          baseSpaceId={baseSpace.Space.SpaceID}
          onClose={(created) => {
            setPersonaOpen(false);
            if (created) void refetch();
          }}
        />
      )}
    </Container>
  );
}
