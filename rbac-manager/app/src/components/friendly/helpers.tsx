// Building blocks for the friendly resource views.

import {
  Box,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { ReactNode } from 'react';

export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant='subtitle2' sx={{ mb: 0.5, color: 'text.secondary' }}>
        {title}
      </Typography>
      {children}
    </Box>
  );
}

export function EmptyHint({ text }: { text: string }) {
  return (
    <Typography variant='body2' color='text.secondary' sx={{ fontStyle: 'italic' }}>
      {text}
    </Typography>
  );
}

export interface Field {
  label: string;
  value: ReactNode;
}

export function FieldList({ fields }: { fields: Field[] }) {
  return (
    <Table size='small'>
      <TableBody>
        {fields
          .filter((f) => f.value !== undefined && f.value !== null && f.value !== '')
          .map((f) => (
            <TableRow key={f.label}>
              <TableCell sx={{ width: 140, color: 'text.secondary', border: 0, py: 0.25 }}>
                {f.label}
              </TableCell>
              <TableCell sx={{ border: 0, py: 0.25 }}>{f.value}</TableCell>
            </TableRow>
          ))}
      </TableBody>
    </Table>
  );
}

export function MiniTable({ columns, rows }: { columns: string[]; rows: ReactNode[][] }) {
  return (
    <Table size='small'>
      <TableHead>
        <TableRow>
          {columns.map((c) => (
            <TableCell key={c} sx={{ color: 'text.secondary' }}>
              {c}
            </TableCell>
          ))}
        </TableRow>
      </TableHead>
      <TableBody>
        {rows.map((cells, i) => (
          <TableRow key={i}>
            {cells.map((cell, j) => (
              <TableCell key={j} sx={{ verticalAlign: 'top' }}>
                {cell}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

/** Verbs that enable privilege escalation get the loudest treatment. */
const DANGEROUS_VERBS = new Set(['escalate', 'bind', 'impersonate']);
const WRITE_VERBS = new Set(['delete', 'deletecollection']);

export function VerbChip({ verb }: { verb: string }) {
  const color = verb === '*' || DANGEROUS_VERBS.has(verb) ? 'error' : WRITE_VERBS.has(verb) ? 'warning' : 'default';
  return <Chip size='small' label={verb} color={color} variant={color === 'default' ? 'outlined' : 'filled'} sx={{ mr: 0.5, mb: 0.5 }} />;
}

export function VerbChips({ verbs }: { verbs: string[] }) {
  return (
    <Box>
      {verbs.map((v) => (
        <VerbChip key={v} verb={v} />
      ))}
    </Box>
  );
}

/** Render a resource/apiGroup token, highlighting wildcards. */
export function Token({ value, core }: { value: string; core?: boolean }) {
  if (value === '*') {
    return <Chip size='small' color='error' label='* (all)' sx={{ mr: 0.5, mb: 0.5 }} />;
  }
  return (
    <Chip
      size='small'
      variant='outlined'
      label={core === true && value === '' ? '(core)' : value}
      sx={{ mr: 0.5, mb: 0.5 }}
    />
  );
}

export function TokenList({ values, core }: { values: string[]; core?: boolean }) {
  if (values.length === 0) return <EmptyHint text='—' />;
  return (
    <Box>
      {values.map((v, i) => (
        <Token key={`${v}-${i}`} value={v} core={core} />
      ))}
    </Box>
  );
}
