// The `grants` materializer: turn parsed RBAC resources into effective-access
// rows for FQL's `grants` virtual table. One row per (cluster, binding, subject)
// that the access query permits — "who can VERB RESOURCE, on which cluster,
// granted by what." This is the FQL-specific glue over the vendored rbac engine;
// the heavy lifting (role resolution, aggregation, wildcard matching) is in
// semantics.ts / model.ts.

import type { Row } from '../fql';
import { buildClusterRbac, type FleetResource, subjectKey } from './model';
import {
  accessMatches,
  bindingScopeMatches,
  effectiveRules,
  isBuiltinRoleName,
  resolveRoleRef,
} from './semantics';

/** The access question, extracted from the FQL WHERE. Any field omitted is a
 *  wildcard. With none set, every (binding × subject) grant is returned. */
export interface GrantQuery {
  verb?: string;
  resource?: string;
  apiGroup?: string;
  /** Restrict to grants effective in this namespace (RoleBindings honor scope). */
  namespace?: string;
  /** Honor resourceNames restrictions against a specific object name. */
  name?: string;
}

/** Build grant rows from RBAC resources, filtered by the access query. */
export function materializeGrants(resources: FleetResource[], q: GrantQuery = {}): Row[] {
  const clusters = buildClusterRbac(resources);
  // Whether the WHERE asked an access question at all (verb/resource/group/name).
  // `namespace` alone only scopes bindings; it doesn't make it an access test.
  const isAccessTest =
    q.verb !== undefined ||
    q.resource !== undefined ||
    q.apiGroup !== undefined ||
    q.name !== undefined;

  const rows: Row[] = [];
  for (const cluster of clusters.values()) {
    for (const binding of cluster.bindings) {
      if (q.namespace !== undefined && !bindingScopeMatches(binding, q.namespace)) continue;

      const role = resolveRoleRef(binding, cluster);
      let viaBuiltin = false;

      if (isAccessTest) {
        let matches = false;
        if (role) {
          matches = effectiveRules(role, cluster).some((rule) => accessMatches(rule, q));
        } else if (isBuiltinRoleName(binding.roleRef.name)) {
          // We have no manifest for builtins; only cluster-admin is known to
          // grant everything, so only it matches an arbitrary access query.
          matches = binding.roleRef.name === 'cluster-admin';
          viaBuiltin = matches;
        }
        if (!matches) continue;
      } else {
        viaBuiltin = !role && isBuiltinRoleName(binding.roleRef.name);
      }

      const o = binding.origin;
      const scope = binding.kind === 'RoleBinding' ? (binding.namespace ?? null) : null;
      for (const s of binding.subjects) {
        rows.push({
          subject: subjectKey(s),
          subjectKind: s.kind,
          subjectName: s.name,
          cluster: o.cluster,
          space: o.space,
          unit: o.unitSlug,
          target: o.target ?? null,
          scope,
          role: binding.roleRef.name,
          viaBuiltin,
          binding: binding.name,
        });
      }
    }
  }
  return rows;
}
