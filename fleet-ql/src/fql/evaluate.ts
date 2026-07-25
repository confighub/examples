// Client-side evaluation: the exact, complete pass that produces the final
// result from the rows the executor fetched. Applies WHERE (residual), then
// GROUP BY + aggregates (or plain projection), then ORDER BY and LIMIT.
//
// A Row is a flat map of FQL column name → value (the executor flattens API
// results into this shape, including dotted keys like "labels.env").

import type {
  AggExpr,
  ColumnExpr,
  CompareExpr,
  CompareOp,
  Expr,
  IsNullExpr,
  LiteralExpr,
  Projection,
  SelectStmt,
} from './ast';
import { FqlError } from './errors';

export type Row = Record<string, unknown>;
export interface ResultSet {
  columns: string[];
  rows: Row[];
}

/** Per-evaluation context. Single-table mode strips the FROM alias from column
 *  paths; join mode keeps columns alias-qualified (`d.kind`) — combined rows are
 *  keyed that way and each side's doc lives at `<alias>.__doc`. */
interface EvalCtx {
  alias: string | null;
  joinAliases?: ReadonlySet<string>;
}

/** The reserved row key holding the raw resource doc for data-path traversal. */
const DOC_KEY = '__doc';

/** Strip a leading table-alias segment from a column path (r.kind → kind).
 *  Quoted columns are verbatim and never stripped. In join mode the alias is part
 *  of the qualified key, so nothing is stripped. */
function effectivePath(col: ColumnExpr, ctx: EvalCtx): string[] {
  if (ctx.joinAliases) return col.path;
  if (!col.quoted && ctx.alias && col.path.length >= 2 && col.path[0] === ctx.alias) {
    return col.path.slice(1);
  }
  return col.path;
}

/**
 * Collect the candidate value(s) for a column over a row. A simple/curated
 * column yields a single value (its flat field). A raw YAML data path traverses
 * the resource doc (row.__doc), expanding `*` (every array element) and numeric
 * indices — so it can yield multiple values, which the comparison treats
 * existentially (true if ANY matches, mirroring ConfigHub's `.*.` semantics).
 */
function collectValues(row: Row, col: ColumnExpr, ctx: EvalCtx): unknown[] {
  const path = effectivePath(col, ctx);
  const flatKey = path.join('.');

  // Flat field (curated column / map key already flattened onto the row).
  if (!col.quoted && flatKey in row) return [row[flatKey]];

  // Pick the doc to traverse. In join mode a qualified raw path (`d.spec.x`)
  // traverses that side's doc at `d.__doc`; otherwise the row's own `__doc`.
  let doc: unknown;
  let rest = path;
  if (ctx.joinAliases && path.length >= 1 && ctx.joinAliases.has(path[0])) {
    doc = row[`${path[0]}.${DOC_KEY}`];
    rest = path.slice(1);
  } else {
    doc = row[DOC_KEY];
  }
  if (doc === undefined) {
    // No doc to traverse; fall back to shallow nested lookup on the row itself.
    return [shallowGet(row, path)];
  }
  return traverse(doc, rest);
}

/** Walk `node` along `path`, expanding `*` to all array elements and numeric
 *  segments to that index. Returns every leaf value reached. */
function traverse(node: unknown, path: string[]): unknown[] {
  if (path.length === 0) return [node];
  if (node == null) return [];
  const [seg, ...rest] = path;

  if (seg === '*') {
    if (!Array.isArray(node)) return [];
    return node.flatMap((el) => traverse(el, rest));
  }
  if (Array.isArray(node)) {
    const idx = Number(seg);
    if (Number.isInteger(idx) && idx >= 0 && idx < node.length) return traverse(node[idx], rest);
    return [];
  }
  if (typeof node === 'object') {
    return traverse((node as Record<string, unknown>)[seg], rest);
  }
  return [];
}

function shallowGet(row: Row, path: string[]): unknown {
  let cur: unknown = row;
  for (const seg of path) {
    if (cur == null || typeof cur !== 'object') return undefined;
    cur = (cur as Record<string, unknown>)[seg];
  }
  return cur;
}

/** Single representative value for projection/order/group (first candidate). */
function getValue(row: Row, col: ColumnExpr, ctx: EvalCtx): unknown {
  const vs = collectValues(row, col, ctx);
  return vs.length > 0 ? vs[0] : undefined;
}

// ─── Predicate evaluation (residual WHERE) ────────────────────────────────────

/** SQL LIKE → RegExp. `%`=>.*  `_`=>.  others escaped. */
function likeToRegExp(pattern: string, flags: string): RegExp {
  let out = '';
  for (const ch of pattern) {
    if (ch === '%') out += '.*';
    else if (ch === '_') out += '.';
    else out += ch.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }
  return new RegExp(`^${out}$`, flags);
}

function asStr(v: unknown): string {
  return v == null ? '' : String(v);
}
function asNum(v: unknown): number {
  return typeof v === 'number' ? v : Number(v);
}

function compare(op: CompareOp, left: unknown, right: LiteralExpr): boolean {
  // IN / NOT IN handled by caller (right is a list there).
  const rv = right.value;
  switch (op) {
    case '=':
      return right.type === 'number' ? asNum(left) === rv : asStr(left) === asStr(rv);
    case '!=':
      return right.type === 'number' ? asNum(left) !== rv : asStr(left) !== asStr(rv);
    case '<':
      return right.type === 'number' ? asNum(left) < (rv as number) : asStr(left) < asStr(rv);
    case '>':
      return right.type === 'number' ? asNum(left) > (rv as number) : asStr(left) > asStr(rv);
    case '<=':
      return right.type === 'number' ? asNum(left) <= (rv as number) : asStr(left) <= asStr(rv);
    case '>=':
      return right.type === 'number' ? asNum(left) >= (rv as number) : asStr(left) >= asStr(rv);
    case 'LIKE':
      return likeToRegExp(asStr(rv), '').test(asStr(left));
    case 'NOT LIKE':
      return !likeToRegExp(asStr(rv), '').test(asStr(left));
    case 'ILIKE':
      return likeToRegExp(asStr(rv), 'i').test(asStr(left));
    case '~':
      return new RegExp(asStr(rv)).test(asStr(left));
    case '~*':
      return new RegExp(asStr(rv), 'i').test(asStr(left));
    case '!~':
      return !new RegExp(asStr(rv)).test(asStr(left));
    case '!~*':
      return !new RegExp(asStr(rv), 'i').test(asStr(left));
    default:
      return false;
  }
}

function evalCompare(e: CompareExpr, row: Row, ctx: EvalCtx): boolean {
  // Existential over candidate values: a path with `*` yields many; the
  // predicate holds if ANY candidate matches (NOT IN/`!=` likewise hold if any
  // candidate satisfies the negated test — consistent with ConfigHub).
  const candidates = collectValues(row, e.left, ctx);
  const values = candidates.length > 0 ? candidates : [undefined];

  if (e.op === 'IN' || e.op === 'NOT IN') {
    if (e.right.kind !== 'list') return false;
    const items = e.right.items;
    const anyIn = values.some((left) =>
      items.some((it) =>
        it.type === 'number' ? asNum(left) === it.value : asStr(left) === asStr(it.value),
      ),
    );
    return e.op === 'IN' ? anyIn : !anyIn;
  }
  // Column-to-column comparison (e.g. HeadRevisionNum > LiveRevisionNum):
  // resolve the RHS column's value and compare as a synthesized literal whose
  // type follows the RHS value (numeric vs string).
  if (e.right.kind === 'column') {
    const rv = getValue(row, e.right, ctx);
    const rlit: LiteralExpr =
      typeof rv === 'number'
        ? { kind: 'literal', value: rv, type: 'number', pos: e.right.pos }
        : { kind: 'literal', value: asStr(rv), type: 'string', pos: e.right.pos };
    return values.some((left) => compare(e.op, left, rlit));
  }

  if (e.right.kind !== 'literal') return false;
  const lit = e.right;
  return values.some((left) => compare(e.op, left, lit));
}

function evalIsNull(e: IsNullExpr, row: Row, ctx: EvalCtx): boolean {
  const candidates = collectValues(row, e.column, ctx);
  const present = candidates.some((v) => v !== null && v !== undefined && v !== '');
  const isNull = !present;
  return e.negated ? !isNull : isNull;
}

/** Evaluate a (residual) predicate over a row. */
export function evalPredicate(e: Expr, row: Row, ctx: EvalCtx = { alias: null }): boolean {
  switch (e.kind) {
    case 'logical':
      return e.op === 'AND'
        ? evalPredicate(e.left, row, ctx) && evalPredicate(e.right, row, ctx)
        : evalPredicate(e.left, row, ctx) || evalPredicate(e.right, row, ctx);
    case 'not':
      return !evalPredicate(e.expr, row, ctx);
    case 'compare':
      return evalCompare(e, row, ctx);
    case 'isnull':
      return evalIsNull(e, row, ctx);
    default:
      // A bare column/literal/agg as a predicate isn't meaningful; treat truthy.
      return false;
  }
}

// ─── Projection naming ─────────────────────────────────────────────────────

function aggName(a: AggExpr): string {
  const inner = a.arg == null || a.arg.kind === 'star' ? '*' : a.arg.name;
  return `${a.fn.toLowerCase()}(${inner})`;
}

function projName(p: Projection): string {
  if (p.alias) return p.alias;
  switch (p.expr.kind) {
    case 'column':
      return p.expr.name;
    case 'agg':
      return aggName(p.expr);
    case 'star':
      return '*';
    default:
      return '?';
  }
}

// ─── Aggregates ────────────────────────────────────────────────────────────

function applyAgg(a: AggExpr, group: Row[], ctx: EvalCtx): number | null {
  if (a.fn === 'COUNT') {
    if (a.arg == null || a.arg.kind === 'star') return group.length;
    // COUNT(col) → non-null count.
    return group.filter((r) => {
      const v = getValue(r, a.arg as ColumnExpr, ctx);
      return v !== null && v !== undefined && v !== '';
    }).length;
  }
  const col = a.arg as ColumnExpr;
  const nums = group
    .map((r) => asNum(getValue(r, col, ctx)))
    .filter((n) => Number.isFinite(n));
  if (nums.length === 0) return null;
  switch (a.fn) {
    case 'MAX':
      return Math.max(...nums);
    case 'MIN':
      return Math.min(...nums);
    case 'SUM':
      return nums.reduce((x, y) => x + y, 0);
    case 'AVG':
      return nums.reduce((x, y) => x + y, 0) / nums.length;
    default:
      return null;
  }
}

// ─── Row shaping ────────────────────────────────────────────────────────────

function projectRow(projs: Projection[], row: Row, allColumns: string[], ctx: EvalCtx): Row {
  // SELECT * → every fetched column.
  if (projs.length === 1 && projs[0].expr.kind === 'star') {
    const out: Row = {};
    for (const c of allColumns) out[c] = row[c];
    return out;
  }
  const out: Row = {};
  for (const p of projs) {
    if (p.expr.kind === 'column') out[projName(p)] = getValue(row, p.expr, ctx);
  }
  return out;
}

function groupKey(row: Row, cols: ColumnExpr[], ctx: EvalCtx): string {
  return cols.map((c) => asStr(getValue(row, c, ctx))).join(' ');
}

/** Build aggregated output rows for a GROUP BY (or whole-set aggregate). */
function aggregate(stmt: SelectStmt, rows: Row[], ctx: EvalCtx): Row[] {
  const groups = new Map<string, Row[]>();
  if (stmt.groupBy.length === 0) {
    groups.set('', rows); // single implicit group
  } else {
    for (const r of rows) {
      const k = groupKey(r, stmt.groupBy, ctx);
      (groups.get(k) ?? groups.set(k, []).get(k)!).push(r);
    }
  }

  const out: Row[] = [];
  for (const members of groups.values()) {
    const rep = members[0] ?? {};
    const o: Row = {};
    for (const p of stmt.projections) {
      const name = projName(p);
      if (p.expr.kind === 'agg') o[name] = applyAgg(p.expr, members, ctx);
      else if (p.expr.kind === 'column') o[name] = getValue(rep, p.expr, ctx);
    }
    out.push(o);
  }
  return out;
}

// ─── Ordering ────────────────────────────────────────────────────────────────

function compareForSort(a: unknown, b: unknown): number {
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  return asStr(a).localeCompare(asStr(b));
}

function applyOrder(stmt: SelectStmt, rows: Row[], ctx: EvalCtx): Row[] {
  if (stmt.orderBy.length === 0) return rows;
  const keyName = (e: Expr): string =>
    e.kind === 'column' ? e.name : e.kind === 'agg' ? aggName(e) : '?';
  return [...rows].sort((ra, rb) => {
    for (const o of stmt.orderBy) {
      const name = keyName(o.expr);
      // Prefer the projected/aggregated value if present, else read the column.
      const va = name in ra ? ra[name] : o.expr.kind === 'column' ? getValue(ra, o.expr, ctx) : undefined;
      const vb = name in rb ? rb[name] : o.expr.kind === 'column' ? getValue(rb, o.expr, ctx) : undefined;
      const c = compareForSort(va, vb);
      if (c !== 0) return o.dir === 'DESC' ? -c : c;
    }
    return 0;
  });
}

// ─── Public entry ──────────────────────────────────────────────────────────────

/**
 * Run the client-side phase over fetched rows: filter (residual), project /
 * aggregate, order, limit. `allColumns` is the set of columns present in the
 * fetched rows (used for SELECT *).
 */
export function evaluate(
  stmt: SelectStmt,
  rows: Row[],
  allColumns: string[],
  /** Predicate to re-check client-side. Defaults to the statement's WHERE, but
   *  the planner may pass a residual that omits selector-only atoms (e.g.
   *  `revision = …`, handled entirely by the fetch). `undefined` → use
   *  stmt.where; `null` → no client-side filter. */
  residual?: Expr | null,
  /** When set, evaluate in join mode: columns stay alias-qualified and each
   *  side's doc is at `<alias>.__doc` (rows are the combined join rows). */
  joinAliases?: ReadonlySet<string>,
): ResultSet {
  const ctx: EvalCtx = { alias: stmt.from.alias, joinAliases };
  const filter = residual === undefined ? stmt.where : residual;

  // 1. Filter (the complete WHERE, re-checked client-side).
  const filtered = filter ? rows.filter((r) => evalPredicate(filter, r, ctx)) : rows;

  // 2. Project / aggregate.
  const isAggregate =
    stmt.groupBy.length > 0 || stmt.projections.some((p) => p.expr.kind === 'agg');

  let shaped: Row[];
  let columns: string[];
  if (isAggregate) {
    shaped = aggregate(stmt, filtered, ctx);
    columns = stmt.projections.map(projName);
  } else {
    shaped = filtered.map((r) => projectRow(stmt.projections, r, allColumns, ctx));
    columns =
      stmt.projections.length === 1 && stmt.projections[0].expr.kind === 'star'
        ? allColumns
        : stmt.projections.map(projName);
  }

  // 3. Order.
  shaped = applyOrder(stmt, shaped, ctx);

  // 4. Limit.
  if (stmt.limit != null) {
    if (stmt.limit < 0) throw new FqlError('LIMIT must be non-negative', stmt.pos, 'run');
    shaped = shaped.slice(0, stmt.limit);
  }

  return { columns, rows: shaped };
}
