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
import { parse, type ParseOptions } from './parser';
import { plan, type ExecutionPlan } from './planner';
import type { Transport } from './transport';

export { FqlError, renderError } from './errors';
export type { ParseOptions } from './parser';
export type { SelectStmt } from './ast';
export type { ExecutionPlan, FetchSpec } from './planner';
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

/** Parse a query into an AST (throws FqlError on syntax errors). */
export { parse };

/** Parse + plan a query into an ExecutionPlan without running it. Useful for
 *  the "show plan" view in the console. Throws FqlError on parse/plan errors. */
export function planQuery(query: string, opts?: ParseOptions): ExecutionPlan {
  return plan(parse(query, opts));
}

/** Parse, plan, and execute a query against a Transport. */
export async function runQuery(
  query: string,
  transport: Transport,
  opts?: ParseOptions,
): Promise<RunResult> {
  return execute(planQuery(query, opts), transport);
}
