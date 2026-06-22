// The promotion-workflow document. One Workflow is stored as the YAML body
// (Unit.Data) of a single AppConfig/YAML unit in the `promoter` space. The
// document holds the pipeline shape — stage order and per-component variant
// choices. It deliberately does NOT store promotion status: status belongs to
// ConfigHub (which manages the live resources) and is read from a label on
// each variant Space (see model/status.ts). The document only names which
// label key to read.

import { parse, stringify } from 'yaml';

export const WORKFLOW_API_VERSION = 'promoter.confighub.com/v1';

/** Default Space-label key whose value carries a variant's live status. */
export const DEFAULT_STATUS_LABEL = 'Status';

export type PromotionState = 'pending' | 'in_progress' | 'succeeded' | 'failed' | 'unknown';

/** A component's chosen variant within a stage. */
export interface ComponentChoice {
  /** Space label `Component` — the logical service. */
  component: string;
  /** Space label `Variant` — which variant Space feeds this stage. */
  variant: string;
}

export interface Stage {
  name: string;
  components: ComponentChoice[];
}

export interface Workflow {
  apiVersion: string;
  /** Human-readable name (the unit slug is the durable identity). */
  name: string;
  /** Space-label key to read each variant's live status from. */
  statusLabel: string;
  stages: Stage[];
}

export function emptyWorkflow(name: string): Workflow {
  return {
    apiVersion: WORKFLOW_API_VERSION,
    name,
    statusLabel: DEFAULT_STATUS_LABEL,
    stages: [],
  };
}

/** Serialize a Workflow to the YAML stored in Unit.Data. */
export function serializeWorkflow(wf: Workflow): string {
  return stringify({
    apiVersion: wf.apiVersion || WORKFLOW_API_VERSION,
    name: wf.name,
    statusLabel: wf.statusLabel || DEFAULT_STATUS_LABEL,
    stages: wf.stages,
  });
}

/**
 * Parse a Workflow from Unit.Data YAML. Tolerant of partially-formed or older
 * documents (missing stages, or a legacy `status` map which is ignored).
 * `fallbackName` is used when the document omits `name`.
 */
export function parseWorkflow(yaml: string, fallbackName: string): Workflow {
  let doc: unknown;
  try {
    doc = parse(yaml);
  } catch {
    doc = null;
  }
  const obj = (doc ?? {}) as Partial<Workflow>;
  const stages: Stage[] = Array.isArray(obj.stages)
    ? obj.stages.map((s) => ({
        name: String((s as Stage)?.name ?? ''),
        components: Array.isArray((s as Stage)?.components)
          ? (s as Stage).components.map((c) => ({
              component: String((c as ComponentChoice)?.component ?? ''),
              variant: String((c as ComponentChoice)?.variant ?? ''),
            }))
          : [],
      }))
    : [];
  return {
    apiVersion: typeof obj.apiVersion === 'string' ? obj.apiVersion : WORKFLOW_API_VERSION,
    name: typeof obj.name === 'string' && obj.name !== '' ? obj.name : fallbackName,
    statusLabel:
      typeof obj.statusLabel === 'string' && obj.statusLabel !== ''
        ? obj.statusLabel
        : DEFAULT_STATUS_LABEL,
    stages,
  };
}
