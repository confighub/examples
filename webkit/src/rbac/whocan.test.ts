import { describe, expect, it } from 'vitest';

import {
  build,
  clusterRole,
  clusterRoleBinding,
  group,
  res,
  role,
  roleBinding,
  sa,
  user,
} from './fixtures';
import { allSubjects, subjectAccess, whoCan } from './whocan';

const secretReader = clusterRole('secret-reader', [
  { apiGroups: [''], resources: ['secrets'], verbs: ['get', 'list'] },
]);

describe('whoCan', () => {
  it('finds grants across multiple clusters with provenance', () => {
    const clusters = build([
      res('prod-cluster', secretReader),
      res('prod-cluster', clusterRoleBinding('ops-secrets', 'secret-reader', [group('ops')])),
      res('dev-cluster', secretReader),
      res('dev-cluster', clusterRoleBinding('dev-secrets', 'secret-reader', [group('devs')])),
    ]);
    const grants = whoCan(clusters, { verb: 'get', resource: 'secrets', apiGroup: '' });
    expect(grants).toHaveLength(2);
    const byCluster = Object.fromEntries(grants.map((g) => [g.cluster, g]));
    expect(byCluster['prod-cluster'].subjectKey).toBe('Group:ops');
    expect(byCluster['prod-cluster'].binding.name).toBe('ops-secrets');
    expect(byCluster['dev-cluster'].subjectKey).toBe('Group:devs');
  });

  it('scopes RoleBinding grants to their namespace', () => {
    const clusters = build([
      res('c1', role('ns-reader', 'payments', [
        { apiGroups: [''], resources: ['secrets'], verbs: ['get'] },
      ])),
      res('c1', roleBinding('rb', 'payments', { kind: 'Role', name: 'ns-reader' }, [user('alice')])),
    ]);
    expect(
      whoCan(clusters, { verb: 'get', resource: 'secrets', namespace: 'payments' }),
    ).toHaveLength(1);
    expect(
      whoCan(clusters, { verb: 'get', resource: 'secrets', namespace: 'other' }),
    ).toHaveLength(0);
    // Unscoped query includes namespaced grants, marked with their scope.
    const anywhere = whoCan(clusters, { verb: 'get', resource: 'secrets' });
    expect(anywhere).toHaveLength(1);
    expect(anywhere[0].scope).toBe('payments');
  });

  it('credits cluster-admin builtin bindings with any access', () => {
    const clusters = build([
      res('c1', clusterRoleBinding('breakglass', 'cluster-admin', [group('oncall')])),
    ]);
    const grants = whoCan(clusters, { verb: 'delete', resource: 'secrets', apiGroup: '' });
    expect(grants).toHaveLength(1);
    expect(grants[0].viaBuiltinRole).toBe(true);
    expect(grants[0].roleRefName).toBe('cluster-admin');
  });

  it('does not invent access for unknown builtin roles like view', () => {
    const clusters = build([
      res('c1', clusterRoleBinding('viewers', 'view', [group('everyone')])),
    ]);
    expect(whoCan(clusters, { verb: 'get', resource: 'secrets' })).toHaveLength(0);
  });

  it('answers through aggregated roles', () => {
    const clusters = build([
      res('c1', clusterRole('agg', [], {
        aggregationRule: { clusterRoleSelectors: [{ matchLabels: { part: 'yes' } }] },
      })),
      res('c1', clusterRole('part', [
        { apiGroups: [''], resources: ['secrets'], verbs: ['get'] },
      ], { labels: { part: 'yes' } })),
      res('c1', clusterRoleBinding('crb', 'agg', [user('root')])),
    ]);
    expect(whoCan(clusters, { verb: 'get', resource: 'secrets' })).toHaveLength(1);
  });
});

describe('subjectAccess', () => {
  it('lists every role a subject holds across the fleet', () => {
    const clusters = build([
      res('prod-cluster', secretReader),
      res('prod-cluster', clusterRoleBinding('b1', 'secret-reader', [group('ops'), user('alice')])),
      res('dev-cluster', clusterRoleBinding('b2', 'cluster-admin', [user('alice')])),
    ]);
    const grants = subjectAccess(clusters, { kind: 'User', name: 'alice' });
    expect(grants.map((g) => `${g.cluster}/${g.roleRefName}`).sort()).toEqual([
      'dev-cluster/cluster-admin',
      'prod-cluster/secret-reader',
    ]);
  });

  it('distinguishes ServiceAccounts by namespace', () => {
    const clusters = build([
      res('c1', roleBinding('rb', 'apps', { kind: 'ClusterRole', name: 'edit' }, [
        sa('ci-deployer', 'apps'),
      ])),
    ]);
    expect(
      subjectAccess(clusters, { kind: 'ServiceAccount', name: 'ci-deployer', namespace: 'apps' }),
    ).toHaveLength(1);
    expect(
      subjectAccess(clusters, { kind: 'ServiceAccount', name: 'ci-deployer', namespace: 'other' }),
    ).toHaveLength(0);
  });
});

describe('allSubjects', () => {
  it('dedupes subjects across bindings and clusters', () => {
    const clusters = build([
      res('c1', clusterRoleBinding('b1', 'view', [group('devs'), user('alice')])),
      res('c2', clusterRoleBinding('b2', 'view', [group('devs')])),
    ]);
    expect(allSubjects(clusters).map((s) => s.name).sort()).toEqual(['alice', 'devs']);
  });
});
