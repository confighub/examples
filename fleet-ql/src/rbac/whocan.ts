// Fleet-wide effective-access queries: "who can VERB RESOURCE, on which
// cluster, granted by what?" and the inverse subject view.

import { BindingEntity, ClusterRbac, RoleEntity, Subject, subjectKey } from './model';
import {
  AccessQuery,
  bindingScopeMatches,
  effectiveRules,
  isBuiltinRoleName,
  resolveRoleRef,
  ruleMatches,
} from './semantics';

/** One subject's access to the queried resource, with full provenance. */
export interface Grant {
  cluster: string;
  subject: Subject;
  subjectKey: string;
  /** Resolved role, when present in the snapshot. */
  role?: RoleEntity;
  roleRefName: string;
  /** True when the roleRef is a Kubernetes builtin not stored in ConfigHub. */
  viaBuiltinRole: boolean;
  binding: BindingEntity;
  /** Namespace the grant is effective in; undefined = cluster-wide. */
  scope?: string;
}

/**
 * cluster-admin grants everything; for other builtins we have no manifest,
 * so only cluster-admin is treated as matching arbitrary queries. Bindings
 * to admin/edit/view/system:* surface in audits but not in who-can results.
 */
function builtinMatches(roleRefName: string): boolean {
  return roleRefName === 'cluster-admin';
}

/** All grants matching the query in one cluster. */
export function whoCanInCluster(cluster: ClusterRbac, query: AccessQuery): Grant[] {
  const grants: Grant[] = [];
  for (const binding of cluster.bindings) {
    if (!bindingScopeMatches(binding, query.namespace)) continue;

    const role = resolveRoleRef(binding, cluster);
    let matches = false;
    let viaBuiltin = false;
    if (role) {
      matches = effectiveRules(role, cluster).some((rule) => ruleMatches(rule, query));
    } else if (isBuiltinRoleName(binding.roleRef.name)) {
      matches = builtinMatches(binding.roleRef.name);
      viaBuiltin = matches;
    }
    if (!matches) continue;

    for (const subject of binding.subjects) {
      grants.push({
        cluster: cluster.cluster,
        subject,
        subjectKey: subjectKey(subject),
        role,
        roleRefName: binding.roleRef.name,
        viaBuiltinRole: viaBuiltin,
        binding,
        scope: binding.kind === 'RoleBinding' ? binding.namespace : undefined,
      });
    }
  }
  return grants;
}

/** All grants matching the query across the fleet. */
export function whoCan(clusters: Map<string, ClusterRbac>, query: AccessQuery): Grant[] {
  const grants: Grant[] = [];
  for (const cluster of clusters.values()) {
    grants.push(...whoCanInCluster(cluster, query));
  }
  return grants;
}

/** One role held by a subject in one cluster. */
export interface SubjectGrant {
  cluster: string;
  binding: BindingEntity;
  role?: RoleEntity;
  roleRefName: string;
  scope?: string;
}

/** The inverse view: every role a subject holds, fleet-wide. */
export function subjectAccess(
  clusters: Map<string, ClusterRbac>,
  subject: { kind: string; name: string; namespace?: string },
): SubjectGrant[] {
  const out: SubjectGrant[] = [];
  for (const cluster of clusters.values()) {
    for (const binding of cluster.bindings) {
      const held = binding.subjects.some(
        (s) =>
          s.kind === subject.kind &&
          s.name === subject.name &&
          (subject.kind !== 'ServiceAccount' || s.namespace === subject.namespace),
      );
      if (!held) continue;
      out.push({
        cluster: cluster.cluster,
        binding,
        role: resolveRoleRef(binding, cluster),
        roleRefName: binding.roleRef.name,
        scope: binding.kind === 'RoleBinding' ? binding.namespace : undefined,
      });
    }
  }
  return out;
}

/** Every distinct subject appearing in any binding, for autocomplete. */
export function allSubjects(clusters: Map<string, ClusterRbac>): Subject[] {
  const byKey = new Map<string, Subject>();
  for (const cluster of clusters.values()) {
    for (const binding of cluster.bindings) {
      for (const s of binding.subjects) {
        byKey.set(subjectKey(s), s);
      }
    }
  }
  return [...byKey.values()].sort((a, b) => subjectKey(a).localeCompare(subjectKey(b)));
}
