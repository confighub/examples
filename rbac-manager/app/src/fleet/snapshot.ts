// Fleet snapshot loader: one org-wide get-resources invocation (bodies
// included) joined with unit/space/target metadata, parsed into the RBAC
// engine's model.

import { useCallback, useMemo, useRef, useState } from 'react';

import { b64decodeUtf8 } from '../api/encoding';
import { buildClusterRbac, ClusterRbac, FleetResource } from '../rbac/model';
import {
  ExtendedUnitRead,
  useInvokeFunctionsOnOrgMutation,
  useLazyListAllUnitsQuery,
} from '../sdk/confighubapi.gen';

/** Units managed by this app carry the app=rbac-manager label. */
export const FLEET_UNITS_WHERE = "Labels.app = 'rbac-manager'";

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

/** Space roles whose Units are canonical definitions, not deployed config. */
const CANONICAL_SPACE_ROLES = new Set(['base', 'policy']);

export interface FleetSnapshot {
  /** RBAC entities per cluster (Target slug; Space slug for unbound Units). */
  clusters: Map<string, ClusterRbac>;
  /** Every parsed resource, for the explorer table. */
  resources: FleetResource[];
  /** Unit metadata by UnitID, for status display (gates, revisions). */
  units: Map<string, ExtendedUnitRead>;
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

  const [snapshot, setSnapshot] = useState<FleetSnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const callIdRef = useRef(0);

  const refresh = useCallback(async () => {
    const callId = ++callIdRef.current;
    setIsLoading(true);
    setError(null);
    try {
      const [invokeResult, unitsResult] = await Promise.all([
        invoke({
          where: FLEET_UNITS_WHERE,
          functionInvocationsRequest: {
            FunctionInvocations: [
              {
                FunctionName: 'get-resources',
                Arguments: [{ ParameterName: 'body', Value: 'json' }],
              },
            ],
          },
        }),
        listUnits({
          where: FLEET_UNITS_WHERE,
          select:
            'UnitID,Slug,DisplayName,SpaceID,TargetID,Labels,ApplyGates,' +
            'HeadRevisionNum,LiveRevisionNum,UpstreamRevisionNum,LastChangeDescription',
          include: 'SpaceID,TargetID',
        }),
      ]);

      if (callId !== callIdRef.current) return; // a newer refresh superseded us

      if ('error' in invokeResult && invokeResult.error) {
        setError('get-resources invocation failed');
        return;
      }
      if (unitsResult.error) {
        setError('unit metadata fetch failed');
        return;
      }

      const units = new Map<string, ExtendedUnitRead>();
      for (const eu of unitsResult.data ?? []) {
        const id = eu.Unit?.UnitID;
        if (id !== undefined) units.set(id, eu);
      }

      const resources: FleetResource[] = [];
      for (const response of invokeResult.data ?? []) {
        if (!response.Success || !response.UnitID) continue;
        const eu = units.get(response.UnitID);
        const space = response.SpaceSlug ?? eu?.Space?.Slug ?? '';
        const target = eu?.Target?.Slug;
        const canonical = CANONICAL_SPACE_ROLES.has(eu?.Space?.Labels?.role ?? '');
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
              unitSlug: response.UnitSlug ?? eu?.Unit?.Slug ?? '',
              resourceName: raw.ResourceName ?? '',
              canonical,
            },
            doc,
          });
        }
      }

      setSnapshot({
        // Canonical (base/policy) definitions stay out of cluster analysis:
        // nothing deploys there, so they'd produce phantom grants/findings.
        clusters: buildClusterRbac(resources.filter((r) => r.origin.canonical !== true)),
        resources,
        units,
        loadedAt: Date.now(),
      });
    } finally {
      if (callId === callIdRef.current) setIsLoading(false);
    }
  }, [invoke, listUnits]);

  return useMemo(
    () => ({ snapshot, isLoading, error, refresh }),
    [snapshot, isLoading, error, refresh],
  );
}
