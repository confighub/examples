// Shared test fixtures: builds ClusterRbac snapshots from plain JS docs the
// way the snapshot loader does (parsed JSON resource bodies).

import { FleetResource, buildClusterRbac, ClusterRbac } from './model';

let counter = 0;

export function res(cluster: string, doc: unknown): FleetResource {
  counter += 1;
  return {
    origin: {
      cluster,
      space: cluster,
      spaceId: `space-${cluster}`,
      unitId: `unit-${counter}`,
      unitSlug: `unit-${counter}`,
      resourceName: `r-${counter}`,
    },
    doc,
  };
}

export function clusterRole(
  name: string,
  rules: unknown[],
  extra?: { labels?: Record<string, string>; aggregationRule?: unknown },
): unknown {
  return {
    apiVersion: 'rbac.authorization.k8s.io/v1',
    kind: 'ClusterRole',
    metadata: { name, labels: extra?.labels },
    rules,
    aggregationRule: extra?.aggregationRule,
  };
}

export function role(name: string, namespace: string, rules: unknown[]): unknown {
  return {
    apiVersion: 'rbac.authorization.k8s.io/v1',
    kind: 'Role',
    metadata: { name, namespace },
    rules,
  };
}

export function clusterRoleBinding(name: string, roleName: string, subjects: unknown[]): unknown {
  return {
    apiVersion: 'rbac.authorization.k8s.io/v1',
    kind: 'ClusterRoleBinding',
    metadata: { name },
    roleRef: { apiGroup: 'rbac.authorization.k8s.io', kind: 'ClusterRole', name: roleName },
    subjects,
  };
}

export function roleBinding(
  name: string,
  namespace: string,
  roleRef: { kind: string; name: string },
  subjects: unknown[],
): unknown {
  return {
    apiVersion: 'rbac.authorization.k8s.io/v1',
    kind: 'RoleBinding',
    metadata: { name, namespace },
    roleRef: { apiGroup: 'rbac.authorization.k8s.io', ...roleRef },
    subjects,
  };
}

export function serviceAccount(name: string, namespace: string): unknown {
  return { apiVersion: 'v1', kind: 'ServiceAccount', metadata: { name, namespace } };
}

export function group(name: string): unknown {
  return { kind: 'Group', name, apiGroup: 'rbac.authorization.k8s.io' };
}

export function user(name: string): unknown {
  return { kind: 'User', name, apiGroup: 'rbac.authorization.k8s.io' };
}

export function sa(name: string, namespace: string): unknown {
  return { kind: 'ServiceAccount', name, namespace };
}

export function build(resources: FleetResource[]): Map<string, ClusterRbac> {
  return buildClusterRbac(resources);
}
