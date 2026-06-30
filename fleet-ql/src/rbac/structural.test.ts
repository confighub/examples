import { describe, expect, it } from 'vitest';

import {
  clusterRole,
  clusterRoleBinding,
  res,
  role,
  roleBinding,
  user,
} from './fixtures';
import type { FleetResource } from './model';
import { materializeBindings, materializeRoles } from './structural';

const FLEET: FleetResource[] = [
  // prod: a wildcard ClusterRole, a labeled aggregated role, and bindings.
  res('prod', clusterRole('legacy-admin', [{ apiGroups: ['*'], resources: ['*'], verbs: ['*'] }])),
  res('prod', clusterRole('viewer', [{ apiGroups: [''], resources: ['pods'], verbs: ['get', 'list'] }], { labels: { team: 'platform' } })),
  res('prod', clusterRole('monitoring', [], { aggregationRule: { clusterRoleSelectors: [{ matchLabels: { agg: 'true' } }] } })),
  res('prod', clusterRoleBinding('admins', 'legacy-admin', [user('root')])),
  res('prod', clusterRoleBinding('superusers', 'cluster-admin', [user('breakglass')])),
  res('prod', roleBinding('dangling', 'web', { kind: 'Role', name: 'ghost-role' }, [user('nobody')])),
  res('prod', roleBinding('viewers', 'web', { kind: 'ClusterRole', name: 'viewer' }, [user('alice'), user('bob')])),
];

describe('materializeRoles', () => {
  const rows = materializeRoles(FLEET);
  const byName = Object.fromEntries(rows.map((r) => [r.name, r]));

  it('flags wildcard roles', () => {
    expect(byName['legacy-admin'].hasWildcard).toBe(true);
    expect(byName['viewer'].hasWildcard).toBe(false);
  });

  it('flags aggregated roles', () => {
    expect(byName['monitoring'].aggregated).toBe(true);
    expect(byName['viewer'].aggregated).toBe(false);
  });

  it('carries kind, cluster, ruleCount and labels', () => {
    expect(byName['viewer'].kind).toBe('ClusterRole');
    expect(byName['viewer'].cluster).toBe('prod');
    expect(byName['viewer'].ruleCount).toBe(1);
    expect(byName['viewer']['labels.team']).toBe('platform');
  });
});

describe('materializeBindings', () => {
  const rows = materializeBindings(FLEET);
  const byName = Object.fromEntries(rows.map((r) => [r.name, r]));

  it('flags cluster-admin / superuser bindings', () => {
    expect(byName['superusers'].clusterAdmin).toBe(true); // builtin cluster-admin
    expect(byName['admins'].clusterAdmin).toBe(true); // resolves to *-on-*-in-*
    expect(byName['viewers'].clusterAdmin).toBe(false);
  });

  it('flags orphaned bindings (roleRef resolves to nothing, not a builtin)', () => {
    expect(byName['dangling'].orphaned).toBe(true);
    expect(byName['viewers'].orphaned).toBe(false);
    // cluster-admin is a builtin, so the superuser binding is NOT orphaned.
    expect(byName['superusers'].orphaned).toBe(false);
  });

  it('carries roleRef, roleRefKind, subjectCount and scope', () => {
    expect(byName['viewers'].roleRef).toBe('viewer');
    expect(byName['viewers'].roleRefKind).toBe('ClusterRole');
    expect(byName['viewers'].subjectCount).toBe(2);
    expect(byName['viewers'].namespace).toBe('web');
    expect(byName['superusers'].namespace).toBeNull(); // ClusterRoleBinding
  });
});
