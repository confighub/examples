// FQL abstract syntax tree. A query is a single SELECT statement over one
// virtual table. Every node carries a source `pos` so the planner can report
// errors (e.g. "unknown column") pointing at the right place.

import type { Pos } from './errors';

// ─── Operators ──────────────────────────────────────────────────────────────

/** Logical connectives (expression tree shape). */
export type LogicalOp = 'AND' | 'OR';

/**
 * Comparison/match operators. These mirror the subset of ConfigHub's grammar
 * FQL exposes; string-pattern/regex ops are validated against column type in
 * the planner. `=` `!=` `<` `>` `<=` `>=` work on all scalar types; the
 * pattern/regex operators (LIKE, ILIKE, and the tilde family) are string-only;
 * IN/NOT IN take a list.
 */
export type CompareOp =
  | '='
  | '!='
  | '<'
  | '>'
  | '<='
  | '>='
  | 'LIKE'
  | 'NOT LIKE'
  | 'ILIKE'
  | '~'
  | '~*'
  | '!~'
  | '!~*'
  | 'IN'
  | 'NOT IN';

/** Aggregate functions available in projections with GROUP BY. */
export type AggFn = 'COUNT' | 'MAX' | 'MIN' | 'SUM' | 'AVG';

// ─── Expressions ──────────────────────────────────────────────────────────────

export type Expr =
  | LogicalExpr
  | NotExpr
  | CompareExpr
  | IsNullExpr
  | ColumnExpr
  | LiteralExpr
  | ListExpr
  | AggExpr
  | StarExpr;

/** `a AND b`, `a OR b`. */
export interface LogicalExpr {
  kind: 'logical';
  op: LogicalOp;
  left: Expr;
  right: Expr;
  pos: Pos;
}

/** `NOT a`. */
export interface NotExpr {
  kind: 'not';
  expr: Expr;
  pos: Pos;
}

/** `col <op> value`, e.g. `severity = 'CRITICAL'`, `image ~ ':latest'`. */
export interface CompareExpr {
  kind: 'compare';
  op: CompareOp;
  left: ColumnExpr;
  right: LiteralExpr | ListExpr;
  pos: Pos;
}

/** `col IS NULL` / `col IS NOT NULL`. */
export interface IsNullExpr {
  kind: 'isnull';
  negated: boolean; // IS NOT NULL
  column: ColumnExpr;
  pos: Pos;
}

/** A column reference, possibly dotted (`labels.env`, `metadata.name`) or a
 *  backtick-quoted verbatim data path (`` `spec.containers.*.image` ``). */
export interface ColumnExpr {
  kind: 'column';
  /** Full dotted path as written, e.g. "labels.env". */
  name: string;
  /** Path split on dots: ["labels","env"]. */
  path: string[];
  /** True when written backtick-quoted — treat as a raw data path. */
  quoted?: boolean;
  pos: Pos;
}

/** A scalar literal: string, number, or boolean. */
export interface LiteralExpr {
  kind: 'literal';
  value: string | number | boolean;
  /** The literal's source type, so the planner can type-check. */
  type: 'string' | 'number' | 'boolean';
  pos: Pos;
}

/** A parenthesized value list for IN / NOT IN. */
export interface ListExpr {
  kind: 'list';
  items: LiteralExpr[];
  pos: Pos;
}

/** `COUNT(*)`, `MAX(cveCount)`, etc. Only valid in projections. */
export interface AggExpr {
  kind: 'agg';
  fn: AggFn;
  /** null for COUNT(*); otherwise the aggregated column. */
  arg: ColumnExpr | StarExpr | null;
  pos: Pos;
}

/** `*` — only valid as COUNT(*) arg or a bare SELECT *. */
export interface StarExpr {
  kind: 'star';
  pos: Pos;
}

// ─── Statement ──────────────────────────────────────────────────────────────

/** One output column: an expression with an optional alias. */
export interface Projection {
  expr: Expr;
  /** Explicit `AS alias`, or null (planner derives a name). */
  alias: string | null;
  pos: Pos;
}

export type SortDir = 'ASC' | 'DESC';

export interface OrderKey {
  expr: Expr;
  dir: SortDir;
  pos: Pos;
}

/** The whole query: a single SELECT over one virtual table. */
export interface SelectStmt {
  kind: 'select';
  /** SELECT * is represented as a single StarExpr projection. */
  projections: Projection[];
  /** Virtual table name from FROM, with an optional alias (`FROM resources r`
   *  or `FROM resources AS r`). The alias qualifies columns as `r.col`. */
  from: { name: string; alias: string | null; pos: Pos };
  where: Expr | null;
  groupBy: ColumnExpr[];
  orderBy: OrderKey[];
  limit: number | null;
  pos: Pos;
}
