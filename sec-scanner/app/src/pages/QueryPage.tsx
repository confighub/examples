// FQL console: a SQL-like REPL over the fleet. Type a query, Run, see the
// result grid. Errors render with a caret under the offending token; the
// "Show plan" toggle reveals which ConfigHub API calls FQL will issue (the
// pushdown), making the compile step legible. The engine lives in ../fql; this
// page only wires it to the fqlTransport and renders.

import {
  Alert,
  Box,
  Button,
  Chip,
  Container,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { useState } from 'react';

import { fqlTransport } from '../api/fqlTransport';
import { FqlError, planQuery, renderError, runQuery, type RunResult } from '../fql';

const EXAMPLES: { label: string; query: string }[] = [
  { label: 'Units in the demo fleet', query: "SELECT slug, space, toolchain\nFROM units\nWHERE space LIKE 'sec-demo-%'" },
  {
    label: 'Critical or floating-tag images',
    query:
      "SELECT unit, image, severity\nFROM resources\nWHERE kind = 'Deployment'\n  AND (severity = 'CRITICAL' OR image ~ ':latest')\nORDER BY severity LIMIT 20",
  },
  {
    label: 'Critical count per space',
    query: "SELECT space, COUNT(*) AS n\nFROM resources\nWHERE severity = 'CRITICAL'\nGROUP BY space ORDER BY n DESC",
  },
  { label: 'Prod spaces', query: "SELECT slug FROM spaces WHERE labels.env = 'prod'" },
];

/** The compiled plan, rendered as the list of API calls FQL will make. */
function PlanView({ query }: { query: string }) {
  let body: React.ReactNode;
  try {
    const plan = planQuery(query);
    body = (
      <>
        <Typography variant='body2' color='text.secondary'>
          Source table: <Box component='code'>{plan.source}</Box> ·{' '}
          {plan.fetches.length} server fetch(es){plan.fetches.length > 1 ? ' (OR split, unioned)' : ''}
        </Typography>
        {plan.fetches.map((f, i) => (
          <Box key={i} component='pre' sx={{ m: 0, mt: 1, fontSize: 12, whiteSpace: 'pre-wrap' }}>
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
        <Typography variant='caption' color='text.secondary' sx={{ display: 'block', mt: 1 }}>
          The full WHERE is always re-evaluated client-side; pushdown only narrows the fetch.
        </Typography>
      </>
    );
  } catch (e) {
    body = (
      <Typography variant='body2' color='error'>
        {e instanceof FqlError ? e.message : String(e)}
      </Typography>
    );
  }
  return (
    <Paper variant='outlined' sx={{ p: 2, mb: 2, bgcolor: 'grey.50' }}>
      {body}
    </Paper>
  );
}

function ResultGrid({ result }: { result: RunResult }) {
  if (result.rows.length === 0) {
    return <Alert severity='info'>No rows. ({result.stats.fetchedRows} fetched, all filtered out.)</Alert>;
  }
  return (
    <>
      <Typography variant='caption' color='text.secondary' sx={{ display: 'block', mb: 1 }}>
        {result.stats.resultRows} row(s) · {result.stats.fetchedRows} fetched · {result.stats.fetches} API call(s)
      </Typography>
      <Table size='small'>
        <TableHead>
          <TableRow>
            {result.columns.map((c) => (
              <TableCell key={c} sx={{ fontWeight: 600 }}>
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
    </>
  );
}

export function QueryPage() {
  const [query, setQuery] = useState(EXAMPLES[1].query);
  const [result, setResult] = useState<RunResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showPlan, setShowPlan] = useState(false);

  const run = async () => {
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
  };

  return (
    <Container maxWidth='lg' sx={{ mt: 3 }}>
      <Typography variant='h5' gutterBottom>
        Fleet Query
      </Typography>
      <Typography variant='body2' color='text.secondary' sx={{ mb: 2 }}>
        A SQL-like query language over the fleet. Tables: <Box component='code'>units</Box>,{' '}
        <Box component='code'>resources</Box>, <Box component='code'>spaces</Box>,{' '}
        <Box component='code'>targets</Box>.
      </Typography>

      <Stack direction='row' spacing={1} useFlexGap flexWrap='wrap' sx={{ mb: 1.5 }}>
        {EXAMPLES.map((ex) => (
          <Chip key={ex.label} label={ex.label} size='small' onClick={() => setQuery(ex.query)} />
        ))}
      </Stack>

      <Box
        component='textarea'
        value={query}
        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setQuery(e.target.value)}
        onKeyDown={(e: React.KeyboardEvent) => {
          if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') void run();
        }}
        spellCheck={false}
        rows={7}
        sx={{
          width: '100%',
          fontFamily: 'monospace',
          fontSize: 14,
          p: 1.5,
          boxSizing: 'border-box',
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          resize: 'vertical',
        }}
      />

      <Stack direction='row' spacing={2} alignItems='center' sx={{ mt: 1.5, mb: 2 }}>
        <Button variant='contained' onClick={() => void run()} disabled={busy}>
          {busy ? 'Running…' : 'Run (⌘/Ctrl+Enter)'}
        </Button>
        <Button size='small' onClick={() => setShowPlan((s) => !s)}>
          {showPlan ? 'Hide plan' : 'Show plan'}
        </Button>
      </Stack>

      {showPlan && <PlanView query={query} />}

      {error !== null && (
        <Alert severity='error' sx={{ mb: 2 }} onClose={() => setError(null)}>
          <Box component='pre' sx={{ m: 0, fontFamily: 'monospace', fontSize: 13, whiteSpace: 'pre-wrap' }}>
            {error}
          </Box>
        </Alert>
      )}

      {result !== null && <ResultGrid result={result} />}
    </Container>
  );
}
