// The explorer: schema sidebar (left) + FQL editor and results (main). Run a
// query → see the plan (which ConfigHub API calls it compiles to) and the
// result grid. Everything reads through the portable fql/ engine; the only
// app coupling is fqlTransport (REST over /api).

import {
  Alert,
  Box,
  Button,
  Chip,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { useCallback, useState } from 'react';

import { fqlTransport } from '../api/fqlTransport';
import { FqlError, planQuery, renderError, runQuery, type RunResult } from '../fql';
import { QueryEditor } from './QueryEditor';
import { TableSidebar } from './TableSidebar';

const STARTER = "SELECT unit, kind, metadata.name\nFROM resources\nWHERE kind = 'Deployment'";

const EXAMPLES: { label: string; query: string }[] = [
  { label: 'Deployments', query: STARTER },
  {
    label: 'Floating :latest images',
    query:
      "SELECT unit, `spec.template.spec.containers.*.image` AS image\nFROM resources\nWHERE `spec.template.spec.containers.*.image` LIKE '%:latest'",
  },
  {
    label: 'Critical (scanner verdict)',
    query:
      "SELECT unit,\n  metadata.annotations['sec-scanner.confighub.com/max-severity'] AS severity\nFROM resources\nWHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'",
  },
  { label: 'Drift (unapplied)', query: 'SELECT slug, space FROM units WHERE headRevisionNum > liveRevisionNum' },
  {
    label: 'Units by cluster',
    query: 'SELECT cluster, COUNT(*) AS units\nFROM units\nGROUP BY cluster\nORDER BY units DESC',
  },
  {
    label: 'Audit trail (revisions)',
    query:
      "SELECT unit, revisionNum, source, description\nFROM revisions\nWHERE space = 'sec-demo-dev'\nORDER BY revisionNum DESC LIMIT 20",
  },
  {
    label: 'Changed in last 24h',
    query:
      "SELECT unit, revisionNum, source, userId\nFROM revisions\nWHERE space = 'sec-demo-dev' AND createdAt > now() - interval '24h'\nORDER BY createdAt DESC",
  },
  {
    label: 'Resource at a past revision',
    query:
      "SELECT unit, kind, metadata.name,\n  `spec.template.spec.containers.*.image` AS image\nFROM resources\nWHERE unit = 'legacy-frontend' AND revision = 1",
  },
  { label: 'Prod spaces', query: "SELECT slug FROM spaces WHERE labels.env = 'prod'" },
  {
    label: 'Who can delete pods',
    query:
      "SELECT subject, cluster, role, scope\nFROM grants\nWHERE verb = 'delete' AND resource = 'pods'\nORDER BY cluster",
  },
  {
    label: 'Cluster-admin holders',
    query: "SELECT subject, cluster, binding\nFROM grants\nWHERE role = 'cluster-admin'",
  },
  {
    label: "A subject's access",
    query: "SELECT cluster, role, scope\nFROM grants\nWHERE subject = 'Group:developers'",
  },
  {
    label: 'Wildcard roles',
    query: 'SELECT cluster, name, kind\nFROM roles\nWHERE hasWildcard = true',
  },
  {
    label: 'Orphaned bindings',
    query: 'SELECT cluster, name, roleRef\nFROM bindings\nWHERE orphaned = true',
  },
];

function PlanView({ query }: { query: string }) {
  try {
    const plan = planQuery(query);
    return (
      <Paper variant='outlined' sx={{ p: 1.5, mb: 2, bgcolor: 'grey.50' }}>
        <Typography variant='caption' color='text.secondary'>
          {plan.source} · {plan.fetches.length} server fetch(es)
          {plan.fetches.length > 1 ? ' (OR split, unioned)' : ''} · full WHERE re-checked client-side
        </Typography>
        {plan.fetches.map((f, i) => (
          <Box key={i} component='pre' sx={{ m: 0, mt: 0.5, fontSize: 12, whiteSpace: 'pre-wrap' }}>
            {`#${i + 1}  ` +
              (Object.keys(f).length === 0
                ? '(no pushdown — fetch all, filter client-side)'
                : [
                    f.where && `where:         ${f.where}`,
                    f.whereData && `where_data:    ${f.whereData}`,
                    f.whereResource && `whereResource: ${f.whereResource}`,
                  ]
                    .filter(Boolean)
                    .join('\n     '))}
          </Box>
        ))}
      </Paper>
    );
  } catch (e) {
    return (
      <Alert severity='warning' sx={{ mb: 2 }}>
        {e instanceof FqlError ? e.message : String(e)}
      </Alert>
    );
  }
}

function ResultGrid({ result }: { result: RunResult }) {
  if (result.rows.length === 0) {
    return <Alert severity='info'>No rows. ({result.stats.fetchedRows} fetched, all filtered out.)</Alert>;
  }
  return (
    <Box sx={{ overflow: 'auto' }}>
      <Typography variant='caption' color='text.secondary' sx={{ display: 'block', mb: 1 }}>
        {result.stats.resultRows} row(s) · {result.stats.fetchedRows} fetched ·{' '}
        {result.stats.fetches} API call(s)
      </Typography>
      <Table size='small' stickyHeader>
        <TableHead>
          <TableRow>
            {result.columns.map((c) => (
              <TableCell key={c} sx={{ fontWeight: 600, fontFamily: 'monospace' }}>
                {c}
              </TableCell>
            ))}
          </TableRow>
        </TableHead>
        <TableBody>
          {result.rows.map((row, i) => (
            <TableRow key={i} hover>
              {result.columns.map((c) => (
                <TableCell key={c}>
                  <Box component='code' sx={{ fontSize: 13 }}>
                    {row[c] == null ? '' : String(row[c])}
                  </Box>
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  );
}

export function ExplorerPage() {
  const [query, setQuery] = useState(STARTER);
  const [result, setResult] = useState<RunResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const run = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      setResult(await runQuery(query, fqlTransport));
    } catch (e) {
      setResult(null);
      setError(e instanceof FqlError ? renderError(query, e) : String((e as Error).message ?? e));
    } finally {
      setBusy(false);
    }
  }, [query]);

  // Sidebar click → append the token (the editor owns the caret).
  const insert = useCallback((token: string) => {
    setQuery((q) => (q.endsWith(' ') || q === '' ? q + token : q + ' ' + token));
  }, []);

  return (
    <Box sx={{ display: 'flex', height: '100%' }}>
      <Box sx={{ width: 260, flexShrink: 0 }}>
        <TableSidebar onInsert={insert} />
      </Box>

      <Box sx={{ flexGrow: 1, minWidth: 0, p: 2, overflow: 'auto' }}>
        <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap' sx={{ mb: 1.5 }}>
          {EXAMPLES.map((ex) => (
            <Chip key={ex.label} label={ex.label} size='small' onClick={() => setQuery(ex.query)} />
          ))}
        </Stack>

        <QueryEditor value={query} onChange={setQuery} onRun={() => void run()} />

        <Stack direction='row' spacing={2} alignItems='center' sx={{ my: 1.5 }}>
          <Button variant='contained' onClick={() => void run()} disabled={busy}>
            {busy ? 'Running…' : 'Run (⌘/Ctrl+Enter)'}
          </Button>
          <Typography variant='caption' color='text.secondary'>
            Tab/Enter accepts a completion · Esc dismisses
          </Typography>
        </Stack>

        <PlanView query={query} />

        {error !== null && (
          <Alert severity='error' sx={{ mb: 2 }} onClose={() => setError(null)}>
            <Box component='pre' sx={{ m: 0, fontFamily: 'monospace', fontSize: 13, whiteSpace: 'pre-wrap' }}>
              {error}
            </Box>
          </Alert>
        )}

        {result !== null && <ResultGrid result={result} />}
      </Box>
    </Box>
  );
}
