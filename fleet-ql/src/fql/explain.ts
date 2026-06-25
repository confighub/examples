// Turn an ExecutionPlan into a human-readable EXPLAIN: the pipeline stages (a
// small DAG) and the API calls each fetch compiles to. Portable (no UI), so the
// console renders it and tests can assert on it.

import type { Expr } from './ast';
import type { ExecutionPlan, FetchSpec, UnionPlan } from './planner';
import type { TableSource } from './schema';

/** One stage in the plan DAG: a title plus detail lines (calls, keys, columns). */
export interface ExplainStage {
  title: string;
  lines: string[];
}

export interface PlanExplain {
  /** The leaf scan(s): one for a single table, two for a join (run concurrently). */
  inputs: ExplainStage[];
  /** The client-side pipeline applied to the fetched rows, top to bottom. */
  pipeline: ExplainStage[];
}

// ─── Expr → readable FQL (for the residual / filter stage) ───────────────────

function literalText(v: string | number | boolean, type: string): string {
  if (type === 'string') return `'${String(v).replaceAll("'", "''")}'`;
  return String(v);
}

/** Render an expression back to readable FQL (used for the residual filter). */
export function formatExpr(e: Expr): string {
  switch (e.kind) {
    case 'logical':
      return `(${formatExpr(e.left)} ${e.op} ${formatExpr(e.right)})`;
    case 'not':
      return `NOT ${formatExpr(e.expr)}`;
    case 'isnull':
      return `${e.column.name} IS ${e.negated ? 'NOT NULL' : 'NULL'}`;
    case 'compare': {
      const r = e.right;
      let rhs: string;
      if (r.kind === 'literal') rhs = literalText(r.value, r.type);
      else if (r.kind === 'column') rhs = r.name;
      else rhs = `(${r.items.map((i) => literalText(i.value, i.type)).join(', ')})`;
      return `${e.left.name} ${e.op} ${rhs}`;
    }
    default:
      return '?';
  }
}

// ─── Plan → stages: the generated-SDK calls each fetch compiles to ───────────
//
// Calls go through the generated openapi-fetch client (`cub`, src/sdk/client.ts),
// typed against the OpenAPI `paths` map. The explainer renders the real client
// call — method, path, and the args derived from the pushed-down predicates — so
// "show plan" points straight at the SDK.

/** A query/path arg object as openapi-fetch receives it, e.g. `{ where: "…" }`. */
function args(obj: Record<string, string | undefined>): string {
  return Object.entries(obj)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => `${k}: ${JSON.stringify(v)}`)
    .join(', ');
}

/** `cub.GET('<path>', { params: { query: {…} } })` (params omitted when empty). */
function getCall(path: string, query: Record<string, string | undefined>): string {
  const q = args(query);
  return q
    ? `cub.GET('${path}', { params: { query: { ${q} } } })`
    : `cub.GET('${path}', {})  // no pushdown — fetch all, filter client-side`;
}

/** `cub.POST('/function/invoke', …)` running get-resources, with pushed query/body. */
function invokeCall(spec: Pick<FetchSpec, 'where' | 'whereData' | 'whereResource'>): string {
  const q = args({ where: spec.where, where_data: spec.whereData });
  const body = [
    spec.whereResource ? `WhereResource: ${JSON.stringify(spec.whereResource)}` : '',
    `FunctionInvocations: [get-resources]`,
  ]
    .filter(Boolean)
    .join(', ');
  const params = q ? `params: { query: { ${q} } }, ` : '';
  const note = q ? '' : '  // no pushdown — fetch all, filter client-side';
  return `cub.POST('/function/invoke', { ${params}body: { ${body} } })${note}`;
}

/** The generated-client call(s) a single fetch compiles to, for `source`. */
function sdkCalls(source: TableSource, spec: FetchSpec): string[] {
  switch (source) {
    case 'units':
      return [getCall('/unit', { where: spec.where })];
    case 'spaces':
      return [getCall('/space', { where: spec.where })];
    case 'revisions':
      return [
        `${getCall('/unit', { where: spec.whereUnit })}  // scope units`,
        `${getCall('/space/{space_id}/unit/{unit_id}/revision', { where: spec.where })}  // per unit`,
      ];
    case 'resources':
      // Time-travel: resolve each unit's RevisionID, then read its data blob.
      if (spec.revision !== undefined) {
        return [
          `${getCall('/unit', { where: spec.where })}  // in-scope units`,
          `cub.GET('/space/{space_id}/unit/{unit_id}/revision', …)  // resolve RevisionID`,
          `cub.GET('/space/{space_id}/unit/{unit_id}/revision/{revision_id}/data', { parseAs: 'text' })  // revision = ${spec.revision}`,
        ];
      }
      return [invokeCall(spec)];
    case 'grants':
    case 'roles':
    case 'bindings':
    case 'rbac_findings':
      return [
        invokeCall({ where: spec.where }),
        `→ rbac materialize (${source})`,
        ...(spec.accessQuery ? [`access selector: ${JSON.stringify(spec.accessQuery)}`] : []),
      ];
  }
}

function scanStage(source: TableSource, alias: string | null, fetches: FetchSpec[]): ExplainStage {
  const title = `scan ${source}${alias ? ` ${alias}` : ''}`;
  if (fetches.length === 0) return { title, lines: ['(no fetch)'] };
  const lines: string[] = [];
  fetches.forEach((spec, i) => {
    if (fetches.length > 1) lines.push(`# fetch ${i + 1}`);
    lines.push(...sdkCalls(source, spec));
  });
  return { title, lines };
}

function exprName(e: Expr): string {
  if (e.kind === 'column') return e.name;
  if (e.kind === 'agg') return `${e.fn.toLowerCase()}(${e.arg && e.arg.kind === 'column' ? e.arg.name : '*'})`;
  return '*';
}

function projectionNames(plan: ExecutionPlan): string[] {
  return plan.stmt.projections.map((p) => {
    if (p.alias) return p.alias;
    if (p.expr.kind === 'star') return '*';
    return exprName(p.expr);
  });
}

/** Each UNION branch as one stage: its full sub-plan (scans + pipeline) nested. */
function branchStage(i: number, branch: ExecutionPlan): ExplainStage {
  const be = explainPlan(branch);
  const lines: string[] = [];
  for (const s of be.inputs) {
    lines.push(s.title);
    for (const l of s.lines) lines.push(`  ${l}`);
  }
  for (const s of be.pipeline) {
    lines.push(`· ${s.title}`);
    for (const l of s.lines) lines.push(`  ${l}`);
  }
  return { title: `branch ${i + 1}`, lines };
}

function explainUnion(plan: UnionPlan): PlanExplain {
  const inputs = plan.branches.map((b, i) => branchStage(i, b));
  const allAll = plan.all.every((a) => a);
  const anyDistinct = plan.all.some((a) => !a);
  const pipeline: ExplainStage[] = [
    {
      title: `union${allAll ? ' all' : ''} · ${plan.branches.length} branches`,
      lines: [
        anyDistinct ? 'de-dupe on the full output row' : 'keep all rows (no de-dupe)',
        'columns aligned by position to branch 1',
      ],
    },
  ];
  if (plan.orderBy.length) {
    pipeline.push({
      title: 'order by',
      lines: [plan.orderBy.map((o) => `${exprName(o.expr)} ${o.dir}`).join(', ')],
    });
  }
  if (plan.limit != null) pipeline.push({ title: `limit ${plan.limit}`, lines: [] });
  return { inputs, pipeline };
}

/** Build the EXPLAIN view of a plan: inputs (scans) + the client-side pipeline. */
export function explainPlan(plan: ExecutionPlan | UnionPlan): PlanExplain {
  if ('kind' in plan) return explainUnion(plan);

  const inputs: ExplainStage[] = [];
  const pipeline: ExplainStage[] = [];
  const stmt = plan.stmt;

  if (plan.join) {
    const j = plan.join;
    inputs.push(scanStage(j.left.source, j.left.alias, j.left.fetches));
    inputs.push(scanStage(j.right.source, j.right.alias, j.right.fetches));
    pipeline.push({
      title: `hash join · ${j.type}`,
      lines: [
        ...j.on.map((o) => `on ${j.left.alias}.${o.left} = ${j.right.alias}.${o.right}`),
        j.type === 'left' ? 'unmatched left rows kept (right cols → null)' : 'unmatched rows dropped',
      ],
    });
    if (j.residual) {
      pipeline.push({ title: 'filter · full WHERE (client-side)', lines: [formatExpr(j.residual)] });
    }
  } else {
    inputs.push(scanStage(plan.source, stmt.from.alias, plan.fetches));
    if (plan.fetches.length > 1) {
      pipeline.push({
        title: 'union + de-dupe · OR branches',
        lines: [`${plan.fetches.length} fetches merged (ConfigHub has no server-side OR)`],
      });
    }
    if (plan.residual) {
      pipeline.push({ title: 'filter · full WHERE (client-side)', lines: [formatExpr(plan.residual)] });
    }
  }

  const hasAgg = stmt.groupBy.length > 0 || stmt.projections.some((p) => p.expr.kind === 'agg');
  if (hasAgg) {
    pipeline.push({
      title: stmt.groupBy.length ? 'group + aggregate' : 'aggregate',
      lines: stmt.groupBy.length ? [`by ${stmt.groupBy.map((c) => c.name).join(', ')}`] : [],
    });
  }
  if (stmt.orderBy.length) {
    pipeline.push({
      title: 'order by',
      lines: [stmt.orderBy.map((o) => `${exprName(o.expr)} ${o.dir}`).join(', ')],
    });
  }
  if (stmt.limit != null) pipeline.push({ title: `limit ${stmt.limit}`, lines: [] });
  pipeline.push({ title: 'project · SELECT', lines: [projectionNames(plan).join(', ')] });

  return { inputs, pipeline };
}
