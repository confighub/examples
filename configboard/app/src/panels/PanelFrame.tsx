import BookmarkAddIcon from '@mui/icons-material/BookmarkAdd';
import CodeIcon from '@mui/icons-material/Code';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import TableChartIcon from '@mui/icons-material/TableChart';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Card from '@mui/material/Card';
import CircularProgress from '@mui/material/CircularProgress';
import Collapse from '@mui/material/Collapse';
import IconButton from '@mui/material/IconButton';
import Stack from '@mui/material/Stack';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import type { ReactNode } from 'react';
import { useState } from 'react';

export interface PanelFrameProps {
  title: string;
  description?: string;
  /** The `cub` equivalent of this panel's query. */
  query: string;
  isLoading?: boolean;
  isFetching?: boolean;
  error?: string;
  /** Notes shown under the chart: exclusions, row budget warnings, empty states. */
  notes?: string[];
  chart: ReactNode;
  table: ReactNode;
  /** Set when the panel is waiting to be run; renders a run affordance instead. */
  held?: { label: string; onRun: () => void };
  /** Present when the panel's query can be promoted to a ConfigHub Filter. */
  onSaveAsFilter?: () => void;
}

/**
 * Chrome shared by every panel: title, the query behind it, a table toggle, and a
 * footer that states what the panel left out. A chart that silently drops rows is the
 * specific way this kind of tool loses trust, so exclusions are printed, not implied.
 */
export function PanelFrame({
  title,
  description,
  query,
  isLoading,
  isFetching,
  error,
  notes,
  chart,
  table,
  held,
  onSaveAsFilter,
}: PanelFrameProps) {
  const [showTable, setShowTable] = useState(false);
  const [showQuery, setShowQuery] = useState(false);

  return (
    <Card variant="outlined" sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1}>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600, lineHeight: 1.3 }}>
            {title}
          </Typography>
          {description && (
            <Typography variant="caption" color="text.secondary">
              {description}
            </Typography>
          )}
        </Box>
        <Stack direction="row" alignItems="center" spacing={0.5} sx={{ flexShrink: 0 }}>
          {isFetching && !isLoading && <CircularProgress size={14} />}
          <Tooltip title="Show the equivalent cub command">
            <IconButton size="small" onClick={() => setShowQuery((v) => !v)}>
              <CodeIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          {onSaveAsFilter && (
            <Tooltip title="Save this query as a ConfigHub Filter">
              <IconButton size="small" onClick={onSaveAsFilter}>
                <BookmarkAddIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          )}
          <Tooltip title={showTable ? 'Show chart' : 'Show table'}>
            <IconButton size="small" onClick={() => setShowTable((v) => !v)}>
              <TableChartIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Stack>
      </Stack>

      <Collapse in={showQuery}>
        <Box
          component="pre"
          sx={{
            mt: 1,
            mb: 0,
            p: 1,
            fontSize: 11,
            overflowX: 'auto',
            bgcolor: 'action.hover',
            borderRadius: 1,
          }}
        >
          {query}
        </Box>
      </Collapse>

      <Box sx={{ flex: 1, mt: 1, minHeight: 0 }}>
        {error ? (
          <Alert severity="error" variant="outlined">
            {error}
          </Alert>
        ) : held ? (
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1, py: 5 }}>
            <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center' }}>
              {held.label}
            </Typography>
            <Button size="small" variant="outlined" startIcon={<PlayArrowIcon />} onClick={held.onRun}>
              Run check
            </Button>
          </Box>
        ) : isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={22} />
          </Box>
        ) : showTable ? (
          table
        ) : (
          chart
        )}
      </Box>

      {notes && notes.length > 0 && (
        <Box sx={{ mt: 1 }}>
          {notes.map((note) => (
            <Typography key={note} variant="caption" color="text.secondary" display="block">
              {note}
            </Typography>
          ))}
        </Box>
      )}
    </Card>
  );
}
