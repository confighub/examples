// Fleet snapshot loader. The RBAC resources come from ConfigHub's Resource inventory in
// one query — the server extracts and indexes resources as Units change, so asking for
// `rbac.authorization.k8s.io/*` plus `v1/ServiceAccount` across the organization needs no
// per-Unit function execution and ships back no configuration the app does not analyze.
//
// The Unit list runs alongside it: it is what applies the user's scope (Targets and
// Spaces) and what carries the gate, warning, and revision metadata. Resources whose Unit
// is out of scope are dropped on the join.

import { getResources, resourceDoc } from '@confighub/examples-webkit/api';
import {
  createSnapshotContext,
  isCanonicalSpace,
  loadScopedUnits,
  type ExtendedUnit,
  type FleetScope,
  type Origin,
} from '@confighub/examples-webkit/fleet';
import {
  analyzeDefinitions,
  analyzeFleet,
  buildClusterRbac,
  type ClusterRbac,
  type FleetResource,
  type Finding,
} from '@confighub/examples-webkit/rbac';

import { scopeStore } from './scope';

const K8S_UNITS_WHERE = "ToolchainType = 'Kubernetes/YAML'";

// ServiceAccounts are not in the RBAC API group but are the subjects most bindings name,
// so the inventory covers both. A `where` is a conjunction, hence the regex rather than
// two queries.
const RBAC_RESOURCES_WHERE =
  "ResourceType ~ '^(rbac[.]authorization[.]k8s[.]io/|v1/ServiceAccount$)'";

export interface FleetSnapshot {
  /** RBAC entities per cluster (Target slug; Space slug for unbound Units). */
  clusters: Map<string, ClusterRbac>;
  /** Every parsed resource, for the explorer table. */
  resources: FleetResource[];
  /** In-scope unit metadata by UnitID (gates, warnings, revisions). */
  units: Map<string, ExtendedUnit>;
  /** RBAC entities per base/policy Space, keyed by Space slug. */
  definitions: Map<string, ClusterRbac>;
  /**
   * Hygiene findings, analyzed once and read by every page: the full analysis over each
   * cluster, plus the definition-local analysis over each base Space.
   */
  findings: Finding[];
}

async function build(scope: FleetScope): Promise<FleetSnapshot> {
  const [inventory, scoped] = await Promise.all([
    getResources({ where: RBAC_RESOURCES_WHERE }),
    loadScopedUnits(scope, { where: K8S_UNITS_WHERE }),
  ]);

  const resources: FleetResource[] = [];
  for (const entry of inventory) {
    const resource = entry.Resource;
    const unitId = resource?.UnitID;
    if (!resource || unitId === undefined) continue;
    const eu = scoped.units.get(unitId);
    if (!eu) continue; // out of scope, or not a Kubernetes/YAML Unit
    const doc = resourceDoc(entry);
    if (doc === undefined || doc === null) continue;

    const space = resource.SpaceSlug ?? eu.Space?.Slug ?? '';
    const target = eu.Target?.Slug;
    const origin: Origin = {
      cluster: target ?? space,
      target,
      space,
      spaceId: resource.SpaceID ?? eu.Unit?.SpaceID ?? '',
      unitId,
      unitSlug: resource.UnitSlug ?? eu.Unit?.Slug ?? '',
      resourceName: resource.ResourceName ?? '',
      resourceId: resource.ResourceID,
      canonical: isCanonicalSpace(eu.Space?.Labels),
    };
    resources.push({ origin, doc });
  }

  // Canonical (base/policy) definitions stay out of cluster analysis: nothing deploys
  // there, so they would produce phantom grants and cross-reference findings. They are
  // analyzed as their own groups instead, with the analyzers that judge a definition on
  // its own terms — the base Space is where an over-broad role is fixed once for the
  // whole fleet.
  const clusters = buildClusterRbac(resources.filter((r) => r.origin.canonical !== true));
  const definitions = buildClusterRbac(resources.filter((r) => r.origin.canonical === true));
  return {
    clusters,
    definitions,
    resources,
    units: scoped.units,
    findings: [...analyzeFleet(clusters), ...analyzeDefinitions(definitions)],
  };
}

export const { SnapshotProvider, useSnapshot } = createSnapshotContext<FleetSnapshot>(
  scopeStore,
  build,
);
