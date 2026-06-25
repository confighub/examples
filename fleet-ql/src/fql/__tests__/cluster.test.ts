import { describe, expect, it } from 'vitest';

import type { Row } from '../evaluate';
import { runQuery } from '../index';
import { planQuery } from '../index';
import { resolveColumn, TABLES } from '../schema';
import type { ListParams, ResourceParams, Transport } from '../transport';

describe('cluster — schema', () => {
  it('resolves on units and resources, client-side (no pushdown)', () => {
    const u = resolveColumn(TABLES.units, ['cluster']);
    const r = resolveColumn(TABLES.resources, ['cluster']);
    expect(u?.type).toBe('string');
    expect(u?.pushdown).toBeUndefined();
    expect(r?.pushdown).toBeUndefined();
  });

  it('target still pushes on units, but not on resources', () => {
    expect(resolveColumn(TABLES.units, ['target'])?.pushdown?.expr).toBe('Target.Slug');
    expect(resolveColumn(TABLES.resources, ['target'])?.pushdown).toBeUndefined();
  });
});

describe('cluster — planner', () => {
  it('cluster = x does not push; the residual re-checks it client-side', () => {
    const p = planQuery("SELECT slug FROM units WHERE cluster = 'prod'");
    expect(p.fetches).toEqual([{}]); // unconstrained fetch
    expect(p.residual).not.toBeNull();
  });

  it('combines a pushed scope with a client-side cluster filter', () => {
    const p = planQuery("SELECT slug FROM units WHERE space = 'rbac-demo-prod' AND cluster = 'prod'");
    // space pushes; cluster stays client-side.
    expect(p.fetches).toEqual([{ where: "Space.Slug = 'rbac-demo-prod'" }]);
  });
});

// A fleet where cluster is target-or-space: two bound units share target 'prod';
// one unbound unit falls back to its space slug.
const UNITS: Row[] = [
  { __id: '1', slug: 'frontend', space: 'rbac-demo-prod', target: 'prod', cluster: 'prod' },
  { __id: '2', slug: 'api', space: 'rbac-demo-staging', target: 'prod', cluster: 'prod' },
  { __id: '3', slug: 'legacy', space: 'rbac-demo-base', target: null, cluster: 'rbac-demo-base' },
];

function mockTransport(): Transport {
  return {
    async units(_p: ListParams) {
      return UNITS.map((r) => ({ ...r }));
    },
    async resources(_p: ResourceParams) {
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
    async roles() {
      return [];
    },
    async bindings() {
      return [];
    },
    async rbacFindings() {
      return [];
    },
  };
}

describe('cluster — end to end', () => {
  it('groups units across spaces by their deploy cluster', async () => {
    const res = await runQuery('SELECT cluster, COUNT(*) AS n FROM units GROUP BY cluster', mockTransport());
    const byCluster = Object.fromEntries(res.rows.map((r) => [r.cluster, r.n]));
    expect(byCluster).toEqual({ prod: 2, 'rbac-demo-base': 1 });
  });

  it('cluster = prod selects bound units regardless of space', async () => {
    const res = await runQuery("SELECT slug FROM units WHERE cluster = 'prod'", mockTransport());
    expect(res.rows.map((r) => r.slug).sort()).toEqual(['api', 'frontend']);
  });

  it('an unbound unit falls back to its space slug as cluster', async () => {
    const res = await runQuery("SELECT slug FROM units WHERE cluster = 'rbac-demo-base'", mockTransport());
    expect(res.rows.map((r) => r.slug)).toEqual(['legacy']);
  });
});
