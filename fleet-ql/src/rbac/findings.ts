// RBAC hygiene analyzers, mirroring the published fleet-audit findings:
// wildcards, dangerous verbs, risky grants, cluster-admin bindings, orphaned
// bindings, and unbound ServiceAccounts. Pure functions over the snapshot;
// enforcement (gates) is server-side via Triggers — these are the
// analysis-only complement.

import { ClusterRbac, ResourceOrigin, subjectKey } from './model';
import { effectiveRules, isBuiltinRoleName, resolveRoleRef } from './semantics';

export type Severity = 'high' | 'medium' | 'low';

export interface Finding {
  /** Stable id for list rendering and dedup. */
  id: string;
  analyzer: string;
  severity: Severity;
  cluster: string;
  origin: ResourceOrigin;
  resourceKind: string;
  resourceName: string;
  namespace?: string;
  message: string;
}

const ESCALATION_VERBS = new Set(['escalate', 'bind', 'impersonate']);

function finding(
  analyzer: string,
  severity: Severity,
  origin: ResourceOrigin,
  resourceKind: string,
  resourceName: string,
  message: string,
  namespace?: string,
): Finding {
  return {
    id: `${analyzer}:${origin.cluster}:${resourceKind}:${namespace ?? ''}:${resourceName}`,
    analyzer,
    severity,
    cluster: origin.cluster,
    origin,
    resourceKind,
    resourceName,
    namespace,
    message,
  };
}

function wildcardFindings(cluster: ClusterRbac): Finding[] {
  const out: Finding[] = [];
  for (const role of cluster.roles) {
    const parts: string[] = [];
    role.rules.forEach((rule, i) => {
      if (rule.verbs.includes('*')) parts.push(`rule ${i}: wildcard verbs`);
      if (rule.resources.includes('*')) parts.push(`rule ${i}: wildcard resources`);
      if (rule.apiGroups.includes('*')) parts.push(`rule ${i}: wildcard apiGroups`);
    });
    if (parts.length > 0) {
      out.push(
        finding(
          'wildcard-rules',
          'high',
          role.origin,
          role.kind,
          role.name,
          `Wildcard permissions (${parts.join('; ')}). Enumerate the specific verbs/resources needed.`,
          role.namespace,
        ),
      );
    }
  }
  return out;
}

function escalationFindings(cluster: ClusterRbac): Finding[] {
  const out: Finding[] = [];
  for (const role of cluster.roles) {
    const verbs = new Set(
      role.rules.flatMap((r) => r.verbs.filter((v) => ESCALATION_VERBS.has(v))),
    );
    if (verbs.size > 0) {
      out.push(
        finding(
          'privilege-escalation-verbs',
          'high',
          role.origin,
          role.kind,
          role.name,
          `Grants privilege-escalation verb(s): ${[...verbs].join(', ')}.`,
          role.namespace,
        ),
      );
    }
  }
  return out;
}

/** Grants that aren't wildcards but deserve eyes: secrets, exec, webhooks, CRDs. */
function riskyGrantFindings(cluster: ClusterRbac): Finding[] {
  const out: Finding[] = [];
  const WRITE_VERBS = ['create', 'update', 'patch', 'delete', 'deletecollection', '*'];
  for (const role of cluster.roles) {
    const risks: string[] = [];
    for (const rule of role.rules) {
      const writes = rule.verbs.some((v) => WRITE_VERBS.includes(v));
      if (rule.resources.some((r) => r === 'secrets')) {
        risks.push(writes ? 'secrets write' : 'secrets read');
      }
      if (rule.resources.some((r) => r === 'pods/exec' || r === 'pods/attach')) {
        risks.push('pod exec/attach');
      }
      if (
        writes &&
        rule.apiGroups.some((g) => g === 'admissionregistration.k8s.io' || g === 'apiextensions.k8s.io')
      ) {
        risks.push('webhook/CRD write');
      }
    }
    if (risks.length > 0) {
      out.push(
        finding(
          'risky-grants',
          'medium',
          role.origin,
          role.kind,
          role.name,
          `Sensitive access: ${[...new Set(risks)].join(', ')}.`,
          role.namespace,
        ),
      );
    }
  }
  return out;
}

function clusterAdminFindings(cluster: ClusterRbac): Finding[] {
  const out: Finding[] = [];
  for (const binding of cluster.bindings) {
    let isSuperuser = binding.roleRef.name === 'cluster-admin';
    if (!isSuperuser) {
      const role = resolveRoleRef(binding, cluster);
      if (role) {
        isSuperuser = effectiveRules(role, cluster).some(
          (r) => r.verbs.includes('*') && r.resources.includes('*') && r.apiGroups.includes('*'),
        );
      }
    }
    if (isSuperuser) {
      const subjects = binding.subjects.map(subjectKey).join(', ') || '(no subjects)';
      out.push(
        finding(
          'cluster-admin-bindings',
          binding.kind === 'ClusterRoleBinding' ? 'high' : 'medium',
          binding.origin,
          binding.kind,
          binding.name,
          `Grants superuser (${binding.roleRef.name}) to: ${subjects}.`,
          binding.namespace,
        ),
      );
    }
  }
  return out;
}

function orphanedBindingFindings(cluster: ClusterRbac): Finding[] {
  const out: Finding[] = [];
  for (const binding of cluster.bindings) {
    if (resolveRoleRef(binding, cluster)) continue;
    if (isBuiltinRoleName(binding.roleRef.name)) continue;
    out.push(
      finding(
        'orphaned-bindings',
        'medium',
        binding.origin,
        binding.kind,
        binding.name,
        `References ${binding.roleRef.kind} "${binding.roleRef.name}", which does not exist on this cluster. Remove the binding or restore the role.`,
        binding.namespace,
      ),
    );
  }
  return out;
}

function unboundServiceAccountFindings(cluster: ClusterRbac): Finding[] {
  const bound = new Set<string>();
  for (const binding of cluster.bindings) {
    for (const s of binding.subjects) {
      if (s.kind === 'ServiceAccount') bound.add(`${s.namespace ?? ''}/${s.name}`);
    }
  }
  const out: Finding[] = [];
  for (const sa of cluster.serviceAccounts) {
    if (bound.has(`${sa.namespace}/${sa.name}`)) continue;
    out.push(
      finding(
        'unbound-service-accounts',
        'low',
        sa.origin,
        'ServiceAccount',
        sa.name,
        `ServiceAccount has no role bindings in this snapshot — possibly unused.`,
        sa.namespace,
      ),
    );
  }
  return out;
}

const ANALYZERS: ((cluster: ClusterRbac) => Finding[])[] = [
  wildcardFindings,
  escalationFindings,
  riskyGrantFindings,
  clusterAdminFindings,
  orphanedBindingFindings,
  unboundServiceAccountFindings,
];

/** Run every analyzer over every cluster. */
export function analyzeFleet(clusters: Map<string, ClusterRbac>): Finding[] {
  const out: Finding[] = [];
  for (const cluster of clusters.values()) {
    for (const analyzer of ANALYZERS) {
      out.push(...analyzer(cluster));
    }
  }
  const severityOrder: Record<Severity, number> = { high: 0, medium: 1, low: 2 };
  return out.sort(
    (a, b) =>
      severityOrder[a.severity] - severityOrder[b.severity] ||
      a.cluster.localeCompare(b.cluster) ||
      a.id.localeCompare(b.id),
  );
}
