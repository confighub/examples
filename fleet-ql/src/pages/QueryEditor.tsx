// A lightweight FQL editor: a monospace textarea with a token-based
// autocomplete popup (keywords, table names, and the columns of whatever table
// the query is FROM). Zero editor dependencies — just a textarea + a positioned
// MUI list. Ctrl/Cmd+Enter runs; Tab/Enter accepts a completion; Esc dismisses.

import { Box, List, ListItemButton, ListItemText, Paper } from '@mui/material';
import { useMemo, useRef, useState } from 'react';

import { type Completion, completionsAt, currentWord } from '../fql';

type Suggestion = Completion;

export interface QueryEditorProps {
  value: string;
  onChange: (v: string) => void;
  onRun: () => void;
}

export function QueryEditor({ value, onChange, onRun }: QueryEditorProps) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [caret, setCaret] = useState(0);

  // The contextual candidate pool: only what's grammatical at the caret
  // (clause-aware — see fql/complete.ts).
  const pool = useMemo<Suggestion[]>(() => completionsAt(value, caret), [value, caret]);

  const { word, start } = currentWord(value, caret);
  const matches = useMemo(() => {
    const w = word.toLowerCase();
    // Empty word → show the whole contextual pool (e.g. the SELECT fields right
    // after typing "GROUP BY "); otherwise prefix-filter it.
    const filtered =
      w === ''
        ? pool
        : pool.filter((s) => s.label.toLowerCase().startsWith(w) && s.label.toLowerCase() !== w);
    return filtered.slice(0, 8);
  }, [pool, word]);

  const showPopup = open && matches.length > 0;

  const accept = (s: Suggestion) => {
    const insert = s.insert ?? s.label;
    const next = value.slice(0, start) + insert + value.slice(caret);
    onChange(next);
    setOpen(false);
    // Restore focus + place caret after the inserted token.
    requestAnimationFrame(() => {
      const el = ref.current;
      if (el) {
        const pos = start + insert.length;
        el.focus();
        el.setSelectionRange(pos, pos);
        setCaret(pos);
      }
    });
  };

  return (
    <Box sx={{ position: 'relative' }}>
      <Box
        component='textarea'
        ref={ref}
        value={value}
        spellCheck={false}
        rows={8}
        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => {
          onChange(e.target.value);
          setCaret(e.target.selectionStart ?? 0);
          setOpen(true);
        }}
        onKeyUp={(e: React.KeyboardEvent<HTMLTextAreaElement>) =>
          setCaret((e.target as HTMLTextAreaElement).selectionStart ?? 0)
        }
        onClick={(e: React.MouseEvent<HTMLTextAreaElement>) =>
          setCaret((e.target as HTMLTextAreaElement).selectionStart ?? 0)
        }
        onKeyDown={(e: React.KeyboardEvent<HTMLTextAreaElement>) => {
          if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
            e.preventDefault();
            setOpen(false);
            onRun();
            return;
          }
          if (!showPopup) return;
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActive((a) => (a + 1) % matches.length);
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActive((a) => (a - 1 + matches.length) % matches.length);
          } else if (e.key === 'Enter' || e.key === 'Tab') {
            e.preventDefault();
            accept(matches[Math.min(active, matches.length - 1)]);
          } else if (e.key === 'Escape') {
            e.preventDefault();
            setOpen(false);
          }
        }}
        sx={{
          width: '100%',
          fontFamily: 'monospace',
          fontSize: 14,
          lineHeight: 1.5,
          p: 1.5,
          boxSizing: 'border-box',
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          resize: 'vertical',
          outline: 'none',
        }}
      />
      {showPopup && (
        <Paper
          elevation={4}
          sx={{ position: 'absolute', zIndex: 10, mt: 0.5, left: 8, minWidth: 260, maxWidth: 360 }}
        >
          <List dense disablePadding>
            {matches.map((s, i) => (
              <ListItemButton
                key={s.label}
                selected={i === Math.min(active, matches.length - 1)}
                onMouseDown={(e) => {
                  e.preventDefault(); // keep textarea focus
                  accept(s);
                }}
              >
                <ListItemText
                  primary={s.label}
                  secondary={s.detail}
                  primaryTypographyProps={{ fontFamily: 'monospace', fontSize: 13 }}
                  secondaryTypographyProps={{ fontSize: 11 }}
                />
              </ListItemButton>
            ))}
          </List>
        </Paper>
      )}
    </Box>
  );
}
