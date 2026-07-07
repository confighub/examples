import { describe, expect, it } from 'vitest';

import type { Row } from '../evaluate';
import { planQuery, runQuery } from '../index';
import type { ListParams, Transport } from '../transport';

describe('roles / bindings — planner', () => {
  it('pushes space to where; computed flags are client-side residual', () => {
    const p = planQuery("SELECT name FROM roles WHERE space = 'rbac-demo-prod' AND hasWildcard = true");
    expect(p.source).toBe('roles');
    expect(p.fetches).toEqual([{ where: "Space.Slug = 'rbac-demo-prod'" }]);
    expect(p.residual).not.toBeNull(); // hasWildcard re-checked client-side
  });

  it('bindings: orphaned / clusterAdmin are queryable booleans', () => {
    const p = planQuery('SELECT name, roleRef FROM bindings WHERE orphaned = true OR clusterAdmin = true');
    expect(p.source).toBe('bindings');
    // OR over two client-side booleans → two empty fetch specs, de-duped to one
    // unconstrained fetch; the full OR is re-checked client-side.
    expect(p.fetches).toEqual([{}]);
    expect(p.residual).not.toBeNull();
  });

  it('cluster stays client-side on roles (Target/Space fallback)', () => {
    const p = planQuery("SELECT name FROM roles WHERE cluster = 'prod'");
    expect(p.fetches).toEqual([{}]);
  });
});

function mockTransport(roles: Row[], bindings: Row[]): Transport & { roleCalls: ListParams[] } {
  const t = {
    roleCalls: [] as ListParams[],
    async units() {
      return [];
    },
    async resources() {
      return [];
    },
    async spaces() {
      return [];
    },
    async revisions() {
      return [];
    },
    async grants() {
      return [];
    },
    async roles(p: ListParams) {
      t.roleCalls.push(p);
      return roles.map((r) => ({ ...r }));
    },
    async bindings() {
      return bindings.map((r) => ({ ...r }));
    },
    async rbacFindings() {
      return [];
    },
  };
  return t;
}

describe('roles / bindings — end to end', () => {
  const ROLES: Row[] = [
    { name: 'legacy-admin', kind: 'ClusterRole', cluster: 'prod', space: 'p', unit: 'u', namespace: null, hasWildcard: true, aggregated: false, ruleCount: 1 },
    { name: 'viewer', kind: 'ClusterRole', cluster: 'prod', space: 'p', unit: 'u', namespace: null, hasWildcard: false, aggregated: false, ruleCount: 1 },
  ];
  const BINDINGS: Row[] = [
    { name: 'dangling', kind: 'RoleBinding', cluster: 'prod', space: 'p', unit: 'u', namespace: 'web', roleRef: 'ghost', roleRefKind: 'Role', subjectCount: 1, orphaned: true, clusterAdmin: false },
    { name: 'viewers', kind: 'RoleBinding', cluster: 'prod', space: 'p', unit: 'u', namespace: 'web', roleRef: 'viewer', roleRefKind: 'ClusterRole', subjectCount: 2, orphaned: false, clusterAdmin: false },
  ];

  it('filters wildcard roles client-side and forwards the space scope', async () => {
    const t = mockTransport(ROLES, BINDINGS);
    const res = await runQuery("SELECT name FROM roles WHERE space = 'p' AND hasWildcard = true", t);
    expect(t.roleCalls[0].where).toBe("Space.Slug = 'p'");
    expect(res.rows).toEqual([{ name: 'legacy-admin' }]);
  });

  it('finds orphaned bindings', async () => {
    const t = mockTransport(ROLES, BINDINGS);
    const res = await runQuery('SELECT name, roleRef FROM bindings WHERE orphaned = true', t);
    expect(res.rows).toEqual([{ name: 'dangling', roleRef: 'ghost' }]);
  });
});
