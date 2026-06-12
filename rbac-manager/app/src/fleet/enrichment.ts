// Cluster context lookup for the friendly views: prefer the full cluster
// snapshot (cross-unit roleRef resolution, reverse SA lookups); for
// canonical base/policy units — which are excluded from cluster analysis —
// fall back to a transient context built from the unit's own resources so
// same-unit references still resolve.

import { buildClusterRbac, ClusterRbac } from '../rbac/model';
import { FleetSnapshot } from './snapshot';

export function clusterContextFor(
  snapshot: FleetSnapshot | null,
  clusterKey: string,
  unitId: string,
): ClusterRbac | undefined {
  if (!snapshot) return undefined;
  const fromClusters = snapshot.clusters.get(clusterKey);
  if (fromClusters) return fromClusters;
  const own = snapshot.resources.filter((r) => r.origin.unitId === unitId);
  if (own.length === 0) return undefined;
  return buildClusterRbac(own).values().next().value as ClusterRbac | undefined;
}
