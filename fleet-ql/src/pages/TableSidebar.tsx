// Left panel: the virtual-table catalog. Each table expands to its columns;
// clicking a table or column inserts its name into the editor at the caret
// (well, appends — the editor owns the caret, so we hand the token up and let
// the page decide). A database-explorer's schema tree.

import {
  Box,
  Chip,
  Collapse,
  List,
  ListItemButton,
  ListItemText,
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { describeTables } from '../fql';

export interface TableSidebarProps {
  /** Called when a table or column name is clicked, to insert into the editor. */
  onInsert: (token: string) => void;
}

export function TableSidebar({ onInsert }: TableSidebarProps) {
  const tables = describeTables();
  const [openTable, setOpenTable] = useState<string | null>('resources');

  return (
    <Box sx={{ height: '100%', overflow: 'auto', bgcolor: 'grey.50', borderRight: 1, borderColor: 'divider' }}>
      <Typography variant='overline' sx={{ px: 2, pt: 1.5, display: 'block', color: 'text.secondary' }}>
        Tables
      </Typography>
      <List dense disablePadding>
        {tables.map((t) => {
          const isOpen = openTable === t.name;
          return (
            <Box key={t.name}>
              <ListItemButton onClick={() => setOpenTable(isOpen ? null : t.name)}>
                <Box component='span' sx={{ width: 18, color: 'text.secondary', fontSize: 12 }}>
                  {isOpen ? '▾' : '▸'}
                </Box>
                <ListItemText
                  primary={t.name}
                  primaryTypographyProps={{ fontFamily: 'monospace', fontSize: 14 }}
                />
              </ListItemButton>
              <Collapse in={isOpen} timeout='auto' unmountOnExit>
                <List dense disablePadding>
                  {t.columns.map((c) => (
                    <ListItemButton
                      key={c.name}
                      sx={{ pl: 4, py: 0.25 }}
                      onClick={() => onInsert(c.name)}
                      title={c.pushdown ? `pushes down (${c.pushdown})` : 'evaluated client-side'}
                    >
                      <ListItemText
                        primary={c.name}
                        primaryTypographyProps={{ fontFamily: 'monospace', fontSize: 12.5 }}
                      />
                      <Chip
                        label={c.type}
                        size='small'
                        variant='outlined'
                        sx={{ height: 18, fontSize: 10, '& .MuiChip-label': { px: 0.75 } }}
                      />
                    </ListItemButton>
                  ))}
                  {t.rawDataPaths && (
                    <ListItemButton sx={{ pl: 4, py: 0.25 }} onClick={() => onInsert('`spec.`')}>
                      <ListItemText
                        primary='`any.yaml.path`'
                        secondary='raw resource path'
                        primaryTypographyProps={{ fontFamily: 'monospace', fontSize: 12.5 }}
                        secondaryTypographyProps={{ fontSize: 10 }}
                      />
                    </ListItemButton>
                  )}
                </List>
              </Collapse>
            </Box>
          );
        })}
      </List>
    </Box>
  );
}
