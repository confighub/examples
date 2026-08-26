// Fleet snapshot loader. Discovers every Kubernetes/YAML Unit in scope, then extracts
// just the RBAC resources server-side:
//   - `whereData` skips Units whose configuration contains no RBAC kinds at all, so a
//     rendered chart with no RBAC never ships its data back;
//   - `whereResource` makes get-resources return only the RBAC resources of the Units
//     that remain.
// Two invocations run in parallel because a WhereResource conjunction cannot express
// "rbac.authorization.k8s.io/* OR v1/ServiceAccount".

import { getResources, resourceDocs } from '@confighub/examples-webkit/api';
import {
  createSnapshotContext,
  isCanonicalSpace,
  loadScopedUnits,
  type ExtendedUnit,
  type FleetScope,
  type Origin,
} from '@confighub/examples-webkit/fleet';
import { buildClusterRbac, type ClusterRbac, type FleetResource } from '@confighub/examples-webkit/rbac';

import { scopeStore } from './scope';

const K8S_UNITS_WHERE = "ToolchainType = 'Kubernetes/YAML'";

const RBAC_WHERE_DATA = "kind IN ('Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding')";
const RBAC_WHERE_RESOURCE = "ConfigHub.ResourceType LIKE 'rbac.authorization.k8s.io/%'";
const SA_WHERE_DATA = "kind = 'ServiceAccount'";
const SA_WHERE_RESOURCE = "ConfigHub.ResourceType = 'v1/ServiceAccount'";

export interface FleetSnapshot {
  /** RBAC entities per cluster (Target slug; Space slug for unbound Units). */
  clusters: Map<string, ClusterRbac>;
  /** Every parsed resource, for the explorer table. */
  resources: FleetResource[];
  /** In-scope unit metadata by UnitID (gates, warnings, revisions). */
  units: Map<string, ExtendedUnit>;
}

async function build(scope: FleetScope): Promise<FleetSnapshot> {
  const [rbacResponses, saResponses, scoped] = await Promise.all([
    getResources({
      where: K8S_UNITS_WHERE,
      whereData: RBAC_WHERE_DATA,
      whereResource: RBAC_WHERE_RESOURCE,
    }),
    getResources({
      where: K8S_UNITS_WHERE,
      whereData: SA_WHERE_DATA,
      whereResource: SA_WHERE_RESOURCE,
    }),
    loadScopedUnits(scope, { where: K8S_UNITS_WHERE }),
  ]);

  const resources: FleetResource[] = [];
  for (const response of [...rbacResponses, ...saResponses]) {
    if (!response.Success || !response.UnitID) continue;
    const eu = scoped.units.get(response.UnitID);
    if (!eu) continue; // out of scope
    const space = response.SpaceSlug ?? eu.Space?.Slug ?? '';
    const target = eu.Target?.Slug;
    const origin: Omit<Origin, 'resourceName'> = {
      cluster: target ?? space,
      target,
      space,
      spaceId: response.SpaceID ?? '',
      unitId: response.UnitID,
      unitSlug: response.UnitSlug ?? eu.Unit?.Slug ?? '',
      canonical: isCanonicalSpace(eu.Space?.Labels),
    };
    for (const { raw, doc } of resourceDocs(response)) {
      resources.push({ origin: { ...origin, resourceName: raw.ResourceName ?? '' }, doc });
    }
  }

  return {
    // Canonical (base/policy) definitions stay out of cluster analysis: nothing deploys
    // there, so they would produce phantom grants and findings.
    clusters: buildClusterRbac(resources.filter((r) => r.origin.canonical !== true)),
    resources,
    units: scoped.units,
  };
}

export const { SnapshotProvider, useSnapshot } = createSnapshotContext<FleetSnapshot>(
  scopeStore,
  build,
);
