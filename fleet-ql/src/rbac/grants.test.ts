import { describe, expect, it } from 'vitest';

import {
  clusterRole,
  clusterRoleBinding,
  group,
  res,
  role,
  roleBinding,
  sa,
  user,
} from './fixtures';
import type { Row } from '../fql';
import { materializeGrants } from './grants';
import type { FleetResource } from './model';

// A two-cluster fleet:
//  prod: ClusterRole editor (get/list/delete pods) bound to alice + devs;
//        plus a cluster-admin binding to root (builtin).
//  dev:  ClusterRole viewer (get/list pods) bound to alice;
//        plus a namespaced RoleBinding in 'web' to web-admin (* on secrets) → robot SA.
const FLEET: FleetResource[] = [
  res('prod', clusterRole('editor', [{ apiGroups: [''], resources: ['pods'], verbs: ['get', 'list', 'delete'] }])),
  res('prod', clusterRoleBinding('editor-binding', 'editor', [user('alice'), group('devs')])),
  res('prod', clusterRoleBinding('root-admin', 'cluster-admin', [user('root')])),
  res('dev', clusterRole('viewer', [{ apiGroups: [''], resources: ['pods'], verbs: ['get', 'list'] }])),
  res('dev', clusterRoleBinding('viewer-binding', 'viewer', [user('alice')])),
  res('dev', role('web-admin', 'web', [{ apiGroups: [''], resources: ['secrets'], verbs: ['*'] }])),
  res('dev', roleBinding('web-admin-binding', 'web', { kind: 'Role', name: 'web-admin' }, [sa('robot', 'web')])),
];

const subjectsOf = (rows: Row[]) => rows.map((r) => `${r.subject}@${r.cluster}`).sort();

describe('materializeGrants — who can do what, on which cluster', () => {
  it('who can delete pods: editor (incl. group) + cluster-admin, not viewer', () => {
    const rows = materializeGrants(FLEET, { verb: 'delete', resource: 'pods' });
    expect(subjectsOf(rows)).toEqual(['Group:devs@prod', 'User:alice@prod', 'User:root@prod']);
  });

  it('who can get secrets: wildcard Role + cluster-admin', () => {
    const rows = materializeGrants(FLEET, { verb: 'get', resource: 'secrets' });
    expect(subjectsOf(rows)).toEqual(['ServiceAccount:web/robot@dev', 'User:root@prod']);
  });

  it('namespace scoping excludes a RoleBinding outside the queried namespace', () => {
    // secrets in 'db': the web RoleBinding doesn't apply; only the cluster-wide
    // cluster-admin reaches it.
    const rows = materializeGrants(FLEET, { verb: 'get', resource: 'secrets', namespace: 'db' });
    expect(subjectsOf(rows)).toEqual(['User:root@prod']);
  });

  it('cluster-admin grant is flagged viaBuiltin with cluster-wide scope', () => {
    const rows = materializeGrants(FLEET, { verb: 'delete', resource: 'pods' });
    const root = rows.find((r) => r.subject === 'User:root');
    expect(root?.viaBuiltin).toBe(true);
    expect(root?.role).toBe('cluster-admin');
    expect(root?.scope).toBeNull();
  });

  it('a namespaced grant carries its namespace as scope', () => {
    const rows = materializeGrants(FLEET, { verb: 'get', resource: 'secrets' });
    const robot = rows.find((r) => r.subject === 'ServiceAccount:web/robot');
    expect(robot?.scope).toBe('web');
    expect(robot?.role).toBe('web-admin');
    expect(robot?.viaBuiltin).toBe(false);
  });

  it('no access query → every (binding × subject) grant, all clusters', () => {
    const rows = materializeGrants(FLEET);
    // editor(alice,devs) + root + viewer(alice) + robot = 5
    expect(rows).toHaveLength(5);
    expect(subjectsOf(rows)).toEqual([
      'Group:devs@prod',
      'ServiceAccount:web/robot@dev',
      'User:alice@dev',
      'User:alice@prod',
      'User:root@prod',
    ]);
  });

  it('verb omitted = any verb (who can touch pods at all)', () => {
    const rows = materializeGrants(FLEET, { resource: 'pods' });
    // editor + viewer + cluster-admin all touch pods.
    expect(subjectsOf(rows)).toEqual([
      'Group:devs@prod',
      'User:alice@dev',
      'User:alice@prod',
      'User:root@prod',
    ]);
  });
});
