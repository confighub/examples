// Structural RBAC inventory materializers for FQL's `roles` and `bindings`
// tables: one row per Role/ClusterRole and per RoleBinding/ClusterRoleBinding,
// with the computed audit flags from the rbac findings analyzers (wildcards,
// aggregation, orphaned references, superuser bindings). The effective-access
// view ("who can what") is the separate `grants` table — see grants.ts.

import type { Row } from '../fql';
import { analyzeFleet } from './findings';
import { buildClusterRbac, type FleetResource } from './model';
import { effectiveRules, isBuiltinRoleName, resolveRoleRef } from './semantics';

/** Does any rule wildcard verbs, resources, or apiGroups? */
function hasWildcardRule(rules: { verbs: string[]; resources: string[]; apiGroups: string[] }[]): boolean {
  return rules.some(
    (r) => r.verbs.includes('*') || r.resources.includes('*') || r.apiGroups.includes('*'),
  );
}

/** One row per Role/ClusterRole, with structural facts and the wildcard flag. */
export function materializeRoles(resources: FleetResource[]): Row[] {
  const clusters = buildClusterRbac(resources);
  const rows: Row[] = [];
  for (const cluster of clusters.values()) {
    for (const role of cluster.roles) {
      const o = role.origin;
      const row: Row = {
        name: role.name,
        kind: role.kind, // Role | ClusterRole
        namespace: role.namespace ?? null, // null for ClusterRole
        cluster: o.cluster,
        space: o.space,
        unit: o.unitSlug,
        target: o.target ?? null,
        hasWildcard: hasWildcardRule(role.rules),
        aggregated: role.aggregationSelectors.length > 0,
        ruleCount: role.rules.length,
      };
      for (const [k, v] of Object.entries(role.labels)) row[`labels.${k}`] = v;
      rows.push(row);
    }
  }
  return rows;
}

/** One row per RoleBinding/ClusterRoleBinding, with roleRef + audit flags. */
export function materializeBindings(resources: FleetResource[]): Row[] {
  const clusters = buildClusterRbac(resources);
  const rows: Row[] = [];
  for (const cluster of clusters.values()) {
    for (const b of cluster.bindings) {
      const o = b.origin;
      const role = resolveRoleRef(b, cluster);
      // Orphaned: roleRef resolves to nothing and isn't a known builtin.
      const orphaned = !role && !isBuiltinRoleName(b.roleRef.name);
      // Superuser: bound to cluster-admin, or to a role whose effective rules
      // are *-on-*-in-* (mirrors findings.ts clusterAdminFindings).
      let clusterAdmin = b.roleRef.name === 'cluster-admin';
      if (!clusterAdmin && role) {
        clusterAdmin = effectiveRules(role, cluster).some(
          (r) => r.verbs.includes('*') && r.resources.includes('*') && r.apiGroups.includes('*'),
        );
      }
      rows.push({
        name: b.name,
        kind: b.kind, // RoleBinding | ClusterRoleBinding
        namespace: b.namespace ?? null, // null for ClusterRoleBinding
        cluster: o.cluster,
        space: o.space,
        unit: o.unitSlug,
        target: o.target ?? null,
        roleRef: b.roleRef.name,
        roleRefKind: b.roleRef.kind, // Role | ClusterRole
        subjectCount: b.subjects.length,
        orphaned,
        clusterAdmin,
      });
    }
  }
  return rows;
}

/** One row per RBAC hygiene finding (analyzeFleet): the audit complement that
 *  covers what the boolean flags on roles/bindings don't — escalation verbs,
 *  risky grants (secrets/exec/webhooks), unbound ServiceAccounts, etc. */
export function materializeFindings(resources: FleetResource[]): Row[] {
  const clusters = buildClusterRbac(resources);
  return analyzeFleet(clusters).map((f) => ({
    analyzer: f.analyzer,
    severity: f.severity, // high | medium | low
    cluster: f.cluster,
    space: f.origin.space,
    unit: f.origin.unitSlug,
    target: f.origin.target ?? null,
    resourceKind: f.resourceKind,
    resourceName: f.resourceName,
    namespace: f.namespace ?? null,
    message: f.message,
    __id: f.id, // stable finding id, for de-dup (excluded from SELECT *)
  }));
}
