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

import { FqlError } from './errors';
import { lex, type Token } from './lexer';
import { describeTable, tableNames } from './schema';

export interface Completion {
  label: string;
  detail?: string;
  /** Text inserted (defaults to label). */
  insert?: string;
  /** Prefix-matched against the in-progress word instead of `label` — a map
   *  column matches on its bare prefix (`labels`), not the display `labels['key']`. */
  match?: string;
  /** Place the caret this many chars before the end of the inserted text, to drop
   *  inside the quotes of a `labels['']` scaffold. */
  caretBack?: number;
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

const strip = (toks: Token[]): Token[] => toks.filter((t) => t.type !== 'eof');

/** Lex defensively: blank out an offending char and retry (up to a few times), so
 *  a stray illegal char anywhere in the query doesn't kill all completion — the
 *  surrounding clause structure is preserved. */
function tolerantLex(src: string): Token[] {
  let s = src;
  for (let i = 0; i < 8; i++) {
    try {
      return strip(lex(s));
    } catch (e) {
      const at = e instanceof FqlError && e.pos ? e.pos.start : -1;
      if (at < 0 || at >= s.length) break;
      s = s.slice(0, at) + ' ' + s.slice(at + 1);
    }
  }
  return [];
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

/** Columns of the FROM table (+ raw-path completions for resources). */
function columnCompletions(table: string | null): Completion[] {
  const info = table ? describeTable(table) : null;
  if (!info) return [];
  const out: Completion[] = [];
  for (const c of info.columns) {
    if (c.kind === 'map') {
      // Map columns (labels['key'], gate['key'], …) — offer a scaffold and drop
      // the caret inside the quotes; match on the bare prefix so typing toward it
      // narrows (the display label's `['key']` placeholder never prefix-matches a
      // real key).
      const prefix = c.name.split('[')[0];
      out.push({
        label: c.name,
        detail: `${c.type} key`,
        match: prefix,
        insert: `${prefix}['']`,
        caretBack: 2,
      });
    } else {
      out.push({ label: c.name, detail: `${c.type}${c.pushdown ? '' : ' · client'}` });
    }
  }
  if (info.rawDataPaths) out.push(...rawPathCompletions());
  return out;
}

/** Curated raw-path completions for `resources`. Closed (no dangling backtick) so
 *  accepting one doesn't strand the editor inside an open literal. */
function rawPathCompletions(): Completion[] {
  return [
    {
      label: '`spec.template.spec.containers.*.image`',
      detail: 'container image',
      match: '`spec',
      insert: '`spec.template.spec.containers.*.image`',
    },
    { label: '`spec.replicas`', detail: 'raw path', match: '`spec', insert: '`spec.replicas`' },
    {
      label: "metadata.annotations['key']",
      detail: 'annotation',
      match: 'metadata',
      insert: "metadata.annotations['']",
      caretBack: 2,
    },
  ];
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
/** A token that ends a column reference (a bare/dotted ident or a `]` subscript). */
const isColumnEnd = (t: Token | undefined) => !!t && (t.type === 'ident' || t.type === 'rbracket');

/** True when the tail's caret position is inside an `IN ( … )` value list (the
 *  nearest unclosed `(` was opened right after an `IN`). Such a `(` wraps literals,
 *  not a predicate group, so we must not offer columns there. */
function inInList(tail: Token[]): boolean {
  const stack: boolean[] = [];
  for (let i = 0; i < tail.length; i++) {
    const t = tail[i];
    if (t.type === 'lparen') stack.push(isKw(tail[i - 1], 'IN'));
    else if (t.type === 'rparen') stack.pop();
  }
  return stack.length > 0 && stack[stack.length - 1];
}

// Single-shot clauses: never suggest one that already exists elsewhere in the query.
const SINGLE_CLAUSES = new Set(['WHERE', 'GROUP BY', 'ORDER BY', 'LIMIT']);
function presentClauses(text: string): Set<string> {
  const toks = tolerantLex(text);
  const s = new Set<string>();
  for (let i = 0; i < toks.length; i++) {
    const t = toks[i];
    if (t.type !== 'keyword') continue;
    if (t.value === 'WHERE') s.add('WHERE');
    else if (t.value === 'LIMIT') s.add('LIMIT');
    else if (t.value === 'GROUP' && toks[i + 1]?.value === 'BY') s.add('GROUP BY');
    else if (t.value === 'ORDER' && toks[i + 1]?.value === 'BY') s.add('ORDER BY');
  }
  return s;
}

// ─── Public entry ──────────────────────────────────────────────────────────────

/** Contextual completion candidates for the caret position (unfiltered by the
 *  in-progress word; the editor prefix-filters). */
export function completionsAt(text: string, caret: number): Completion[] {
  const { word, start } = currentWord(text, caret);
  // Suppress inside a literal opened BEFORE the word, or while typing a string
  // value. A backtick the word itself opens is a raw path → keep completing.
  if (inLiteral(text, start)) return [];
  if (word.startsWith("'")) return [];

  const tokens = tolerantLex(text.slice(0, start));
  const out = contextCandidates(tokens);
  // Drop clause keywords that already exist elsewhere in the query (no look-ahead
  // past the caret would otherwise let us suggest a duplicate clause).
  const present = presentClauses(text);
  return out.filter((c) => !(SINGLE_CLAUSES.has(c.label) && present.has(c.label)));
}

function contextCandidates(tokens: Token[]): Completion[] {
  const { clause, tail } = clauseOf(tokens);
  const table = tableOf(tokens);
  const last = tail[tail.length - 1];
  const prev = tail[tail.length - 2];

  switch (clause) {
    case 'start':
      return kw('SELECT');

    case 'select': {
      const openAgg = last?.type === 'lparen' && prev?.type === 'keyword' && AGG.has(prev.value);
      if (openAgg) return [{ label: '*', detail: 'all' }, ...columnCompletions(table)];
      if (!last || last.type === 'comma') {
        return [...columnCompletions(table), ...aggCompletions(), { label: '*', detail: 'all' }];
      }
      if (isKw(last, 'AS')) return []; // free-text alias
      return kw('AS', 'FROM');
    }

    case 'from':
      return last ? kw('WHERE', 'GROUP BY', 'ORDER BY', 'LIMIT') : tableCompletions();

    case 'where': {
      // Inside an IN (...) value list: literals only — never columns.
      if (inInList(tail)) return [];
      // IS [NOT] NULL (checked before the predicate-start NOT guard).
      if (isKw(last, 'IS')) return kw('NULL', 'NOT NULL');
      if (isKw(last, 'NOT') && isKw(prev, 'IS')) return kw('NULL');
      // NOT after a column → it can only introduce IN / LIKE.
      if (isKw(last, 'NOT') && isColumnEnd(prev)) return kw('IN', 'LIKE');
      // IN / NOT IN → open the value list.
      if (isKw(last, 'IN')) return [{ label: '(', detail: 'value list' }];
      // After a comparison operator → value help.
      if (last?.type === 'op' && COMPARE.has(last.value)) return valueHelp(table, prev);
      // LIKE / ILIKE expect a quoted pattern → free text.
      if (isKw(last, 'LIKE') || isKw(last, 'ILIKE')) return [];
      // Predicate start: after WHERE / AND / OR / a predicate-group '(' / a
      // *leading* NOT (post-column/IS NOT already handled above).
      if (!last || last.type === 'lparen' || isKw(last, 'AND') || isKw(last, 'OR') || isKw(last, 'NOT')) {
        return [...columnCompletions(table), ...kw('NOT')];
      }
      // After a column LHS (ident/`]` not preceded by a comparison op) → operators.
      if (isColumnEnd(last) && !(prev?.type === 'op' && COMPARE.has(prev.value))) {
        return kw('=', '!=', '<', '>', '<=', '>=', 'LIKE', 'ILIKE', 'IN', 'IS');
      }
      // After a complete predicate (value / closed list / RHS column).
      return kw('AND', 'OR', 'GROUP BY', 'ORDER BY', 'LIMIT');
    }

    case 'group': {
      const scope = selectScope(tokens);
      if (!last || last.type === 'comma') {
        return scope.star ? columnCompletions(table) : scope.group.map((g) => ({ label: g }));
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
