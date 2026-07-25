import { describe, expect, it } from 'vitest';

import type { Row } from '../evaluate';
import { planQuery, runQuery } from '../index';
import { resolveColumn, TABLES } from '../schema';
import type { ListParams, Transport } from '../transport';

describe('environment/component/region — schema & pushdown', () => {
  it('push to Labels.* on units and spaces', () => {
    expect(resolveColumn(TABLES.units, ['environment'])?.pushdown).toEqual({
      target: 'where',
      expr: 'Labels.Environment',
    });
    expect(resolveColumn(TABLES.spaces, ['component'])?.pushdown?.expr).toBe('Labels.Component');
    expect(resolveColumn(TABLES.units, ['region'])?.pushdown?.expr).toBe('Labels.Region');
  });

  it('are client-side on resources (stamped via enrichment)', () => {
    expect(resolveColumn(TABLES.resources, ['environment'])?.pushdown).toBeUndefined();
  });

  it('compiles WHERE environment = ... to a Labels.Environment where clause', () => {
    const p = planQuery("SELECT slug FROM units WHERE environment = 'Prod'");
    expect(p.fetches).toEqual([{ where: "Labels.Environment = 'Prod'" }]);
  });

  it('resources environment is residual, not pushed', () => {
    const p = planQuery("SELECT unit FROM resources WHERE space = 'acme-x' AND environment = 'Prod'");
    expect(p.fetches).toEqual([{ where: "Space.Slug = 'acme-x'" }]);
    expect(p.residual).not.toBeNull();
  });
});

const RESOURCES: Row[] = [
  { space: 'acme-storefront-prod', unit: 'storefront', kind: 'Deployment', environment: 'Prod', component: 'storefront' },
  { space: 'acme-storefront-dev', unit: 'storefront', kind: 'Deployment', environment: 'Dev', component: 'storefront' },
];

function mockTransport(): Transport {
  return {
    async units(_p: ListParams) {
      return [];
    },
    async resources() {
      return RESOURCES.map((r) => ({ ...r }));
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

describe('environment on resources — end to end', () => {
  it('filters resources by the stamped environment client-side', async () => {
    const res = await runQuery(
      "SELECT unit, component FROM resources WHERE environment = 'Prod'",
      mockTransport(),
    );
    expect(res.rows).toEqual([{ unit: 'storefront', component: 'storefront' }]);
  });
});
