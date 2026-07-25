import { describe, expect, it } from 'vitest';

import type { UnionStmt } from '../ast';
import type { Row } from '../evaluate';
import { parseStatement, planStatement, runQuery } from '../index';
import type { Transport } from '../transport';

// A tiny two-table fleet. WHERE narrows client-side (residual), so the mock can
// return everything and let evaluate filter.
const UNITS: Row[] = [
  { slug: 'a', space: 's1' },
  { slug: 'b', space: 's1' },
  { slug: 'c', space: 's2' },
];
const RESOURCES: Row[] = [
  { name: 'a', kind: 'Deployment', unit: 'a' },
  { name: 'svc', kind: 'Service', unit: 'a' },
];

function mock(): Transport {
  return {
    async units() {
      return UNITS.map((r) => ({ ...r }));
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

const asUnion = (q: string): UnionStmt => parseStatement(q) as UnionStmt;

describe('UNION — parser', () => {
  it('parses two branches with a distinct connector', () => {
    const u = asUnion('SELECT slug FROM units UNION SELECT slug FROM units WHERE space = ' + "'s2'");
    expect(u.kind).toBe('union');
    expect(u.branches).toHaveLength(2);
    expect(u.all).toEqual([false]);
  });

  it('parses UNION ALL and chains of 3+ branches', () => {
    const u = asUnion('SELECT slug FROM units UNION ALL SELECT slug FROM units UNION SELECT slug FROM units');
    expect(u.branches).toHaveLength(3);
    expect(u.all).toEqual([true, false]);
  });

  it('binds a trailing ORDER BY / LIMIT to the union, not a branch', () => {
    const u = asUnion('SELECT slug FROM units UNION SELECT name FROM resources ORDER BY slug DESC LIMIT 2');
    expect(u.orderBy).toHaveLength(1);
    expect(u.limit).toBe(2);
    expect(u.branches[0].orderBy).toEqual([]);
  });

  it('rejects a per-branch ORDER BY before UNION', () => {
    expect(() => parseStatement('SELECT slug FROM units ORDER BY slug UNION SELECT slug FROM units')).toThrow(
      /unexpected/i,
    );
  });
});

describe('UNION — planner', () => {
  it('plans one ExecutionPlan per branch', () => {
    const p = planStatement('SELECT slug FROM units UNION SELECT name FROM resources');
    expect('kind' in p && p.kind === 'union').toBe(true);
    if ('kind' in p) expect(p.branches).toHaveLength(2);
  });

  it('rejects branches with mismatched column counts', () => {
    expect(() => planStatement('SELECT slug FROM units UNION SELECT slug, space FROM units')).toThrow(
      /same number of columns/,
    );
  });
});

describe('UNION — execution', () => {
  it('de-dupes overlapping rows (plain UNION)', async () => {
    const res = await runQuery("SELECT slug FROM units WHERE space = 's1' UNION SELECT slug FROM units", mock());
    expect(res.columns).toEqual(['slug']);
    expect(res.rows.map((r) => r.slug).sort()).toEqual(['a', 'b', 'c']);
  });

  it('keeps duplicates with UNION ALL', async () => {
    const res = await runQuery(
      "SELECT slug FROM units WHERE space = 's1' UNION ALL SELECT slug FROM units",
      mock(),
    );
    // {a,b} ++ {a,b,c} = 5 rows, no de-dupe.
    expect(res.rows).toHaveLength(5);
  });

  it('aligns columns by position across different tables', async () => {
    const res = await runQuery(
      "SELECT slug FROM units WHERE slug = 'a' UNION SELECT name FROM resources WHERE kind = 'Service'",
      mock(),
    );
    expect(res.columns).toEqual(['slug']); // output names come from the first branch
    expect(res.rows.map((r) => r.slug).sort()).toEqual(['a', 'svc']);
  });

  it('applies a trailing ORDER BY / LIMIT to the combined result', async () => {
    const res = await runQuery(
      'SELECT slug FROM units UNION SELECT name FROM resources ORDER BY slug DESC LIMIT 2',
      mock(),
    );
    // distinct slugs {a,b,c,svc} → DESC → [svc,c,b,a] → limit 2.
    expect(res.rows.map((r) => r.slug)).toEqual(['svc', 'c']);
  });

  it('sums fetch stats across branches', async () => {
    const res = await runQuery('SELECT slug FROM units UNION SELECT name FROM resources', mock());
    expect(res.stats.fetches).toBe(2);
  });

  it('rejects a trailing ORDER BY on a non-output column', async () => {
    await expect(
      runQuery('SELECT slug FROM units UNION SELECT name FROM resources ORDER BY space', mock()),
    ).rejects.toThrow(/output column/);
  });
});
