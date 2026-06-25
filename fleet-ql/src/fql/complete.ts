// Context-aware completion for the FQL editor. Rather than a flat pool offered
// everywhere, we derive the *clause* and what's grammatical next from the tokens
// before the caret (an error-tolerant lex + a small state machine) and suggest
// only that. This is "parser context" without a full AST — the query is usually
// mid-typed and invalid, so we never run the real parser here.
//
// Rules:
//  - after FROM  → table names only
//  - WHERE       → columns → operators → value help → AND/OR/clause
//  - GROUP BY    → only the SELECT clause's base columns
//  - ORDER BY    → only the SELECT clause's output columns, then ASC/DESC
//  - clause keywords are offered in grammar order only
//  - completion is suppressed inside a string / backtick literal

import { lex, type Token } from './lexer';
import { describeTable, tableNames } from './schema';

export interface Completion {
  label: string;
  detail?: string;
  /** Text inserted (defaults to label). */
  insert?: string;
}

const AGG = new Set(['COUNT', 'MAX', 'MIN', 'SUM', 'AVG']);

/** The word currently being typed, and its start offset. Dotted paths and
 *  brackets complete as one unit; we break on whitespace and structural punct. */
export function currentWord(text: string, caret: number): { word: string; start: number } {
  let start = caret;
  while (start > 0 && !/[\s(),]/.test(text[start - 1])) start--;
  return { word: text.slice(start, caret), start };
}

/** True when the caret sits inside an unterminated '…' string or `…` path. */
function inLiteral(text: string, caret: number): boolean {
  let str = false;
  let tick = false;
  for (let i = 0; i < caret; i++) {
    const c = text[i];
    if (str) {
      if (c === "'") {
        if (text[i + 1] === "'") i++; // '' escape
        else str = false;
      }
    } else if (tick) {
      if (c === '`') tick = false;
    } else if (c === "'") str = true;
    else if (c === '`') tick = true;
  }
  return str || tick;
}

function tolerantLex(src: string): Token[] | null {
  try {
    return lex(src).filter((t) => t.type !== 'eof');
  } catch {
    return null;
  }
}

const kw = (...words: string[]): Completion[] =>
  words.map((w) => ({ label: w, detail: 'keyword' }));

function tableCompletions(): Completion[] {
  return tableNames().map((t) => ({ label: t, detail: 'table' }));
}

function aggCompletions(): Completion[] {
  return [
    { label: 'COUNT(*)', detail: 'aggregate' },
    ...['MAX', 'MIN', 'SUM', 'AVG'].map((f) => ({
      label: `${f}(`,
      detail: 'aggregate',
      insert: `${f}(`,
    })),
  ];
}

/** Columns of the FROM table (+ a raw-path stub for resources). */
function columnCompletions(table: string | null): Completion[] {
  const info = table ? describeTable(table) : null;
  if (!info) return [];
  const out: Completion[] = info.columns.map((c) => ({
    label: c.name,
    detail: `${c.type}${c.pushdown ? '' : ' · client'}`,
  }));
  if (info.rawDataPaths) out.push({ label: '`spec.…`', detail: 'raw YAML path', insert: '`spec.' });
  return out;
}

// ─── Token reconstruction (column text + scope extraction) ────────────────────

function tokenText(t: Token): string {
  switch (t.type) {
    case 'ident':
      return t.quoted ? `\`${t.value}\`` : (t.raw ?? t.value);
    case 'keyword':
      return t.raw ?? t.value;
    case 'string':
      return `'${t.value}'`;
    case 'dot':
      return '.';
    case 'lbracket':
      return '[';
    case 'rbracket':
      return ']';
    case 'star':
      return '*';
    default:
      return t.value;
  }
}

const colText = (toks: Token[]): string => toks.map(tokenText).join('');

function aggName(toks: Token[]): string {
  const fn = toks[0].value.toLowerCase();
  const open = toks.findIndex((t) => t.type === 'lparen');
  const close = toks.length - 1; // assume trailing rparen
  const inner = open >= 0 ? toks.slice(open + 1, close) : [];
  const arg = inner.length === 1 && inner[0].type === 'star' ? '*' : colText(inner);
  return `${fn}(${arg})`;
}

/** Split a token list on top-level commas (depth tracked via parens). */
function splitTopComma(toks: Token[]): Token[][] {
  const parts: Token[][] = [];
  let cur: Token[] = [];
  let depth = 0;
  for (const t of toks) {
    if (t.type === 'lparen') depth++;
    else if (t.type === 'rparen') depth--;
    if (t.type === 'comma' && depth === 0) {
      parts.push(cur);
      cur = [];
    } else cur.push(t);
  }
  if (cur.length) parts.push(cur);
  return parts;
}

interface SelectScope {
  /** Base column names usable in GROUP BY (plain projected columns). */
  group: string[];
  /** Output names usable in ORDER BY (aliases / columns / agg outputs). */
  order: string[];
  /** True for `SELECT *` — no named outputs. */
  star: boolean;
}

/** Extract the SELECT projection scope from the full token list. */
function selectScope(tokens: Token[]): SelectScope {
  const sel = tokens.findIndex((t) => t.type === 'keyword' && t.value === 'SELECT');
  if (sel < 0) return { group: [], order: [], star: false };
  let end = tokens.findIndex((t, i) => i > sel && t.type === 'keyword' && t.value === 'FROM');
  if (end < 0) end = tokens.length;
  const body = tokens.slice(sel + 1, end);
  if (body.length === 1 && body[0].type === 'star') return { group: [], order: [], star: true };

  const group: string[] = [];
  const order: string[] = [];
  for (const part of splitTopComma(body)) {
    if (part.length === 0) continue;
    const asIdx = part.findIndex((t) => t.type === 'keyword' && t.value === 'AS');
    const base = asIdx >= 0 ? part.slice(0, asIdx) : part;
    const alias = asIdx >= 0 ? part[asIdx + 1] : undefined;
    if (base.length === 0) continue;
    const isAgg = base[0].type === 'keyword' && AGG.has(base[0].value);
    const isStar = base.length === 1 && base[0].type === 'star';
    if (alias) order.push(tokenText(alias));
    else if (isAgg) order.push(aggName(base));
    else if (!isStar) order.push(colText(base));
    if (!isAgg && !isStar) group.push(colText(base));
  }
  return { group, order, star: false };
}

// ─── Clause + sub-state detection ─────────────────────────────────────────────

type Clause = 'start' | 'select' | 'from' | 'where' | 'group' | 'order' | 'limit';

function clauseOf(tokens: Token[]): { clause: Clause; tail: Token[] } {
  let clause: Clause = 'start';
  let end = 0;
  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i];
    if (t.type !== 'keyword') continue;
    if (t.value === 'SELECT') (clause = 'select'), (end = i + 1);
    else if (t.value === 'FROM') (clause = 'from'), (end = i + 1);
    else if (t.value === 'WHERE') (clause = 'where'), (end = i + 1);
    else if (t.value === 'GROUP' && tokens[i + 1]?.value === 'BY') (clause = 'group'), (end = i + 2);
    else if (t.value === 'ORDER' && tokens[i + 1]?.value === 'BY') (clause = 'order'), (end = i + 2);
    else if (t.value === 'LIMIT') (clause = 'limit'), (end = i + 1);
  }
  return { clause, tail: tokens.slice(end) };
}

function tableOf(tokens: Token[]): string | null {
  const f = tokens.findIndex((t) => t.type === 'keyword' && t.value === 'FROM');
  const next = f >= 0 ? tokens[f + 1] : undefined;
  return next && next.type === 'ident' ? next.value.toLowerCase() : null;
}

const COMPARE = new Set(['=', '!=', '<', '>', '<=', '>=', '~', '~*', '!~', '!~*']);
const isKw = (t: Token | undefined, v: string) => !!t && t.type === 'keyword' && t.value === v;

// ─── Public entry ──────────────────────────────────────────────────────────────

/** Contextual completion candidates for the caret position (unfiltered by the
 *  in-progress word; the editor prefix-filters). Empty inside string literals. */
export function completionsAt(text: string, caret: number): Completion[] {
  if (inLiteral(text, caret)) return [];
  const { start } = currentWord(text, caret);
  const tokens = tolerantLex(text.slice(0, start));
  if (tokens === null) return [];

  const { clause, tail } = clauseOf(tokens);
  const table = tableOf(tokens);
  const last = tail[tail.length - 1];
  const prev = tail[tail.length - 2];

  switch (clause) {
    case 'start':
      return kw('SELECT');

    case 'select': {
      // Inside an aggregate's parens: COUNT( … → arg.
      const openAgg =
        last?.type === 'lparen' && prev?.type === 'keyword' && AGG.has(prev.value);
      if (openAgg) return [{ label: '*', detail: 'all' }, ...columnCompletions(table)];
      if (!last || last.type === 'comma') {
        return [...columnCompletions(table), ...aggCompletions(), { label: '*', detail: 'all' }];
      }
      if (isKw(last, 'AS')) return []; // free-text alias
      // After a complete projection item.
      return kw('AS', 'FROM');
    }

    case 'from': {
      if (!last) return tableCompletions();
      // table (and optional alias) present → next clauses.
      return kw('WHERE', 'GROUP BY', 'ORDER BY', 'LIMIT');
    }

    case 'where': {
      // Predicate start: after WHERE / AND / OR / NOT / '('.
      if (
        !last ||
        last.type === 'lparen' ||
        isKw(last, 'AND') ||
        isKw(last, 'OR') ||
        isKw(last, 'NOT')
      ) {
        return [...columnCompletions(table), ...kw('NOT')];
      }
      // IS [NOT] NULL.
      if (isKw(last, 'IS')) return kw('NULL', 'NOT NULL');
      if (isKw(last, 'NOT') && isKw(prev, 'IS')) return kw('NULL');
      // IN / NOT IN → value list.
      if (isKw(last, 'IN')) return [{ label: '(', detail: 'value list' }];
      // After a comparison operator → value help.
      if (last.type === 'op' && COMPARE.has(last.value)) return valueHelp(table, prev);
      // LIKE/ILIKE expect a quoted pattern → free text.
      if (isKw(last, 'LIKE') || isKw(last, 'ILIKE')) return [];
      // After a column (an ident NOT preceded by a comparison op) → operators.
      const afterColumn =
        (last.type === 'ident' || last.type === 'rbracket') &&
        !(prev?.type === 'op' && COMPARE.has(prev.value));
      if (afterColumn) return kw('=', '!=', '<', '>', '<=', '>=', 'LIKE', 'ILIKE', 'IN', 'IS');
      // After a complete predicate (value/closed list/RHS column).
      return kw('AND', 'OR', 'GROUP BY', 'ORDER BY', 'LIMIT');
    }

    case 'group': {
      const scope = selectScope(tokens);
      if (!last || last.type === 'comma') {
        const cols = scope.star ? columnCompletions(table) : scope.group.map((g) => ({ label: g }));
        return cols;
      }
      return kw('ORDER BY', 'LIMIT');
    }

    case 'order': {
      const scope = selectScope(tokens);
      if (!last || last.type === 'comma') {
        return scope.star
          ? columnCompletions(table)
          : scope.order.map((o) => ({ label: o, detail: 'output' }));
      }
      if (isKw(last, 'ASC') || isKw(last, 'DESC')) return kw('LIMIT');
      // After an order key → direction first, then LIMIT.
      return kw('ASC', 'DESC', 'LIMIT');
    }

    case 'limit':
      return []; // an integer — nothing to complete
  }
}

/** Value-position help: booleans get TRUE/FALSE, timestamp-ish columns get now().
 *  Other scalars are free literals → no list. `prev` is the operator's LHS col. */
function valueHelp(table: string | null, lhs: Token | undefined): Completion[] {
  if (!table || !lhs || (lhs.type !== 'ident' && lhs.type !== 'rbracket')) return [];
  const info = describeTable(table);
  const col = info?.columns.find((c) => c.name === (lhs.raw ?? lhs.value));
  if (col?.type === 'boolean') return [{ label: 'true' }, { label: 'false' }];
  if (col && /At$|Time$/.test(col.name)) return [{ label: 'now()', detail: 'current time' }];
  return [];
}
