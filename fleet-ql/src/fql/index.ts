// FQL — Fleet Query Language. Public API.
//
// A SQL-like query language over a ConfigHub fleet. Queries are parsed to an
// AST, planned into ConfigHub `where`/`where_data`/`whereResource` pushdown
// fetches (OR is split into multiple calls), then the full predicate is
// re-evaluated client-side for exact results. See README.md for the grammar
// and the pushdown model.
//
//   import { runQuery } from './fql';
//   const res = await runQuery("SELECT unit, image FROM resources WHERE severity = 'CRITICAL'", transport);

import { execute, type RunResult } from './executor';
import { parse, parseStatement, type ParseOptions } from './parser';
import { plan, type ExecutionPlan, type UnionPlan } from './planner';
import type { Transport } from './transport';

export { FqlError, renderError } from './errors';
export type { ParseOptions } from './parser';
export type { SelectStmt, Statement, UnionStmt } from './ast';
export type { ExecutionPlan, FetchSpec, UnionPlan } from './planner';
export type {
  Transport,
  ListParams,
  ResourceParams,
  RevisionParams,
  GrantsParams,
  AccessQuerySpec,
} from './transport';
export type { Row, ResultSet } from './evaluate';
export type { RunResult, RunStats } from './executor';
export { TABLES, columnNames, describeTable, describeTables, tableNames } from './schema';
export type { ColumnInfo, TableInfo } from './schema';
export { completionsAt, currentWord } from './complete';
export type { Completion } from './complete';
export { explainPlan, formatExpr } from './explain';
export type { PlanExplain, ExplainStage } from './explain';

/** Parse a query into an AST (throws FqlError on syntax errors). `parse` reads a
 *  single SELECT; `parseStatement` also accepts a UNION of SELECTs. */
export { parse, parseStatement };

/** Parse + plan a single SELECT without running it (throws on a UNION — use
 *  planStatement for that). */
export function planQuery(query: string, opts?: ParseOptions): ExecutionPlan {
  return plan(parse(query, opts));
}

/** Parse + plan any statement (single SELECT or a UNION) without running it.
 *  This is the "show plan" entry the console uses. Throws on parse/plan errors. */
export function planStatement(query: string, opts?: ParseOptions): ExecutionPlan | UnionPlan {
  return plan(parseStatement(query, opts));
}

/** Parse, plan, and execute a query (single SELECT or UNION) against a Transport. */
export async function runQuery(
  query: string,
  transport: Transport,
  opts?: ParseOptions,
): Promise<RunResult> {
  return execute(plan(parseStatement(query, opts)), transport);
}
