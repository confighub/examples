import { Box, Card, CardContent, Chip, Divider, Stack, Tooltip, Typography } from '@mui/material';

import { PromoteButton } from './PromoteButton';
import { Catalog, VariantRef } from '../data/catalog';
import { rollupStatus, statusProvider } from '../model/status';
import { PromotionState, Stage } from '../model/workflow';

const STATE_COLOR: Record<PromotionState, 'default' | 'success' | 'error' | 'warning' | 'info'> = {
  unknown: 'default',
  pending: 'default',
  in_progress: 'info',
  succeeded: 'success',
  failed: 'error',
};

const STATE_LABEL: Record<PromotionState, string> = {
  unknown: 'no status',
  pending: 'pending',
  in_progress: 'in progress',
  succeeded: 'ready',
  failed: 'failed',
};

/** State of one component in a stage, from its variant Space's status label. */
function componentState(
  catalog: Catalog,
  statusLabel: string,
  component: string,
  variant: string,
): { ref: VariantRef | undefined; state: PromotionState; raw: string | undefined } {
  const ref = catalog.resolve(component, variant);
  const raw = statusProvider.raw(ref?.labels, statusLabel);
  return { ref, state: statusProvider.get(ref?.labels, statusLabel), raw };
}

/** Roll a whole stage up to one state (used for the header and gating). */
export function stageState(stage: Stage, catalog: Catalog, statusLabel: string): PromotionState {
  return rollupStatus(
    stage.components.map((c) => componentState(catalog, statusLabel, c.component, c.variant).state),
  );
}

export function StageColumn({
  stage,
  stageIndex,
  prevStage,
  statusLabel,
  catalog,
  onPromoted,
}: {
  stage: Stage;
  stageIndex: number;
  prevStage: Stage | undefined;
  statusLabel: string;
  catalog: Catalog;
  onPromoted: () => void;
}) {
  const rollup = stageState(stage, catalog, statusLabel);
  // The promotion gate opens only once the upstream stage is fully ready.
  const upstreamState = prevStage ? stageState(prevStage, catalog, statusLabel) : undefined;
  const upstreamReady = upstreamState === 'succeeded';

  return (
    <Card variant='outlined' sx={{ minWidth: 280, flexShrink: 0 }}>
      <CardContent>
        <Stack direction='row' alignItems='center' spacing={1}>
          <Typography variant='overline' color='text.secondary' sx={{ flexGrow: 1 }}>
            Stage {stageIndex + 1}
          </Typography>
          <Chip
            size='small'
            label={STATE_LABEL[rollup]}
            color={STATE_COLOR[rollup]}
            variant={rollup === 'unknown' || rollup === 'pending' ? 'outlined' : 'filled'}
          />
        </Stack>
        <Typography variant='h6' gutterBottom>
          {stage.name}
        </Typography>
        <Divider sx={{ mb: 1.5 }} />

        {stage.components.length === 0 && (
          <Typography variant='body2' color='text.secondary'>
            No components.
          </Typography>
        )}

        <Stack spacing={2}>
          {stage.components.map((choice, i) => {
            const { ref, state, raw } = componentState(
              catalog,
              statusLabel,
              choice.component,
              choice.variant,
            );
            const upstreamChoice = prevStage?.components.find(
              (c) => c.component === choice.component,
            );
            const upstream = upstreamChoice
              ? catalog.resolve(upstreamChoice.component, upstreamChoice.variant)
              : undefined;

            return (
              <Box key={i}>
                <Stack direction='row' alignItems='center' spacing={1}>
                  <Typography variant='subtitle2' sx={{ flexGrow: 1 }}>
                    {choice.component}
                  </Typography>
                  <Tooltip
                    title={
                      raw !== undefined
                        ? `${statusLabel}=${raw}`
                        : `no ${statusLabel} label on ${ref?.spaceSlug ?? 'variant'}`
                    }
                  >
                    <Chip
                      size='small'
                      label={STATE_LABEL[state]}
                      color={STATE_COLOR[state]}
                      variant={state === 'unknown' || state === 'pending' ? 'outlined' : 'filled'}
                    />
                  </Tooltip>
                </Stack>
                <Stack direction='row' alignItems='center' spacing={1} sx={{ mt: 0.5 }}>
                  <Tooltip title={ref ? ref.spaceSlug : 'variant not found in catalog'}>
                    <Chip size='small' variant='outlined' label={choice.variant || '—'} />
                  </Tooltip>
                  <Box sx={{ flexGrow: 1 }} />
                  {stageIndex > 0 && (
                    <PromoteButton
                      target={ref}
                      upstream={upstream}
                      blockedReason={
                        upstreamReady
                          ? undefined
                          : `Upstream stage “${prevStage?.name}” is not ready (${
                              upstreamState ? STATE_LABEL[upstreamState] : 'unknown'
                            }).`
                      }
                      onPromoted={onPromoted}
                    />
                  )}
                </Stack>
              </Box>
            );
          })}
        </Stack>
      </CardContent>
    </Card>
  );
}
