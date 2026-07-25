import { describe, expect, it } from 'vitest';

import type { CompareExpr } from '../ast';
import { FqlError } from '../errors';
import type { Row } from '../evaluate';
import { runQuery } from '../index';
import { parse } from '../parser';
import { plan } from '../planner';
import type { Transport } from '../transport';

const planOf = (q: string) => plan(parse(q));

// ─── Parser ───────────────────────────────────────────────────────────────────

describe('join — parser', () => {
  it('parses an INNER join (bare JOIN) with an ON equality', () => {
    const s = parse('SELECT d.name FROM resources d JOIN resources p ON d.name = p.name');
    expect(s.from).toMatchObject({ name: 'resources', alias: 'd' });
    expect(s.joins).toHaveLength(1);
    expect(s.joins[0].type).toBe('inner');
    expect(s.joins[0].table).toMatchObject({ name: 'resources', alias: 'p' });
    expect((s.joins[0].on as CompareExpr).op).toBe('=');
  });

  it('parses LEFT [OUTER] JOIN as a left join', () => {
    expect(parse('SELECT u.slug FROM units u LEFT JOIN units v ON u.space = v.space').joins[0].type).toBe('left');
    expect(parse('SELECT u.slug FROM units u LEFT OUTER JOIN units v ON u.space = v.space').joins[0].type).toBe('left');
    expect(parse('SELECT u.slug FROM units u INNER JOIN units v ON u.space = v.space').joins[0].type).toBe('inner');
  });
});

// ─── Planner ──────────────────────────────────────────────────────────────────

describe('join — planner', () => {
  it('pushes each side independently and extracts the ON keys', () => {
    const p = planOf(
      "SELECT u.slug FROM units u JOIN units v ON u.space = v.space WHERE u.toolchain = 'Helm' AND v.toolchain = 'Kustomize'",
    );
    expect(p.join).toBeDefined();
    expect(p.join!.type).toBe('inner');
    expect(p.join!.on).toEqual([{ left: 'space', right: 'space' }]);
    expect(p.join!.left.fetches).toEqual([{ where: "ToolchainType = 'Helm'" }]);
    expect(p.join!.right.fetches).toEqual([{ where: "ToolchainType = 'Kustomize'" }]);
    expect(p.join!.residual).not.toBeNull(); // full WHERE re-checked on combined rows
  });

  it('rejects a missing alias', () => {
    expect(() => planOf('SELECT u.slug FROM units JOIN units v ON u.space = v.space')).toThrow(
      /JOIN requires table aliases/,
    );
  });

  it('rejects an unqualified column', () => {
    expect(() =>
      planOf("SELECT slug FROM units u JOIN units v ON u.space = v.space WHERE toolchain = 'Helm'"),
    ).toThrow(/must be qualified/);
  });

  it('rejects a fetch selector (revision) inside a join', () => {
    expect(() =>
      planOf("SELECT d.name FROM resources d JOIN resources p ON d.name = p.name WHERE d.revision = 5"),
    ).toThrow(/selector.*JOIN/);
  });

  it('rejects a raw-path ON key', () => {
    expect(() =>
      planOf('SELECT d.name FROM resources d JOIN resources p ON `d.spec.x` = `p.spec.x`'),
    ).toThrow(/must be a column/);
  });

  it('rejects more than one join', () => {
    expect(() =>
      planOf('SELECT a.slug FROM units a JOIN units b ON a.space = b.space JOIN units c ON a.space = c.space'),
    ).toThrow(/only one JOIN/);
  });
});

// ─── Executor (hash join) ───────────────────────────────────────────────────────

const img = (image: string) => ({
  spec: { template: { spec: { containers: [{ image }] } } },
});
const RES: Row[] = [
  { name: 'frontend', environment: 'Dev', space: 'dev', __doc: img('nginx:1.27') },
  { name: 'frontend', environment: 'Prod', space: 'prod', __doc: img('nginx:1.26') },
  { name: 'api', environment: 'Dev', space: 'dev', __doc: img('py:3.12') },
  { name: 'api', environment: 'Prod', space: 'prod', __doc: img('py:3.12') },
];

function mockTransport(sets: { units?: Row[]; resources?: Row[] }): Transport {
  const stub = async () => [];
  const give = (rows: Row[] | undefined) => async () => (rows ?? []).map((r) => ({ ...r }));
  return {
    units: give(sets.units) as Transport['units'],
    spaces: stub,
    revisions: stub,
    grants: stub,
    roles: stub,
    bindings: stub,
    rbacFindings: stub,
    resources: give(sets.resources) as Transport['resources'],
  };
}

describe('join — executor (self-join image diff)', () => {
  it('finds workloads whose dev image differs from prod (qualified raw paths)', async () => {
    const res = await runQuery(
      'SELECT d.name AS name, `d.spec.template.spec.containers.*.image` AS dev, ' +
        '`p.spec.template.spec.containers.*.image` AS prod ' +
        'FROM resources d JOIN resources p ON d.name = p.name ' +
        "WHERE d.environment = 'Dev' AND p.environment = 'Prod' " +
        'AND `d.spec.template.spec.containers.*.image` != `p.spec.template.spec.containers.*.image`',
      mockTransport({ resources: RES }),
    );
    // frontend drifted (1.27 vs 1.26); api converged → excluded.
    expect(res.rows).toEqual([{ name: 'frontend', dev: 'nginx:1.27', prod: 'nginx:1.26' }]);
  });

  it('INNER drops unmatched rows; LEFT keeps them with a null right side', async () => {
    // units LEFT JOIN their resources — `empty` has none.
    const t = mockTransport({
      units: [{ slug: 'has-res' }, { slug: 'empty' }],
      resources: [{ unit: 'has-res', name: 'web' }],
    });
    const inner = await runQuery(
      'SELECT u.slug AS s, r.name AS rn FROM units u JOIN resources r ON u.slug = r.unit',
      t,
    );
    expect(inner.rows).toEqual([{ s: 'has-res', rn: 'web' }]); // `empty` dropped
    const lj = await runQuery(
      'SELECT u.slug AS s, r.name AS rn FROM units u LEFT JOIN resources r ON u.slug = r.unit ORDER BY s',
      t,
    );
    expect(lj.rows.map((r) => r.s)).toEqual(['empty', 'has-res']); // empty kept
    expect(lj.rows[0].rn ?? null).toBeNull(); // right side empty
    expect(lj.rows[1].rn).toBe('web');
  });
});
