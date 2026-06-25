// The cost domain model: one row per workload, plus presentation helpers. All
// values come from the cost-estimator.confighub.com/* annotations the estimator
// wrote onto each Unit (read in snapshot.ts).

export type BudgetStatus = 'OK' | 'WARN' | 'OVER' | 'UNKNOWN';

export const ANNO = 'cost-estimator.confighub.com/';

export interface CostRow {
  space: string;
  unit: string;
  environment: string;
  region: string;
  workload: string;
  kind: string;
  monthlyUsd: number | null;
  cpuCores: number | null;
  memoryGb: number | null;
  storageGb: number | null;
  budgetStatus: BudgetStatus;
  estimatedAt: string;
  pricingVersion: string;
  /** Trigger slugs currently gating this Unit's apply (from ApplyGates). */
  gates: string[];
}

/** MUI palette color for a budget verdict. */
export function statusColor(s: BudgetStatus): 'success' | 'warning' | 'error' | 'default' {
  switch (s) {
    case 'OK':
      return 'success';
    case 'WARN':
      return 'warning';
    case 'OVER':
      return 'error';
    default:
      return 'default';
  }
}

export function usd(v: number | null): string {
  if (v == null) return '—';
  return v.toLocaleString('en-US', { style: 'currency', currency: 'USD' });
}

export function num(v: number | null, digits = 2): string {
  return v == null ? '—' : v.toFixed(digits);
}
