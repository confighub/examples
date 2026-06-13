// Fleet snapshot loader. Discovers every Kubernetes/YAML Unit the user can
// view (optionally narrowed by the scope filter expressions), then extracts
// just the RBAC resources server-side:
//   - `where_data` skips units whose config contains no RBAC kinds at all,
//     so rendered-chart units without RBAC never ship their data back;
//   - request-level `WhereResource` makes get-resources return only the
//     RBAC resources of the units that remain.
// Two invocations run in parallel because WhereResource conjunctions can't
// express "rbac.authorization.k8s.io/* OR v1/ServiceAccount".

import { useCallback, useMemo, useRef, useState } from 'react';

import { b64decodeUtf8 } from '../api/encoding';
import { buildClusterRbac, ClusterRbac, FleetResource } from '../rbac/model';
import {
  ExtendedUnitRead,
  FunctionInvocationsResponse,
  useInvokeFunctionsOnOrgMutation,
  useLazyListAllTargetsQuery,
  useLazyListAllUnitsQuery,
  useLazyListSpacesQuery,
} from '../sdk/confighubapi.gen';
import { FleetScope, loadScope } from './scope';

const K8S_UNITS_WHERE = "ToolchainType = 'Kubernetes/YAML'";

const RBAC_WHERE_DATA = "kind IN ('Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding')";
const RBAC_WHERE_RESOURCE = "ConfigHub.ResourceType LIKE 'rbac.authorization.k8s.io/%'";
const SA_WHERE_DATA = "kind = 'ServiceAccount'";
const SA_WHERE_RESOURCE = "ConfigHub.ResourceType = 'v1/ServiceAccount'";

interface RawResource {
  ResourceType?: string;
  ResourceName?: string;
  ResourceBody?: string;
}

/** Outputs.ResourceList is base64-encoded JSON (api.Resource[]). */
function decodeResourceList(encoded: string): RawResource[] {
  try {
    const parsed: unknown = JSON.parse(b64decodeUtf8(encoded));
    return Array.isArray(parsed) ? (parsed as RawResource[]) : [];
  } catch {
    return [];
  }
}

// Canonical Spaces hold definitions, not deployed config, so their Units stay
// out of cluster analysis. The standard `Variant=base` label marks a base/
// template Space; the demo fleet additionally uses a `role` label.
const CANONICAL_VARIANTS = new Set(['base']);
const CANONICAL_DEMO_ROLES = new Set(['base', 'policy']);
function isCanonicalSpace(labels: Record<string, string> | undefined): boolean {
  return (
    CANONICAL_VARIANTS.has(labels?.Variant ?? '') ||
    CANONICAL_DEMO_ROLES.has(labels?.role ?? '')
  );
}

export interface FleetSnapshot {
  /** RBAC entities per cluster (Target slug; Space slug for unbound Units). */
  clusters: Map<string, ClusterRbac>;
  /** Every parsed resource, for the explorer table. */
  resources: FleetResource[];
  /** In-scope unit metadata by UnitID (gates, warnings, revisions). */
  units: Map<string, ExtendedUnitRead>;
  /** The scope the snapshot was loaded with. */
  scope: FleetScope;
  loadedAt: number;
}

export interface UseFleetSnapshotResult {
  snapshot: FleetSnapshot | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

/**
 * Imperative snapshot fetch. Pages call refresh() on mount (cheap no-op when
 * a snapshot already exists) and expose a Refresh action; no polling.
 */
export function useFleetSnapshot(): UseFleetSnapshotResult {
  const [invoke] = useInvokeFunctionsOnOrgMutation();
  const [listUnits] = useLazyListAllUnitsQuery();
  const [listSpaces] = useLazyListSpacesQuery();
  const [listTargets] = useLazyListAllTargetsQuery();

  const [snapshot, setSnapshot] = useState<FleetSnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const callIdRef = useRef(0);

  const refresh = useCallback(async () => {
    const callId = ++callIdRef.current;
    setIsLoading(true);
    setError(null);
    const scope = loadScope();
    try {
      const invokeArgs = (whereData: string, whereResource: string) => ({
        where: K8S_UNITS_WHERE,
        whereData,
        functionInvocationsRequest: {
          WhereResource: whereResource,
          FunctionInvocations: [
            {
              FunctionName: 'get-resources',
              Arguments: [{ ParameterName: 'body', Value: 'json' }],
            },
          ],
        },
      });

      const [rbacResult, saResult, unitsResult, spacesResult, targetsResult] = await Promise.all([
        invoke(invokeArgs(RBAC_WHERE_DATA, RBAC_WHERE_RESOURCE)),
        invoke(invokeArgs(SA_WHERE_DATA, SA_WHERE_RESOURCE)),
        listUnits({
          where: K8S_UNITS_WHERE,
          select:
            'UnitID,Slug,DisplayName,SpaceID,TargetID,Labels,ApplyGates,ApplyWarnings,' +
            'HeadRevisionNum,LiveRevisionNum,UpstreamRevisionNum,UpstreamUnitID,LastChangeDescription',
          include: 'SpaceID,TargetID',
        }),
        listSpaces({ where: scope.spaceWhere === '' ? undefined : scope.spaceWhere }),
        listTargets({
          where: scope.targetWhere === '' ? undefined : scope.targetWhere,
          select: 'TargetID,Slug',
        }),
      ]);

      if (callId !== callIdRef.current) return; // a newer refresh superseded us

      for (const [label, r] of [
        ['RBAC resources', rbacResult],
        ['ServiceAccounts', saResult],
      ] as const) {
        if ('error' in r && r.error) {
          setError(`${label} fetch failed`);
          return;
        }
      }
      if (unitsResult.error || spacesResult.error || targetsResult.error) {
        setError(
          spacesResult.error || targetsResult.error
            ? 'Scope filter query failed — check the filter expressions in Settings.'
            : 'Unit metadata fetch failed',
        );
        return;
      }

      const scopedSpaceIds = new Set(
        (spacesResult.data ?? []).map((s) => s.Space?.SpaceID).filter((id) => id !== undefined),
      );
      const scopedTargetIds = new Set(
        (targetsResult.data ?? []).map((t) => t.Target?.TargetID).filter((id) => id !== undefined),
      );

      // Scope rule: targeted units are in scope iff their Target matches the
      // target filter; untargeted (base) units iff their Space matches the
      // space filter.
      const units = new Map<string, ExtendedUnitRead>();
      for (const eu of unitsResult.data ?? []) {
        const id = eu.Unit?.UnitID;
        if (id === undefined) continue;
        const targetId = eu.Unit?.TargetID;
        const inScope =
          targetId !== undefined && targetId !== null
            ? scopedTargetIds.has(targetId)
            : scopedSpaceIds.has(eu.Unit?.SpaceID ?? '');
        if (inScope) units.set(id, eu);
      }

      const resources: FleetResource[] = [];
      const collect = (responses: FunctionInvocationsResponse[] | undefined) => {
        for (const response of responses ?? []) {
          if (!response.Success || !response.UnitID) continue;
          const eu = units.get(response.UnitID);
          if (!eu) continue; // out of scope
          const space = response.SpaceSlug ?? eu.Space?.Slug ?? '';
          const target = eu.Target?.Slug;
          const canonical = isCanonicalSpace(eu.Space?.Labels);
          for (const raw of decodeResourceList(response.Outputs?.['ResourceList'] ?? '')) {
            if (raw.ResourceBody === undefined || raw.ResourceBody === '') continue;
            let doc: unknown;
            try {
              doc = JSON.parse(raw.ResourceBody);
            } catch {
              continue;
            }
            resources.push({
              origin: {
                cluster: target ?? space,
                target,
                space,
                spaceId: response.SpaceID ?? '',
                unitId: response.UnitID,
                unitSlug: response.UnitSlug ?? eu.Unit?.Slug ?? '',
                resourceName: raw.ResourceName ?? '',
                canonical,
              },
              doc,
            });
          }
        }
      };
      collect(rbacResult.data);
      collect(saResult.data);

      setSnapshot({
        // Canonical (base/policy) definitions stay out of cluster analysis:
        // nothing deploys there, so they'd produce phantom grants/findings.
        clusters: buildClusterRbac(resources.filter((r) => r.origin.canonical !== true)),
        resources,
        units,
        scope,
        loadedAt: Date.now(),
      });
    } finally {
      if (callId === callIdRef.current) setIsLoading(false);
    }
  }, [invoke, listUnits, listSpaces, listTargets]);

  return useMemo(
    () => ({ snapshot, isLoading, error, refresh }),
    [snapshot, isLoading, error, refresh],
  );
}
