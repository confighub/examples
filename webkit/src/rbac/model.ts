// Domain model for Kubernetes RBAC resources drawn from ConfigHub Units.
// Parsing is lenient: malformed documents are skipped, never thrown on —
// a bad resource in one Unit must not take down fleet-wide analysis.

/**
 * Where a resource came from in ConfigHub. Clusters are Targets: a Unit's
 * Target identifies the cluster it deploys to, and Units from many Spaces
 * can share one cluster Target. `cluster` is the Target slug when the Unit
 * is bound, falling back to the Space slug for unbound ("paper cluster")
 * Units; `target` is set only when actually bound.
 */
export interface ResourceOrigin {
  cluster: string;
  target?: string;
  space: string;
  spaceId: string;
  unitId: string;
  unitSlug: string;
  resourceName: string;
  /**
   * True for canonical definitions (base/policy Spaces) that aren't deployed
   * anywhere — shown in the explorer but excluded from cluster analysis
   * (who-can, findings).
   */
  canonical?: boolean;
}

export interface PolicyRule {
  apiGroups: string[];
  resources: string[];
  verbs: string[];
  resourceNames: string[];
  nonResourceURLs: string[];
}

export interface RoleEntity {
  kind: 'Role' | 'ClusterRole';
  name: string;
  /** Set for namespaced Roles only. */
  namespace?: string;
  labels: Record<string, string>;
  rules: PolicyRule[];
  /** ClusterRole aggregationRule.clusterRoleSelectors matchLabels, if any. */
  aggregationSelectors: Record<string, string>[];
  origin: ResourceOrigin;
}

export interface Subject {
  kind: string; // User | Group | ServiceAccount
  name: string;
  namespace?: string;
}

export interface BindingEntity {
  kind: 'RoleBinding' | 'ClusterRoleBinding';
  name: string;
  /** Set for RoleBindings only. */
  namespace?: string;
  roleRef: { kind: string; name: string };
  subjects: Subject[];
  origin: ResourceOrigin;
}

export interface ServiceAccountEntity {
  name: string;
  namespace: string;
  origin: ResourceOrigin;
}

/** All RBAC entities of one cluster (one ConfigHub Space). */
export interface ClusterRbac {
  cluster: string;
  roles: RoleEntity[];
  bindings: BindingEntity[];
  serviceAccounts: ServiceAccountEntity[];
}

/** A parsed resource document plus its ConfigHub origin. */
export interface FleetResource {
  origin: ResourceOrigin;
  doc: unknown;
}

const RBAC_GROUP = 'rbac.authorization.k8s.io';

function asRecord(v: unknown): Record<string, unknown> | undefined {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
    ? (v as Record<string, unknown>)
    : undefined;
}

function asString(v: unknown): string | undefined {
  return typeof v === 'string' ? v : undefined;
}

function asStringArray(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}

function asStringMap(v: unknown): Record<string, string> {
  const rec = asRecord(v);
  if (!rec) return {};
  const out: Record<string, string> = {};
  for (const [k, val] of Object.entries(rec)) {
    if (typeof val === 'string') out[k] = val;
  }
  return out;
}

function parseRules(v: unknown): PolicyRule[] {
  if (!Array.isArray(v)) return [];
  const rules: PolicyRule[] = [];
  for (const item of v) {
    const rec = asRecord(item);
    if (!rec) continue;
    rules.push({
      apiGroups: asStringArray(rec.apiGroups),
      resources: asStringArray(rec.resources),
      verbs: asStringArray(rec.verbs),
      resourceNames: asStringArray(rec.resourceNames),
      nonResourceURLs: asStringArray(rec.nonResourceURLs),
    });
  }
  return rules;
}

function parseSubjects(v: unknown): Subject[] {
  if (!Array.isArray(v)) return [];
  const subjects: Subject[] = [];
  for (const item of v) {
    const rec = asRecord(item);
    const kind = asString(rec?.kind);
    const name = asString(rec?.name);
    if (!rec || kind === undefined || name === undefined) continue;
    subjects.push({ kind, name, namespace: asString(rec.namespace) });
  }
  return subjects;
}

function parseAggregationSelectors(v: unknown): Record<string, string>[] {
  const rec = asRecord(v);
  const selectors = rec?.clusterRoleSelectors;
  if (!Array.isArray(selectors)) return [];
  const out: Record<string, string>[] = [];
  for (const sel of selectors) {
    const matchLabels = asStringMap(asRecord(sel)?.matchLabels);
    if (Object.keys(matchLabels).length > 0) out.push(matchLabels);
  }
  return out;
}

/**
 * Index parsed fleet resources into per-cluster RBAC entity sets. Non-RBAC
 * resources (other than ServiceAccounts) and unparseable docs are ignored.
 */
export function buildClusterRbac(resources: FleetResource[]): Map<string, ClusterRbac> {
  const clusters = new Map<string, ClusterRbac>();

  const forCluster = (name: string): ClusterRbac => {
    let c = clusters.get(name);
    if (!c) {
      c = { cluster: name, roles: [], bindings: [], serviceAccounts: [] };
      clusters.set(name, c);
    }
    return c;
  };

  for (const { origin, doc } of resources) {
    const rec = asRecord(doc);
    if (!rec) continue;
    const kind = asString(rec.kind);
    const apiVersion = asString(rec.apiVersion) ?? '';
    const metadata = asRecord(rec.metadata);
    const name = asString(metadata?.name);
    if (kind === undefined || name === undefined) continue;
    const namespace = asString(metadata?.namespace);
    const labels = asStringMap(metadata?.labels);
    const cluster = forCluster(origin.cluster);

    if (kind === 'ServiceAccount' && apiVersion === 'v1') {
      cluster.serviceAccounts.push({ name, namespace: namespace ?? 'default', origin });
      continue;
    }
    if (!apiVersion.startsWith(`${RBAC_GROUP}/`)) continue;

    if (kind === 'Role' || kind === 'ClusterRole') {
      cluster.roles.push({
        kind,
        name,
        namespace: kind === 'Role' ? namespace : undefined,
        labels,
        rules: parseRules(rec.rules),
        aggregationSelectors:
          kind === 'ClusterRole' ? parseAggregationSelectors(rec.aggregationRule) : [],
        origin,
      });
    } else if (kind === 'RoleBinding' || kind === 'ClusterRoleBinding') {
      const roleRef = asRecord(rec.roleRef);
      const refKind = asString(roleRef?.kind);
      const refName = asString(roleRef?.name);
      if (refKind === undefined || refName === undefined) continue;
      cluster.bindings.push({
        kind,
        name,
        namespace: kind === 'RoleBinding' ? namespace : undefined,
        roleRef: { kind: refKind, name: refName },
        subjects: parseSubjects(rec.subjects),
        origin,
      });
    }
  }

  return clusters;
}

/** Stable display form of a subject: `kind:name` or `kind:ns/name`. */
export function subjectKey(s: Subject): string {
  return s.namespace !== undefined
    ? `${s.kind}:${s.namespace}/${s.name}`
    : `${s.kind}:${s.name}`;
}
