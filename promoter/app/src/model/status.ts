// Promotion status, sourced from ConfigHub — not from the workflow.
//
// ConfigHub manages the live resources behind each variant Space, so the
// status of a variant belongs there, as a Space label. The app only reads it.
// Today an operator (or a CLI command, simulating a future agent) sets the
// label; eventually agents watching Argo/etc. will write it. Either way the UI
// reads the same label, so nothing here changes when that happens.
//
// Simulate a status change from the CLI:
//   cub space update --patch <variant-space> --label "Status=Progressing"
//   cub space update --patch <variant-space> --label "Status=Ready"

import { PromotionState } from './workflow';

/** Map a raw Space-label value to a state. Lenient and case-insensitive. */
export function mapStatus(raw: string | undefined): PromotionState {
  const v = (raw ?? '').trim().toLowerCase();
  if (v === '') return 'unknown';
  if (['succeeded', 'success', 'ready', 'healthy', 'deployed', 'synced', 'done'].includes(v)) {
    return 'succeeded';
  }
  if (['failed', 'failure', 'error', 'degraded', 'unhealthy', 'crashloopbackoff'].includes(v)) {
    return 'failed';
  }
  if (['in_progress', 'inprogress', 'progressing', 'deploying', 'running', 'pending'].includes(v)) {
    return 'in_progress';
  }
  if (['notdeployed', 'not_deployed', 'none', 'idle'].includes(v)) return 'pending';
  return 'unknown';
}

export interface StatusProvider {
  /** Human-readable description of where status comes from. */
  readonly source: string;
  /** The raw label value for a variant, or undefined if unset. */
  raw(spaceLabels: Record<string, string> | undefined, labelKey: string): string | undefined;
  /** The mapped state for a variant. */
  get(spaceLabels: Record<string, string> | undefined, labelKey: string): PromotionState;
}

/**
 * Reads status from a label on the variant's Space. This is the realized
 * "ConfigHub/agent-reported" model: whoever manages the live resources writes
 * the label, the app reads it. The app never writes status itself.
 */
export class LabelStatusProvider implements StatusProvider {
  readonly source = 'ConfigHub Space label';

  raw(spaceLabels: Record<string, string> | undefined, labelKey: string): string | undefined {
    return spaceLabels?.[labelKey];
  }

  get(spaceLabels: Record<string, string> | undefined, labelKey: string): PromotionState {
    return mapStatus(this.raw(spaceLabels, labelKey));
  }
}

/** The provider the app currently uses. */
export const statusProvider: StatusProvider = new LabelStatusProvider();

/** Roll several component states up into one stage state. */
export function rollupStatus(states: PromotionState[]): PromotionState {
  if (states.length === 0) return 'unknown';
  if (states.some((s) => s === 'failed')) return 'failed';
  if (states.some((s) => s === 'in_progress')) return 'in_progress';
  if (states.every((s) => s === 'succeeded')) return 'succeeded';
  if (states.some((s) => s === 'succeeded')) return 'in_progress'; // partially promoted
  return 'pending';
}
