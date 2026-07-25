import { confighubApi } from '@confighub/rtk-query';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import Grid from '@mui/material/Grid2';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useCallback, useMemo, useState } from 'react';
import { useDispatch } from 'react-redux';

import type { Dashboard } from '../model/types';
import { PanelRenderer } from '../panels/PanelRenderer';
import { ALL_VALUE, type Scope } from '../query/compile';
import {
  type CrossFilter,
  addCrossFilter,
  crossFilterKey,
  removeCrossFilter,
} from '../query/crossFilter';
import { ScopeBar } from './ScopeBar';
import { BASE_URL } from './config';
import type { AppDispatch } from './store';

export interface DashboardViewProps {
  dashboard: Dashboard;
  errors: string[];
}

function initialScope(dashboard: Dashboard): Scope {
  const scope: Scope = {};
  for (const v of dashboard.variables ?? []) {
    scope[v.name] = v.default ?? (v.allValue ? ALL_VALUE : undefined);
  }
  return scope;
}

export function DashboardView({ dashboard, errors }: DashboardViewProps) {
  const dispatch = useDispatch<AppDispatch>();
  const [scope, setScope] = useState<Scope>(() => initialScope(dashboard));
  const [crossFilters, setCrossFilters] = useState<CrossFilter[]>([]);

  const onChange = useCallback((name: string, value: string) => {
    setScope((prev) => ({ ...prev, [name]: value }));
  }, []);

  const onCrossFilter = useCallback((filter: CrossFilter) => {
    setCrossFilters((prev) => addCrossFilter(prev, filter));
  }, []);

  // Config changes on human timescales, so refresh is a button rather than a poll.
  const onRefresh = useCallback(() => {
    dispatch(confighubApi.util.invalidateTags([]));
    dispatch(confighubApi.util.resetApiState());
  }, [dispatch]);

  const variables = useMemo(() => dashboard.variables ?? [], [dashboard.variables]);

  return (
    <Box>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          {dashboard.title}
        </Typography>
        {dashboard.description && (
          <Typography variant="body2" color="text.secondary">
            {dashboard.description}
          </Typography>
        )}
      </Box>

      <Box sx={{ mb: 2 }}>
        <ScopeBar
          variables={variables}
          scope={scope}
          onChange={onChange}
          onRefresh={onRefresh}
        />
      </Box>

      {crossFilters.length > 0 && (
        <Stack direction="row" spacing={1} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          {crossFilters.map((filter) => (
            <Chip
              key={crossFilterKey(filter)}
              label={filter.label}
              size="small"
              onDelete={() => setCrossFilters((prev) => removeCrossFilter(prev, filter))}
            />
          ))}
          <Chip
            label="Clear all"
            size="small"
            variant="outlined"
            onClick={() => setCrossFilters([])}
          />
        </Stack>
      )}

      {errors.length > 0 && (
        <Alert severity="warning" variant="outlined" sx={{ mb: 2 }}>
          {errors.map((e) => (
            <div key={e}>{e}</div>
          ))}
        </Alert>
      )}

      <Grid container spacing={2}>
        {dashboard.panels.map((panel) => (
          <Grid key={panel.id} size={{ xs: 12, md: panel.span ?? 6 }}>
            <PanelRenderer
              panel={panel}
              scope={scope}
              baseUrl={BASE_URL}
              crossFilters={crossFilters}
              onCrossFilter={onCrossFilter}
            />
          </Grid>
        ))}
      </Grid>
    </Box>
  );
}
