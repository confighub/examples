// Load the fleet's cost snapshot: list the cost-estimator Units, read the cost
// annotations the estimator wrote onto each workload, and the ApplyGates the
// guardrails set. All reads go through the typed openapi-fetch client.

import { parseAllDocuments } from 'yaml';

import { cub, type Schemas } from '../sdk/client';
import { ANNO, type BudgetStatus, type CostRow } from './model';

const WORKLOAD_KINDS = new Set(['Deployment', 'StatefulSet']);

/** Load one CostRow per workload Unit across the demo fleet (or a custom glob). */
export async function loadSnapshot(spaceGlob = 'cost-demo-%'): Promise<CostRow[]> {
  const where = `Labels.app = 'cost-estimator' AND Space.Slug LIKE '${spaceGlob}'`;
  const { data, error, response } = await cub.GET('/unit', {
    params: { query: { where, select: 'Slug,UnitID,SpaceID,Labels,ApplyGates', include: 'SpaceID' } },
  });
  if (error || !data) throw new Error(`GET /unit: HTTP ${response.status}`);

  const rows: CostRow[] = [];
  for (const e of data) {
    const u: Partial<Schemas['Unit']> = e.Unit ?? {};
    const sid = u.SpaceID;
    const uid = u.UnitID;
    if (!sid || !uid) continue;
    const dataRes = await cub.GET('/space/{space_id}/unit/{unit_id}/data', {
      params: { path: { space_id: sid, unit_id: uid } },
      parseAs: 'text',
    });
    if (dataRes.error || dataRes.data === undefined) continue;
    const wl = workloadAnnotations(dataRes.data);
    if (!wl) continue; // not a workload (e.g. a record / status Unit)

    const labels = u.Labels ?? {};
    rows.push({
      space: e.Space?.Slug ?? '',
      unit: u.Slug ?? '',
      environment: labels.Environment ?? '',
      provider: wl.a['provider'] ?? labels.Provider ?? '',
      region: wl.a['region'] ?? labels.Region ?? '',
      workload: labels.workload ?? u.Slug ?? '',
      kind: wl.kind,
      monthlyUsd: numOrNull(wl.a['monthly-usd']),
      cpuCores: numOrNull(wl.a['cpu-cores']),
      memoryGb: numOrNull(wl.a['memory-gb']),
      storageGb: numOrNull(wl.a['storage-gb']),
      budgetStatus: (wl.a['budget-status'] as BudgetStatus) || 'UNKNOWN',
      estimatedAt: wl.a['estimated-at'] ?? '',
      pricingVersion: wl.a['pricing-version'] ?? '',
      gates: Object.keys(u.ApplyGates ?? {}).map(triggerOf),
    });
  }
  rows.sort((a, b) => (b.monthlyUsd ?? -1) - (a.monthlyUsd ?? -1));
  return rows;
}

/** Find the workload doc in a (possibly multi-doc) manifest and return its
 *  cost-estimator annotations, keyed without the namespace prefix. */
function workloadAnnotations(text: string): { kind: string; a: Record<string, string> } | null {
  for (const doc of parseAllDocuments(text)) {
    const obj = doc.toJS() as { kind?: string; metadata?: { annotations?: Record<string, unknown> } } | null;
    if (!obj || typeof obj.kind !== 'string' || !WORKLOAD_KINDS.has(obj.kind)) continue;
    const annos = obj.metadata?.annotations ?? {};
    const a: Record<string, string> = {};
    for (const [k, v] of Object.entries(annos)) {
      if (k.startsWith(ANNO)) a[k.slice(ANNO.length)] = String(v);
    }
    return { kind: obj.kind, a };
  }
  return null;
}

/** "cost-demo-policy/within-budget/vet-celexpr" → "within-budget". */
function triggerOf(gateKey: string): string {
  const parts = gateKey.split('/');
  return parts.length >= 2 ? parts[1] : gateKey;
}

function numOrNull(s?: string): number | null {
  if (s == null || s === '') return null;
  const n = Number(s);
  return Number.isFinite(n) ? n : null;
}
