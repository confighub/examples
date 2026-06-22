import {
  Box,
  Card,
  CardContent,
  Chip,
  Divider,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { PromoteButton } from './PromoteButton';
import { Catalog, VariantRef } from '../data/catalog';
import { statusProvider } from '../model/status';
import { PromotionState, Stage, Workflow } from '../model/workflow';

const STATE_COLOR: Record<PromotionState, 'default' | 'success' | 'error' | 'warning'> = {
  unknown: 'default',
  pending: 'warning',
  succeeded: 'success',
  failed: 'error',
};

const SETTABLE: PromotionState[] = ['pending', 'succeeded', 'failed', 'unknown'];

/** A clickable status chip; the manual provider lets the user set state. */
function StatusChip({
  state,
  canEdit,
  onSet,
}: {
  state: PromotionState;
  canEdit: boolean;
  onSet: (s: PromotionState) => void;
}) {
  const [anchor, setAnchor] = useState<null | HTMLElement>(null);
  const chip = (
    <Chip
      size='small'
      label={state}
      color={STATE_COLOR[state]}
      variant={state === 'unknown' ? 'outlined' : 'filled'}
      onClick={canEdit ? (e) => setAnchor(e.currentTarget) : undefined}
    />
  );
  if (!canEdit) return chip;
  return (
    <>
      {chip}
      <Menu anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)}>
        {SETTABLE.map((s) => (
          <MenuItem
            key={s}
            selected={s === state}
            onClick={() => {
              setAnchor(null);
              onSet(s);
            }}
          >
            Mark {s}
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}

export function StageColumn({
  stage,
  stageIndex,
  prevStage,
  workflow,
  catalog,
  onPromoted,
  onSetStatus,
}: {
  stage: Stage;
  stageIndex: number;
  prevStage: Stage | undefined;
  workflow: Workflow;
  catalog: Catalog;
  onPromoted: (
    component: string,
    state: PromotionState,
    revision?: number,
  ) => Promise<void>;
  onSetStatus: (component: string, state: PromotionState) => Promise<void>;
}) {
  return (
    <Card variant='outlined' sx={{ minWidth: 280, flexShrink: 0 }}>
      <CardContent>
        <Typography variant='overline' color='text.secondary'>
          Stage {stageIndex + 1}
        </Typography>
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
            const target: VariantRef | undefined = catalog.resolve(choice.component, choice.variant);
            const upstreamChoice = prevStage?.components.find(
              (c) => c.component === choice.component,
            );
            const upstream = upstreamChoice
              ? catalog.resolve(upstreamChoice.component, upstreamChoice.variant)
              : undefined;
            const status = statusProvider.get(workflow, stage.name, choice.component);

            return (
              <Box key={i}>
                <Stack direction='row' alignItems='center' spacing={1}>
                  <Typography variant='subtitle2' sx={{ flexGrow: 1 }}>
                    {choice.component}
                  </Typography>
                  <StatusChip
                    state={status.state}
                    canEdit={statusProvider.canEdit}
                    onSet={(s) => onSetStatus(choice.component, s)}
                  />
                </Stack>
                <Stack direction='row' alignItems='center' spacing={1} sx={{ mt: 0.5 }}>
                  <Tooltip title={target ? target.spaceSlug : 'variant not found in catalog'}>
                    <Chip size='small' variant='outlined' label={choice.variant || '—'} />
                  </Tooltip>
                  <Box sx={{ flexGrow: 1 }} />
                  {stageIndex > 0 && (
                    <PromoteButton
                      target={target}
                      upstream={upstream}
                      onPromoted={(state, revision) => onPromoted(choice.component, state, revision)}
                    />
                  )}
                </Stack>
                {status.promotedRevision !== undefined && (
                  <Typography variant='caption' color='text.secondary'>
                    rev {status.promotedRevision}
                    {status.by ? ` · ${status.by}` : ''}
                  </Typography>
                )}
              </Box>
            );
          })}
        </Stack>
      </CardContent>
    </Card>
  );
}
