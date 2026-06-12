// Kubernetes RBAC matching semantics, implemented over the model in model.ts.
// Mirrors the API server's authorization rules: wildcard verbs/resources/
// apiGroups, resource/subresource forms, resourceNames, nonResourceURLs,
// ClusterRole aggregation, and binding scope resolution.

import {
  BindingEntity,
  ClusterRbac,
  PolicyRule,
  RoleEntity,
} from './model';

/** A resource-access question: "who can VERB RESOURCE [in NAMESPACE] [named NAME]?" */
export interface AccessQuery {
  verb: string;
  /** Plural resource, optionally with subresource: "pods", "pods/log". */
  resource: string;
  /** API group; "" (core) when omitted. */
  apiGroup?: string;
  /** Restrict to grants effective in this namespace. Omit for "anywhere". */
  namespace?: string;
  /** Specific object name, to honor resourceNames restrictions. */
  name?: string;
}

function verbMatches(ruleVerbs: string[], verb: string): boolean {
  return ruleVerbs.some((v) => v === '*' || v === verb);
}

function groupMatches(ruleGroups: string[], group: string): boolean {
  return ruleGroups.some((g) => g === '*' || g === group);
}

/**
 * Match a rule's resource entry against a queried resource. Entries and
 * queries may carry a subresource ("pods/log"); each slash-separated segment
 * matches exactly or via "*" ("*\/scale" matches "deployments/scale"). A
 * bare entry never matches a subresource query and vice versa.
 */
function resourceEntryMatches(entry: string, resource: string): boolean {
  if (entry === '*') return true;
  const entrySegs = entry.split('/');
  const querySegs = resource.split('/');
  if (entrySegs.length !== querySegs.length) return false;
  return entrySegs.every((seg, i) => seg === '*' || seg === querySegs[i]);
}

function resourceMatches(ruleResources: string[], resource: string): boolean {
  return ruleResources.some((entry) => resourceEntryMatches(entry, resource));
}

/**
 * resourceNames restrict a rule to specific objects. An empty list means all
 * names. When the query names no object, restricted rules still "can" reach
 * some object, so they match — callers that need strict per-object answers
 * must pass query.name.
 */
function nameMatches(ruleNames: string[], name: string | undefined): boolean {
  if (ruleNames.length === 0) return true;
  if (name === undefined) return true;
  return ruleNames.includes(name);
}

/** Does a single policy rule grant the queried access? */
export function ruleMatches(rule: PolicyRule, query: AccessQuery): boolean {
  if (rule.resources.length === 0) return false; // nonResourceURL-only rule
  return (
    verbMatches(rule.verbs, query.verb) &&
    groupMatches(rule.apiGroups, query.apiGroup ?? '') &&
    resourceMatches(rule.resources, query.resource) &&
    nameMatches(rule.resourceNames, query.name)
  );
}

/** Non-resource URL matching: exact, or prefix via a trailing "*". */
export function nonResourceUrlMatches(rule: PolicyRule, verb: string, url: string): boolean {
  if (!verbMatches(rule.verbs, verb)) return false;
  return rule.nonResourceURLs.some((entry) => {
    if (entry === '*') return true;
    if (entry.endsWith('*')) return url.startsWith(entry.slice(0, -1));
    return entry === url;
  });
}

function labelsMatch(labels: Record<string, string>, selector: Record<string, string>): boolean {
  return Object.entries(selector).every(([k, v]) => labels[k] === v);
}

/**
 * Effective rules of a role. For aggregated ClusterRoles, the API server's
 * controller unions the rules of every ClusterRole matching any selector;
 * aggregated roles can themselves aggregate, so iterate to a fixed point.
 */
export function effectiveRules(role: RoleEntity, cluster: ClusterRbac): PolicyRule[] {
  if (role.aggregationSelectors.length === 0) return role.rules;

  const collected = new Map<RoleEntity, true>();
  const pending: RoleEntity[] = [role];
  while (pending.length > 0) {
    const current = pending.pop()!;
    if (collected.has(current)) continue;
    collected.set(current, true);
    if (current.aggregationSelectors.length === 0) continue;
    for (const candidate of cluster.roles) {
      if (candidate.kind !== 'ClusterRole' || collected.has(candidate)) continue;
      if (current.aggregationSelectors.some((sel) => labelsMatch(candidate.labels, sel))) {
        pending.push(candidate);
      }
    }
  }

  const rules: PolicyRule[] = [];
  for (const r of collected.keys()) rules.push(...r.rules);
  return rules;
}

/**
 * Kubernetes ships these ClusterRoles in every cluster; they exist even when
 * not stored in ConfigHub, so bindings referencing them are not orphans, and
 * cluster-admin's rules are known without a manifest.
 */
export const BUILTIN_CLUSTER_ROLES = new Set(['cluster-admin', 'admin', 'edit', 'view']);

export function isBuiltinRoleName(name: string): boolean {
  return BUILTIN_CLUSTER_ROLES.has(name) || name.startsWith('system:');
}

/**
 * Resolve a binding's roleRef within its cluster. RoleBindings may reference
 * a Role in their own namespace or any ClusterRole; ClusterRoleBindings
 * reference ClusterRoles only. Returns undefined when the role is not in the
 * snapshot (possibly a builtin — see isBuiltinRoleName).
 */
export function resolveRoleRef(
  binding: BindingEntity,
  cluster: ClusterRbac,
): RoleEntity | undefined {
  const { kind, name } = binding.roleRef;
  if (kind === 'ClusterRole') {
    return cluster.roles.find((r) => r.kind === 'ClusterRole' && r.name === name);
  }
  if (kind === 'Role' && binding.kind === 'RoleBinding') {
    return cluster.roles.find(
      (r) => r.kind === 'Role' && r.name === name && r.namespace === binding.namespace,
    );
  }
  return undefined;
}

/**
 * Does this binding's grant apply in the queried namespace? A
 * ClusterRoleBinding applies everywhere; a RoleBinding only within its own
 * namespace. An unspecified query namespace means "anywhere".
 */
export function bindingScopeMatches(binding: BindingEntity, namespace?: string): boolean {
  if (binding.kind === 'ClusterRoleBinding') return true;
  if (namespace === undefined) return true;
  return binding.namespace === namespace;
}
