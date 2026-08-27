// Which Units an app analyzes. Two rules, and the second is the one that is easy to get
// wrong: a Unit bound to a Target is in scope when its Target matches the target filter,
// and a Unit with no Target is in scope when its Space matches the space filter. Base
// Units are not deployed anywhere, so a target filter can say nothing about them.

import type { components } from '@confighub/api';

import { confighub } from '../api/client';
import type { FleetScope } from './scope';

export type ExtendedUnit = components['schemas']['ExtendedUnit'];

/** The Unit fields every fleet view needs: identity, placement, gates, revisions. */
export const FLEET_UNIT_SELECT =
  'UnitID,Slug,DisplayName,SpaceID,TargetID,Labels,ApplyGates,ApplyWarnings,' +
  'HeadRevisionNum,LastReleasedRevisionNum,UpstreamRevisionNum,UpstreamUnitID,LastChangeDescription';

/**
 * Canonical Spaces hold definitions, not deployed config. Analysis that asks what is
 * running has to leave them out or it reports phantom clusters; the standard
 * `Variant=base` Space label marks them, and the example fleets additionally use `role`.
 */
const CANONICAL_VARIANTS = new Set(['base']);
const CANONICAL_DEMO_ROLES = new Set(['base', 'policy']);

export function isCanonicalSpace(labels: Record<string, string> | undefined): boolean {
  return (
    CANONICAL_VARIANTS.has(labels?.Variant ?? '') || CANONICAL_DEMO_ROLES.has(labels?.role ?? '')
  );
}

/** Raised when the user's filter expressions do not parse or do not match. */
export class ScopeQueryError extends Error {}

export interface ScopedUnitsOptions {
  /** Which Units to list, e.g. "ToolchainType = 'Kubernetes/YAML'". */
  where: string;
  /** Fields to return. Defaults to {@link FLEET_UNIT_SELECT}. */
  select?: string;
}

export interface ScopedUnits {
  /** In-scope Units by UnitID. */
  units: Map<string, ExtendedUnit>;
  /** SpaceIDs the space filter selected — also the scope for Units read by Space. */
  spaceIds: Set<string>;
  /** TargetIDs the target filter selected. */
  targetIds: Set<string>;
}

/** List the Units matching `where`, then narrow them to the scope's Targets and Spaces. */
export async function loadScopedUnits(
  scope: FleetScope,
  options: ScopedUnitsOptions,
): Promise<ScopedUnits> {
  const api = confighub();
  const [unitsResult, spacesResult, targetsResult] = await Promise.all([
    api.GET('/unit', {
      params: {
        query: {
          where: options.where,
          select: options.select ?? FLEET_UNIT_SELECT,
          include: 'SpaceID,TargetID',
        },
      },
    }),
    api.GET('/space', {
      params: { query: { where: scope.spaceWhere === '' ? undefined : scope.spaceWhere } },
    }),
    api.GET('/target', {
      params: {
        query: {
          where: scope.targetWhere === '' ? undefined : scope.targetWhere,
          select: 'TargetID,Slug',
        },
      },
    }),
  ]);

  if (spacesResult.error !== undefined || targetsResult.error !== undefined) {
    throw new ScopeQueryError(
      'Scope filter query failed — check the filter expressions in Settings.',
    );
  }
  if (unitsResult.error !== undefined || unitsResult.data === undefined) {
    throw new Error(`GET /unit: HTTP ${unitsResult.response.status}`);
  }

  const spaceIds = new Set(
    (spacesResult.data ?? [])
      .map((s) => s.Space?.SpaceID)
      .filter((id): id is string => id !== undefined),
  );
  const targetIds = new Set(
    (targetsResult.data ?? [])
      .map((t) => t.Target?.TargetID)
      .filter((id): id is string => id !== undefined),
  );

  const units = new Map<string, ExtendedUnit>();
  for (const eu of unitsResult.data) {
    const id = eu.Unit?.UnitID;
    if (id === undefined) continue;
    const targetId = eu.Unit?.TargetID;
    const inScope =
      targetId !== undefined && targetId !== null && targetId !== ''
        ? targetIds.has(targetId)
        : spaceIds.has(eu.Unit?.SpaceID ?? '');
    if (inScope) units.set(id, eu);
  }

  return { units, spaceIds, targetIds };
}

/** Where a resource came from: the cluster it lands on, and the Unit that holds it. */
export interface Origin {
  /** Target slug, or the Space slug for a Unit with no Target. */
  cluster: string;
  target?: string;
  space: string;
  spaceId: string;
  unitId: string;
  unitSlug: string;
  resourceName: string;
  /** True when the Unit lives in a canonical (base/policy) Space. */
  canonical: boolean;
}
