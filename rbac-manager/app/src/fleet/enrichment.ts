// Cluster context lookup for the friendly views: prefer the full cluster snapshot
// (cross-unit roleRef resolution, reverse SA lookups); for canonical base/policy units —
// which are excluded from cluster analysis — use their Space's own definition set, and
// fall back to a transient context built from the unit's own resources so same-unit
// references still resolve.

import {
  buildClusterRbac,
  type ClusterRbac,
  type Finding,
  type FleetResource,
} from '@confighub/examples-webkit/rbac';
import { FleetSnapshot } from './snapshot';

export function clusterContextFor(
  snapshot: FleetSnapshot | null,
  clusterKey: string,
  unitId: string,
): ClusterRbac | undefined {
  if (!snapshot) return undefined;
  const fromClusters = snapshot.clusters.get(clusterKey) ?? snapshot.definitions.get(clusterKey);
  if (fromClusters) return fromClusters;
  const own = snapshot.resources.filter((r) => r.origin.unitId === unitId);
  if (own.length === 0) return undefined;
  return buildClusterRbac(own).values().next().value as ClusterRbac | undefined;
}

/**
 * The resource a Finding is about, so a Finding opens the same detail panel the Explorer
 * does. A Finding carries the origin of the very resource it was raised on, and
 * ResourceName is ConfigHub's own `<namespace>/<name>` identity — so the match is on
 * that, plus kind to separate the Role and the RoleBinding that share a name.
 */
export function resourceForFinding(
  snapshot: FleetSnapshot | null,
  finding: Finding,
): FleetResource | null {
  if (!snapshot) return null;
  const match = snapshot.resources.find((r) => {
    if (r.origin.unitId !== finding.origin.unitId) return false;
    if (r.origin.resourceName !== finding.origin.resourceName) return false;
    const doc = r.doc as { kind?: string } | null;
    return doc?.kind === finding.resourceKind;
  });
  return match ?? null;
}
