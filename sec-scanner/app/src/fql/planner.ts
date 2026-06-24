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
import {
  columnNames,
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

/** Distribute to DNF: an OR-list of AND-groups (each a flat atom list). */
function toDNF(e: Expr): Expr[][] {
  if (e.kind === 'logical') {
    if (e.op === 'OR') return [...toDNF(e.left), ...toDNF(e.right)];
    // AND → cross product of the two sides' groups.
    const out: Expr[][] = [];
    for (const a of toDNF(e.left)) {
      for (const b of toDNF(e.right)) out.push([...a, ...b]);
    }
    return out;
  }
  return [[e]]; // a single atom is one group of one
}

// ─── Pushdown partition ───────────────────────────────────────────────────────

/** Compile the pushable atoms of one AND-group into a FetchSpec. Unpushable
 *  atoms are simply omitted (client-side residual covers them). */
function groupToFetch(table: TableDef, atoms: Expr[], alias: string | null): FetchSpec {
  const whereParts: string[] = [];
  const dataParts: string[] = [];
  const resourceParts: string[] = [];

  for (const atom of atoms) {
    if (atom.kind === 'compare') {
      const col = bind(table, atom.left, alias);
      checkOperator(atom.op, col.type, atom);
      // Column-to-column comparisons (RHS is a column) can't be expressed in
      // ConfigHub's `path op literal` filter — evaluate them client-side. Also
      // validate the RHS column exists.
      if (atom.right.kind === 'column') {
        bind(table, atom.right, alias);
        continue;
      }
      if (!col.pushdown) continue; // client-side only
      const frag = compileCompare(atom, col);
      bucket(col.pushdown.target, frag, whereParts, dataParts, resourceParts);
    } else if (atom.kind === 'isnull') {
      const col = bind(table, atom.column, alias);
      // Only push NULL checks onto entity (`where`) fields; data-path NULL
      // semantics differ (`.|`), so leave those to client-side.
      if (col.pushdown?.target === 'where') {
        whereParts.push(`${col.pushdown.expr} IS ${atom.negated ? 'NOT NULL' : 'NULL'}`);
      }
    }
    // 'not'/'logical' atoms (shouldn't appear post-DNF except residual NOTs):
    // skip — handled client-side.
  }

  const spec: FetchSpec = {};
  if (whereParts.length) spec.where = joinAnd(whereParts);
  if (dataParts.length) spec.whereData = joinAnd(dataParts);
  if (resourceParts.length) spec.whereResource = joinAnd(resourceParts);
  return spec;
}

function bucket(
  target: 'where' | 'where_data' | 'whereResource',
  frag: string,
  where: string[],
  data: string[],
  resource: string[],
): void {
  if (target === 'where') where.push(frag);
  else if (target === 'where_data') data.push(frag);
  else resource.push(frag);
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

  let fetches: FetchSpec[];
  if (!stmt.where) {
    fetches = [{}]; // unconstrained single fetch
  } else {
    const groups = toDNF(nnf(stmt.where, false));
    if (groups.length > MAX_GROUPS) {
      fetches = [{}]; // too many disjuncts → fetch broad, filter client-side
    } else {
      fetches = groups.map((g) => groupToFetch(table, g, stmt.from.alias));
      // De-dup identical fetch specs (e.g. `a OR a`).
      const seen = new Set<string>();
      fetches = fetches.filter((f) => {
        const key = JSON.stringify(f);
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      });
    }
  }

  return {
    source: table.source,
    fetches,
    residual: stmt.where,
    stmt,
    needsResourceData: table.source === 'resources',
  };
}
