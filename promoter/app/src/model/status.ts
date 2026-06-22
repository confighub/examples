// Pluggable promotion-status abstraction.
//
// ConfigHub's built-in status fields (LiveRevisionNum, apply state) are being
// phased out, and the long-term plan is for agents to report stage health from
// Argo and other sources into ConfigHub. Until those agents exist, the app
// records status manually. The UI only ever talks to the StatusProvider
// interface, so swapping in an agent-reported provider later needs no page
// changes.

import {
  PromotionState,
  PromotionStatus,
  statusKey,
  Workflow,
} from './workflow';

export interface StatusProvider {
  /** Whether users can set status through this provider (manual=true). */
  readonly canEdit: boolean;
  /** Current status for a (stage, component) cell. */
  get(wf: Workflow, stage: string, component: string): PromotionStatus;
  /**
   * Record a new state. Returns the updated Workflow (callers persist it).
   * Undefined when the provider is read-only.
   */
  set?(
    wf: Workflow,
    stage: string,
    component: string,
    state: PromotionState,
    extra?: Pick<PromotionStatus, 'promotedRevision' | 'by'>,
  ): Workflow;
}

const UNKNOWN: PromotionStatus = { state: 'unknown' };

/**
 * Default provider: status is whatever a user records, persisted inside the
 * workflow document's `status` map. `set` returns a new Workflow; the caller
 * saves it back to the unit.
 */
export class ManualStatusProvider implements StatusProvider {
  readonly canEdit = true;

  /** `now` is injected so callers (and tests) control the timestamp. */
  constructor(private readonly now: () => string = () => new Date().toISOString()) {}

  get(wf: Workflow, stage: string, component: string): PromotionStatus {
    return wf.status[statusKey(stage, component)] ?? UNKNOWN;
  }

  set(
    wf: Workflow,
    stage: string,
    component: string,
    state: PromotionState,
    extra?: Pick<PromotionStatus, 'promotedRevision' | 'by'>,
  ): Workflow {
    const next: PromotionStatus = { state, at: this.now() };
    if (extra?.promotedRevision !== undefined) next.promotedRevision = extra.promotedRevision;
    if (extra?.by !== undefined) next.by = extra.by;
    return {
      ...wf,
      status: { ...wf.status, [statusKey(stage, component)]: next },
    };
  }
}

/**
 * Stub for the future model: stage health reported into ConfigHub by agents
 * watching Argo/Flux/etc. Read-only and currently always `unknown` — wired in
 * here so the shape is documented and the swap is a one-line provider change.
 * When such agents exist, `get` would read their signal (e.g. a status unit or
 * annotation) instead of returning unknown.
 */
export class AgentReportedStatusProvider implements StatusProvider {
  readonly canEdit = false;

  get(): PromotionStatus {
    return UNKNOWN;
  }
}

/** The provider the app currently uses. */
export const statusProvider: StatusProvider = new ManualStatusProvider();
