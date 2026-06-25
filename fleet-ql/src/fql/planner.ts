// The planner turns a parsed SelectStmt into an ExecutionPlan: it binds and
// type-checks every column, normalizes WHERE to DNF (OR of AND-groups), and
// for each group emits the ConfigHub clause strings it can push down. The full
// original WHERE is carried for client-side re-evaluation — pushdown only
// narrows the fetch, it never decides the final result (see executor/evaluate).

import type {
  AggExpr,
  ColumnExpr,
  CompareExpr,
  CompareOp,
  Expr,
  IsNullExpr,
  SelectStmt,
} from './ast';
import { compileCompare, joinAnd } from './compile';
import { FqlError } from './errors';
import type { AccessQuerySpec } from './transport';
import {
  columnNames,
  type PushdownTarget,
  resolveColumn,
  type ColumnType,
  type ResolvedColumn,
  TABLES,
  type TableDef,
  type TableSource,
} from './schema';

// Max DNF AND-groups (= API calls) before we bail to a single unconstrained
// fetch + full client-side filter. Guards against exponential blow-up from
// deeply nested ORs while staying correct.
const MAX_GROUPS = 32;

/** One server fetch: compiled clause params for the table's list/invoke call. */
export interface FetchSpec {
  where?: string;
  whereData?: string;
  whereResource?: string;
  /** revisions only: filter on which units to pull revisions from. */
  whereUnit?: string;
  /** resources only: read this revision's data instead of head. A RevisionNum,
   *  or the symbolic 'head' / 'live'. */
  revision?: string;
  /** grants only: the RBAC access question the materializer applies. */
  accessQuery?: AccessQuerySpec;
}

export interface ExecutionPlan {
  source: TableSource;
  /** One fetch per DNF AND-group; their row sets are unioned (deduped). */
  fetches: FetchSpec[];
  /** The full original WHERE, re-evaluated client-side over fetched rows. */
  residual: Expr | null;
  /** The validated statement (projections/group/order/limit) for evaluate. */
  stmt: SelectStmt;
  /** resources table needs get-resources bodies; others read entity rows. */
  needsResourceData: boolean;
}

// ─── Operator/type validation (mirrors ConfigHub validateOperator) ───────────

const STRING_ONLY: ReadonlySet<CompareOp> = new Set([
  'LIKE',
  'NOT LIKE',
  'ILIKE',
  '~',
  '~*',
  '!~',
  '!~*',
]);

// ─── Pushdown soundness policy ────────────────────────────────────────────────
//
// Invariant: a pushed-down clause must be a SUPERSET of the true predicate (the
// always-on client-side residual then narrows to the exact answer). A clause is
// only pushed when its server semantics provably match (or over-match) ours.
// Verified against ConfigHub source (libra internal/views + core/function/api):
//
//  - Regex (~ ~* !~ !~*): server uses Go RE2 / Postgres POSIX, NOT JS RegExp —
//    DIVERGENT. Never push; evaluate client-side so JS regex is authoritative.
//  - where_data NEGATION (!=, NOT IN, NOT LIKE): a missing path makes `!=`
//    return TRUE server-side, and `*` is existential — negation over array/
//    optional data paths can DROP rows our residual would keep. Never push.
//  - LIKE/ILIKE/~~/!~~ and =,<,>,<=,>=,IN on scalars: IDENTICAL. Safe to push.
//  - Column-to-column: supported by the entity `where` SQL generator (quotes the
//    RHS column); NOT by where_data (literal-only). Push only for `where`.

const REGEX_OPS: ReadonlySet<CompareOp> = new Set(['~', '~*', '!~', '!~*']);
const NEGATING_OPS: ReadonlySet<CompareOp> = new Set(['!=', 'NOT IN', 'NOT LIKE', '!~', '!~*']);

/** Is pushing this comparison down to ConfigHub provably sound (server ⊇ client)?
 *  `col` is the resolved LHS; `rhsIsColumn` flags a column-to-column RHS. */
function isSoundToPush(op: CompareOp, col: ResolvedColumn, rhsIsColumn: boolean): boolean {
  if (!col.pushdown) return false;

  // Regex dialects diverge — JS regex must be authoritative (client-side only).
  if (REGEX_OPS.has(op)) return false;

  const target = col.pushdown.target;

  // Column-to-column: only the entity `where` generator handles a column RHS;
  // where_data / whereResource are literal-only.
  if (rhsIsColumn) return target === 'where';

  // Negation over a resource data path (where_data) is unsafe: missing-field and
  // array-existential semantics differ. Entity `where` fields are flat scalars,
  // so negation there is sound.
  if (target === 'where_data' && NEGATING_OPS.has(op)) return false;

  return true;
}

function checkOperator(op: CompareOp, type: ColumnType, where: CompareExpr): void {
  if (STRING_ONLY.has(op) && type !== 'string') {
    throw new FqlError(
      `operator ${op} is only valid on string columns, not ${type}`,
      where.pos,
      'plan',
    );
  }
  if (type === 'boolean' && op !== '=' && op !== '!=') {
    throw new FqlError(`booleans support only = and !=, not ${op}`, where.pos, 'plan');
  }
}

// ─── Column binding ──────────────────────────────────────────────────────────

function bind(table: TableDef, col: ColumnExpr, alias: string | null): ResolvedColumn {
  // Strip a leading table-alias segment ("r.kind" → "kind") when it matches the
  // FROM alias. Quoted columns are verbatim paths and are never alias-stripped.
  let path = col.path;
  if (!col.quoted && alias && path.length >= 2 && path[0] === alias) {
    path = path.slice(1);
  }
  const r = resolveColumn(table, path, col.quoted === true);
  if (!r) {
    throw new FqlError(
      `unknown column "${col.name}" on ${table.source}; known: ${columnNames(table).join(', ')}` +
        (table.rawDataPaths ? ' (or any resource data path, optionally `backtick-quoted`)' : ''),
      col.pos,
      'plan',
    );
  }
  return r;
}

// ─── NNF: push NOT to the leaves, flipping operators where possible ──────────

const FLIP: Partial<Record<CompareOp, CompareOp>> = {
  '=': '!=',
  '!=': '=',
  '<': '>=',
  '>=': '<',
  '>': '<=',
  '<=': '>',
  LIKE: 'NOT LIKE',
  'NOT LIKE': 'LIKE',
  '~': '!~',
  '!~': '~',
  '~*': '!~*',
  '!~*': '~*',
  IN: 'NOT IN',
  'NOT IN': 'IN',
};

/** Negation-normal form: returns an equivalent tree with NOT only (implicitly)
 *  at leaves. `neg` tracks whether the subtree is under an odd # of NOTs. */
function nnf(e: Expr, neg: boolean): Expr {
  switch (e.kind) {
    case 'logical': {
      const op = neg ? (e.op === 'AND' ? 'OR' : 'AND') : e.op;
      return { kind: 'logical', op, left: nnf(e.left, neg), right: nnf(e.right, neg), pos: e.pos };
    }
    case 'not':
      return nnf(e.expr, !neg);
    case 'compare': {
      if (!neg) return e;
      const flipped = FLIP[e.op];
      if (flipped) return { ...e, op: flipped };
      // No flippable form (e.g. NOT ILIKE) → keep as a residual-only NOT atom.
      return { kind: 'not', expr: e, pos: e.pos };
    }
    case 'isnull':
      return neg ? { ...e, negated: !e.negated } : e;
    default:
      return neg ? { kind: 'not', expr: e, pos: e.pos } : e;
  }
}

/** Distribute to DNF (an OR-list of AND-groups), aborting to `null` as soon as
 *  the group count would exceed `max`. The cross-product is never materialized
 *  past the cap, so a pathologically nested WHERE can't blow up time or memory —
 *  the old approach built the whole DNF first and only then checked its length. */
function boundedDNF(e: Expr, max: number): Expr[][] | null {
  if (e.kind === 'logical' && e.op === 'OR') {
    const l = boundedDNF(e.left, max);
    if (!l) return null;
    const r = boundedDNF(e.right, max);
    if (!r) return null;
    if (l.length + r.length > max) return null;
    return [...l, ...r];
  }
  if (e.kind === 'logical' && e.op === 'AND') {
    const l = boundedDNF(e.left, max);
    if (!l) return null;
    const r = boundedDNF(e.right, max);
    if (!r) return null;
    if (l.length * r.length > max) return null;
    const out: Expr[][] = [];
    for (const a of l) for (const b of r) out.push([...a, ...b]);
    return out;
  }
  return [[e]]; // a single atom is one group of one
}

/** Atoms that hold for EVERY result row: those reachable from the root through
 *  AND only (an OR subtree guarantees nothing). Pushing just these is the sound
 *  fallback when DNF is too large — one fetch that still narrows by the common
 *  conditions, instead of scanning the whole table. */
function topConjuncts(e: Expr): Expr[] {
  if (e.kind === 'logical' && e.op === 'AND') {
    return [...topConjuncts(e.left), ...topConjuncts(e.right)];
  }
  if (e.kind === 'logical' && e.op === 'OR') return [];
  return [e];
}

// ─── Pushdown partition ───────────────────────────────────────────────────────

/** Compile the pushable atoms of one AND-group into a FetchSpec. Unpushable
 *  atoms are simply omitted (client-side residual covers them). */
function groupToFetch(table: TableDef, atoms: Expr[], alias: string | null): FetchSpec {
  // One bucket per clause-style pushdown target. `revision` is not a clause —
  // it's a selector captured separately below.
  const parts: Record<'where' | 'where_data' | 'whereResource' | 'whereUnit', string[]> = {
    where: [],
    where_data: [],
    whereResource: [],
    whereUnit: [],
  };
  let revision: string | undefined;
  let accessQuery: AccessQuerySpec | undefined;

  for (const atom of atoms) {
    if (atom.kind === 'compare') {
      const col = bind(table, atom.left, alias);
      checkOperator(atom.op, col.type, atom);
      const rhsIsColumn = atom.right.kind === 'column';
      // Always bind (validate) an RHS column even if we won't push it.
      const rhsCol = rhsIsColumn ? bind(table, atom.right as ColumnExpr, alias) : null;

      // The `revision` selector: only `revision = <N | 'head' | 'live'>`. It
      // picks which revision's data to read, so it's a fetch parameter, not a
      // filter clause (the transport stamps `revision` onto every row, so the
      // residual still matches).
      if (col.pushdown?.target === 'revision') {
        if (atom.op === '=' && atom.right.kind === 'literal') {
          revision = revisionSelector(atom);
        }
        // Any other operator/shape falls through to the client-side residual.
        continue;
      }

      // The `accessQuery` selectors (grants: verb/resource/apiGroup/namespace/
      // name): captured into the materializer's RBAC question, not a clause.
      // Stripped from the residual (the row has no such literal field).
      if (col.pushdown?.target === 'accessQuery') {
        if (atom.op === '=' && atom.right.kind === 'literal') {
          accessQuery ??= {};
          accessQuery[col.pushdown.expr as keyof AccessQuerySpec] = String(atom.right.value);
        }
        continue;
      }

      if (!isSoundToPush(atom.op, col, rhsIsColumn)) continue; // client-side residual

      if (rhsIsColumn) {
        // Column-to-column on an entity `where` field: emit `<lhs> op <rhs>`
        // with both server-side column expressions (no literal).
        if (rhsCol!.pushdown?.target === 'where') {
          parts.where.push(`${col.pushdown!.expr} ${atom.op} ${rhsCol!.pushdown.expr}`);
        }
        continue;
      }
      parts[col.pushdown!.target as 'where' | 'where_data' | 'whereResource' | 'whereUnit'].push(
        compileCompare(atom, col),
      );
    } else if (atom.kind === 'isnull') {
      const col = bind(table, atom.column, alias);
      // Push NULL checks onto entity (`where`/`whereUnit`) fields only; data-path
      // NULL semantics differ (`.|`), so leave those to client-side.
      if (col.pushdown && (col.pushdown.target === 'where' || col.pushdown.target === 'whereUnit')) {
        parts[col.pushdown.target].push(
          `${col.pushdown.expr} IS ${atom.negated ? 'NOT NULL' : 'NULL'}`,
        );
      }
    }
    // 'not'/'logical' atoms (shouldn't appear post-DNF except residual NOTs):
    // skip — handled client-side.
  }

  const spec: FetchSpec = {};
  if (parts.where.length) spec.where = joinAnd(parts.where);
  if (parts.where_data.length) spec.whereData = joinAnd(parts.where_data);
  if (parts.whereResource.length) spec.whereResource = joinAnd(parts.whereResource);
  if (parts.whereUnit.length) spec.whereUnit = joinAnd(parts.whereUnit);
  if (revision !== undefined) spec.revision = revision;
  if (accessQuery !== undefined) spec.accessQuery = accessQuery;
  return spec;
}

/** Normalize a `revision = …` RHS literal to a selector string: a RevisionNum,
 *  or the symbolic 'head' / 'live'. */
function revisionSelector(atom: CompareExpr): string | undefined {
  const lit = atom.right;
  if (lit.kind !== 'literal') return undefined;
  if (lit.type === 'number') return String(lit.value);
  const v = String(lit.value).toLowerCase();
  if (v === 'head' || v === 'live') return v;
  // A numeric string ('5') is fine too; anything else is left to the residual.
  return /^\d+$/.test(v) ? v : undefined;
}

// ─── Projection / order / group validation ────────────────────────────────────

function validateAgg(table: TableDef, agg: AggExpr, alias: string | null): void {
  if (agg.arg && agg.arg.kind === 'column') {
    const col = bind(table, agg.arg, alias);
    if (agg.fn !== 'COUNT' && col.type !== 'number') {
      throw new FqlError(`${agg.fn}() requires a numeric column, got ${col.type}`, agg.pos, 'plan');
    }
  }
}

function validateProjections(table: TableDef, stmt: SelectStmt): void {
  const alias = stmt.from.alias;
  const hasAgg = stmt.projections.some((p) => p.expr.kind === 'agg');
  const grouped = hasAgg || stmt.groupBy.length > 0;
  const groupCols = new Set(stmt.groupBy.map((c) => c.name));

  for (const p of stmt.projections) {
    if (p.expr.kind === 'star') continue;
    if (p.expr.kind === 'agg') {
      validateAgg(table, p.expr, alias);
      continue;
    }
    if (p.expr.kind === 'column') {
      bind(table, p.expr, alias);
      if (grouped && !groupCols.has(p.expr.name)) {
        throw new FqlError(
          `column "${p.expr.name}" must appear in GROUP BY or an aggregate`,
          p.expr.pos,
          'plan',
        );
      }
    }
  }
  for (const c of stmt.groupBy) bind(table, c, alias);

  // ORDER BY may reference a SELECT output alias (standard SQL) rather than a
  // base column — collect those so we don't reject them as unknown columns.
  const outAliases = new Set(stmt.projections.map((p) => p.alias).filter((a): a is string => !!a));
  for (const o of stmt.orderBy) {
    if (o.expr.kind === 'agg') {
      validateAgg(table, o.expr, alias);
    } else if (o.expr.kind === 'column' && !outAliases.has(o.expr.name)) {
      bind(table, o.expr, alias);
    }
  }
}

/** Bind & type-check every WHERE atom, independent of how (or whether) it gets
 *  pushed down — so an unknown column or bad operator is rejected the same way
 *  regardless of the fetch strategy (the DNF and the over-cap fallback paths push
 *  different subsets). */
function validateWhere(table: TableDef, e: Expr, alias: string | null): void {
  switch (e.kind) {
    case 'logical':
      validateWhere(table, e.left, alias);
      validateWhere(table, e.right, alias);
      return;
    case 'not':
      validateWhere(table, e.expr, alias);
      return;
    case 'compare': {
      const col = bind(table, e.left, alias);
      checkOperator(e.op, col.type, e);
      if (e.right.kind === 'column') bind(table, e.right, alias);
      return;
    }
    case 'isnull':
      bind(table, e.column, alias);
      return;
    default:
      return;
  }
}

/** De-dupe identical fetch specs (e.g. `a OR a`). */
function dedupeFetches(specs: FetchSpec[]): FetchSpec[] {
  const seen = new Set<string>();
  return specs.filter((f) => {
    const key = JSON.stringify(f);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

// ─── Public entry ──────────────────────────────────────────────────────────────

/** Plan a parsed statement against the virtual-table catalog. */
export function plan(stmt: SelectStmt): ExecutionPlan {
  const table = TABLES[stmt.from.name];
  if (!table) {
    throw new FqlError(
      `unknown table "${stmt.from.name}"; available: ${Object.keys(TABLES).join(', ')}`,
      stmt.from.pos,
      'plan',
    );
  }

  validateProjections(table, stmt);
  const alias = stmt.from.alias;

  let fetches: FetchSpec[];
  if (!stmt.where) {
    fetches = [{}]; // unconstrained single fetch
  } else {
    validateWhere(table, stmt.where, alias);
    const nf = nnf(stmt.where, false);
    const groups = boundedDNF(nf, MAX_GROUPS);
    if (groups === null) {
      // Too many OR-branches to enumerate: push only the guaranteed top-level
      // conjuncts (one fetch that still narrows) rather than scanning everything.
      fetches = [groupToFetch(table, topConjuncts(nf), alias)];
    } else {
      // One AND-only fetch per DNF group, unioned client-side — ConfigHub has no
      // server-side OR. We do NOT collapse an unpushable (empty) branch into a
      // single unconstrained fetch: that would be unsound when other branches
      // carry a `revision` / `accessQuery` selector (collapsing drops it and reads
      // the wrong data), and it discards the narrowing the other branches do get.
      // Identical specs still de-dupe (e.g. two unpushable branches → one {}).
      fetches = dedupeFetches(groups.map((g) => groupToFetch(table, g, alias)));
    }
  }

  return {
    source: table.source,
    fetches,
    // Selector atoms (`revision`, grants `accessQuery` fields) are fully handled
    // by the fetch, not by row fields, so strip them from the client-side
    // residual — otherwise `revision = 'head'` would drop every stamped numeric
    // row, and `verb = 'delete'` would drop every grant (no literal `verb`
    // field). Other predicates remain for exact re-checking.
    residual: stmt.where ? stripSelectors(stmt.where, table, stmt.from.alias) : null,
    stmt,
    needsResourceData: table.source === 'resources',
  };
}

/** Targets that are fetch selectors, not row-field clauses — dropped from the
 *  client-side residual (the fetch handles them; rows carry no matching field). */
const SELECTOR_TARGETS: ReadonlySet<PushdownTarget> = new Set(['revision', 'accessQuery']);

/** Rewrite a WHERE tree, dropping any selector comparison (`revision = …`,
 *  grants `verb`/`resource`/… ) so only re-checkable predicates remain. */
function stripSelectors(e: Expr, table: TableDef, alias: string | null): Expr | null {
  switch (e.kind) {
    case 'logical': {
      const l = stripSelectors(e.left, table, alias);
      const r = stripSelectors(e.right, table, alias);
      if (l === null) return r; // dropped → identity for AND, absorbing handled below
      if (r === null) return l;
      return { ...e, left: l, right: r };
    }
    case 'not': {
      const inner = stripSelectors(e.expr, table, alias);
      return inner === null ? null : { ...e, expr: inner };
    }
    case 'compare': {
      const col = resolveColumn(table, stripAlias(e.left, alias), e.left.quoted === true);
      return col?.pushdown && SELECTOR_TARGETS.has(col.pushdown.target) ? null : e;
    }
    default:
      return e;
  }
}

/** Alias-strip a column path (mirrors bind()), for residual rewriting. */
function stripAlias(col: ColumnExpr, alias: string | null): string[] {
  if (!col.quoted && alias && col.path.length >= 2 && col.path[0] === alias) {
    return col.path.slice(1);
  }
  return col.path;
}
