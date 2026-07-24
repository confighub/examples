import { useListAllTargetsQuery, useListSpacesQuery } from '@confighub/rtk-query';
import RefreshIcon from '@mui/icons-material/Refresh';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import { useMemo } from 'react';

import type { Variable } from '../model/types';
import { ALL_VALUE, type Scope } from '../query/compile';

const TIME_RANGES = [
  { value: '24h', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
];

export interface ScopeBarProps {
  variables: Variable[];
  scope: Scope;
  onChange: (name: string, value: string) => void;
  onRefresh: () => void;
}

/**
 * The filter row. Distinct values come from the Space list every dashboard already
 * fetches and the Target list, so opening a dashboard costs two small requests before
 * any panel runs.
 */
export function ScopeBar({ variables, scope, onChange, onRefresh }: ScopeBarProps) {
  const needsSpaces = variables.some((v) => v.from?.spaceLabel);
  const needsTargets = variables.some((v) => v.from?.target);

  const { data: spaces } = useListSpacesQuery({ summary: true }, { skip: !needsSpaces });
  const { data: targets } = useListAllTargetsQuery({}, { skip: !needsTargets });

  const optionsFor = useMemo(() => {
    return (variable: Variable): string[] => {
      if (variable.from?.spaceLabel) {
        const key = variable.from.spaceLabel;
        const values = new Set<string>();
        for (const s of spaces ?? []) {
          const value = s.Space?.Labels?.[key];
          if (value) values.add(value);
        }
        return [...values].sort();
      }
      if (variable.from?.target) {
        const values = new Set<string>();
        for (const t of targets ?? []) {
          if (t.Target?.Slug) values.add(t.Target.Slug);
        }
        return [...values].sort();
      }
      return [];
    };
  }, [spaces, targets]);

  if (variables.length === 0) {
    return null;
  }

  return (
    <Stack direction="row" spacing={1.5} alignItems="center" sx={{ flexWrap: 'wrap', gap: 1 }}>
      {variables.map((variable) => {
        const value = scope[variable.name] ?? (variable.allValue ? ALL_VALUE : '');
        const options =
          variable.type === 'timeRange'
            ? TIME_RANGES
            : optionsFor(variable).map((o) => ({ value: o, label: o }));

        return (
          <TextField
            key={variable.name}
            select
            size="small"
            label={variable.label}
            value={value}
            onChange={(e) => onChange(variable.name, e.target.value)}
            sx={{ minWidth: 170 }}
          >
            {variable.allValue && <MenuItem value={ALL_VALUE}>All</MenuItem>}
            {options.map((o) => (
              <MenuItem key={o.value} value={o.value}>
                {o.label}
              </MenuItem>
            ))}
          </TextField>
        );
      })}
      <Box sx={{ flex: 1 }} />
      <Tooltip title="Refresh">
        <IconButton size="small" onClick={onRefresh}>
          <RefreshIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </Stack>
  );
}
