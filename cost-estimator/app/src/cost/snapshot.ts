// Load the fleet's cost snapshot: the cost-estimator Units, the cost annotations the
// estimator wrote onto each workload, and the ApplyGates the guardrails set.
//
// Two requests, not one per Unit: the list carries the metadata (labels, gates, Space)
// and the bulk data endpoint carries the configurations, joined on UnitID. A read per
// Unit is the difference between two round trips and a thousand.

import type { components } from '@confighub/api';
import { confighub, listUnitData } from '@confighub/examples-webkit/api';
import { parseAllDocuments } from 'yaml';

import { ANNO, type BudgetStatus, type CostRow } from './model';

const WORKLOAD_KINDS = new Set(['Deployment', 'StatefulSet']);

/** Load one CostRow per workload Unit across the demo fleet (or a custom glob). */
export async function loadSnapshot(spaceGlob = 'cost-demo-%'): Promise<CostRow[]> {
  const where = `Labels.app = 'cost-estimator' AND Space.Slug LIKE '${spaceGlob}'`;
  const [listed, documents] = await Promise.all([
    confighub().GET('/unit', {
      params: { query: { where, select: 'Slug,UnitID,SpaceID,Labels,ApplyGates', include: 'SpaceID' } },
    }),
    listUnitData({ where }),
  ]);
  if (listed.error !== undefined || listed.data === undefined) {
    throw new Error(`GET /unit: HTTP ${listed.response.status}`);
  }

  const configurations = new Map(
    documents.filter((d) => d.UnitID !== undefined).map((d) => [d.UnitID!, d.Data ?? '']),
  );

  const rows: CostRow[] = [];
  for (const e of listed.data) {
    const u: Partial<components['schemas']['Unit']> = e.Unit ?? {};
    const uid = u.UnitID;
    if (uid === undefined) continue;
    const configuration = configurations.get(uid);
    if (configuration === undefined) continue;
    const wl = workloadAnnotations(configuration);
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
