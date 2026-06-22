// The promotion-workflow document. One Workflow is stored as the YAML body
// (Unit.Data) of a single AppConfig/YAML unit in the `promoter` space. All
// durable workflow state — stage order, per-component variant choices, and
// promotion status — lives here, never on Space metadata (a variant Space may
// participate in many workflows).

import { parse, stringify } from 'yaml';

export const WORKFLOW_API_VERSION = 'promoter.confighub.com/v1';

export type PromotionState = 'pending' | 'succeeded' | 'failed' | 'unknown';

export interface PromotionStatus {
  state: PromotionState;
  /** Head revision the target units were upgraded to when this was recorded. */
  promotedRevision?: number;
  /** ISO-8601 timestamp the status was recorded. */
  at?: string;
  /** DisplayName of the user who recorded it (manual provider). */
  by?: string;
}

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
  stages: Stage[];
  /** Keyed by statusKey(stage, component). */
  status: Record<string, PromotionStatus>;
}

export function statusKey(stage: string, component: string): string {
  return `${stage}/${component}`;
}

export function emptyWorkflow(name: string): Workflow {
  return { apiVersion: WORKFLOW_API_VERSION, name, stages: [], status: {} };
}

/** Serialize a Workflow to the YAML stored in Unit.Data. */
export function serializeWorkflow(wf: Workflow): string {
  return stringify({
    apiVersion: wf.apiVersion || WORKFLOW_API_VERSION,
    name: wf.name,
    stages: wf.stages,
    status: wf.status,
  });
}

/**
 * Parse a Workflow from Unit.Data YAML. Tolerant of partially-formed
 * documents (missing stages/status) so a hand-edited or older unit still
 * loads. `fallbackName` is used when the document omits `name`.
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
  const status: Record<string, PromotionStatus> =
    obj.status && typeof obj.status === 'object' ? (obj.status as Workflow['status']) : {};
  return {
    apiVersion: typeof obj.apiVersion === 'string' ? obj.apiVersion : WORKFLOW_API_VERSION,
    name: typeof obj.name === 'string' && obj.name !== '' ? obj.name : fallbackName,
    stages,
    status,
  };
}

/** Index of the stage immediately upstream of `stageName`, or -1 if first/missing. */
export function upstreamStageIndex(wf: Workflow, stageName: string): number {
  const i = wf.stages.findIndex((s) => s.name === stageName);
  return i > 0 ? i - 1 : -1;
}
