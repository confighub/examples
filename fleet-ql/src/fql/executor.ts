// The executor runs an ExecutionPlan against a Transport: it issues one fetch
// per DNF AND-group, unions and de-dupes the resulting rows (since OR branches
// overlap), then runs the client-side evaluate() phase to produce the final,
// exactly-filtered result. Pushdown narrowed the fetch; evaluate decides the
// answer.

import { evaluate, type ResultSet, type Row } from './evaluate';
import type { ExecutionPlan, FetchSpec } from './planner';
import type { GrantsParams, ListParams, ResourceParams, Transport } from './transport';

export interface RunStats {
  /** How many server fetches were issued (= DNF AND-groups, post-dedup). */
  fetches: number;
  /** Rows returned by the server before client-side filtering. */
  fetchedRows: number;
  /** Rows in the final result. */
  resultRows: number;
}

export interface RunResult extends ResultSet {
  stats: RunStats;
}

/** Stable identity for de-duping rows unioned across OR-branch fetches. */
function rowKey(source: ExecutionPlan['source'], row: Row): string {
  switch (source) {
    case 'units':
      return String(row['__id'] ?? `${row['space']}/${row['slug']}`);
    case 'resources':
      // A Unit can hold multiple resources; key on the resource identity, plus
      // the revision so the same resource at different revisions isn't deduped.
      return `${row['space']}/${row['unit']}/${row['resourceType'] ?? ''}/${row['name'] ?? ''}@${row['revision'] ?? 'head'}`;
    case 'revisions':
      // One revision = (space, unit, revision number).
      return `${row['space']}/${row['unit']}/${row['RevisionNum'] ?? ''}`;
    case 'spaces':
      return String(row['__id'] ?? row['slug']);
    case 'grants':
      // One grant = (cluster, space/unit, binding, subject, scope).
      return `${row['cluster']}/${row['space']}/${row['unit']}/${row['binding']}/${row['subject']}/${row['scope'] ?? ''}`;
  }
}

async function fetchFor(
  transport: Transport,
  source: ExecutionPlan['source'],
  spec: FetchSpec,
): Promise<Row[]> {
  switch (source) {
    case 'units':
      return transport.units({ where: spec.where } as ListParams);
    case 'resources':
      return transport.resources({
        where: spec.where,
        whereData: spec.whereData,
        whereResource: spec.whereResource,
        revision: spec.revision,
      } as ResourceParams);
    case 'spaces':
      return transport.spaces({ where: spec.where } as ListParams);
    case 'revisions':
      return transport.revisions({ whereUnit: spec.whereUnit, where: spec.where });
    case 'grants':
      return transport.grants({ where: spec.where, accessQuery: spec.accessQuery } as GrantsParams);
  }
}

/** Execute a plan: fetch (per group), union+dedupe, then evaluate client-side. */
export async function execute(plan: ExecutionPlan, transport: Transport): Promise<RunResult> {
  // Issue all group fetches concurrently.
  const batches = await Promise.all(plan.fetches.map((spec) => fetchFor(transport, plan.source, spec)));

  // Union + de-dupe.
  const byKey = new Map<string, Row>();
  for (const batch of batches) {
    for (const row of batch) {
      const k = rowKey(plan.source, row);
      if (!byKey.has(k)) byKey.set(k, row);
    }
  }
  const fetched = [...byKey.values()];

  // Columns present across fetched rows (for SELECT *).
  const colSet = new Set<string>();
  for (const r of fetched) for (const k of Object.keys(r)) if (!k.startsWith('__')) colSet.add(k);
  const allColumns = [...colSet];

  const result = evaluate(plan.stmt, fetched, allColumns, plan.residual);

  return {
    ...result,
    stats: {
      fetches: plan.fetches.length,
      fetchedRows: fetched.length,
      resultRows: result.rows.length,
    },
  };
}
