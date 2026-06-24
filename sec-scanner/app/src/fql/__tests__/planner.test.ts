import { describe, expect, it } from 'vitest';

import { FqlError } from '../errors';
import { parse } from '../parser';
import { plan } from '../planner';

const planOf = (q: string) => plan(parse(q));

describe('planner — pushdown', () => {
  it('pushes a single AND group to one fetch', () => {
    const p = planOf("SELECT slug FROM units WHERE space LIKE 'sec-demo-%'");
    expect(p.fetches).toEqual([{ where: "Space.Slug LIKE 'sec-demo-%'" }]);
    expect(p.source).toBe('units');
  });

  it('splits a top-level OR into two fetches (DNF)', () => {
    const p = planOf(
      "SELECT unit FROM resources WHERE severity = 'CRITICAL' OR image ~ ':latest'",
    );
    expect(p.fetches).toHaveLength(2);
    // severity is client-side only → that group has no pushdown.
    expect(p.fetches[0]).toEqual({});
    // image pushes to where_data.
    expect(p.fetches[1]).toEqual({
      whereData: "spec.template.spec.containers.*.image ~ ':latest'",
    });
  });

  it('distributes AND over OR correctly', () => {
    // kind='Deployment' AND (replicas>1 OR image~':latest')
    //   => [kind, replicas], [kind, image]
    const p = planOf(
      "SELECT unit FROM resources WHERE kind = 'Deployment' AND (replicas > 1 OR image ~ ':latest')",
    );
    expect(p.fetches).toHaveLength(2);
    expect(p.fetches[0]).toEqual({ whereData: "kind = 'Deployment' AND spec.replicas > 1" });
    expect(p.fetches[1]).toEqual({
      whereData: "kind = 'Deployment' AND spec.template.spec.containers.*.image ~ ':latest'",
    });
  });

  it('separates where / where_data / whereResource buckets', () => {
    const p = planOf(
      "SELECT unit FROM resources WHERE space = 'sec-demo-dev' AND kind = 'Deployment' AND resourceType = 'apps/v1/Deployment'",
    );
    expect(p.fetches).toEqual([
      {
        where: "Space.Slug = 'sec-demo-dev'",
        whereData: "kind = 'Deployment'",
        whereResource: "ConfigHub.ResourceType = 'apps/v1/Deployment'",
      },
    ]);
  });

  it('always keeps the full WHERE as residual', () => {
    const p = planOf("SELECT unit FROM resources WHERE severity = 'CRITICAL'");
    expect(p.residual).not.toBeNull();
    // severity is not pushable → fetch is unconstrained, residual carries it.
    expect(p.fetches).toEqual([{}]);
  });

  it('handles NOT via De Morgan + operator flip', () => {
    // NOT (a = 'x') => a != 'x' pushed down
    const p = planOf("SELECT slug FROM units WHERE NOT (slug = 'frontend')");
    expect(p.fetches).toEqual([{ where: "Slug != 'frontend'" }]);
  });

  it('de-dups identical OR branches', () => {
    const p = planOf("SELECT slug FROM units WHERE space = 'a' OR space = 'a'");
    expect(p.fetches).toHaveLength(1);
  });

  it('strips a table alias before resolving columns', () => {
    const p = planOf("SELECT unit FROM resources r WHERE r.kind = 'Deployment'");
    expect(p.fetches).toEqual([{ whereData: "kind = 'Deployment'" }]);
  });

  it('pushes a raw YAML data path to where_data verbatim', () => {
    const p = planOf("SELECT unit FROM resources WHERE spec.strategy.type = 'RollingUpdate'");
    expect(p.fetches).toEqual([{ whereData: "spec.strategy.type = 'RollingUpdate'" }]);
  });

  it('pushes a backtick-quoted wildcard path', () => {
    const p = planOf(
      "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` ~ ':latest'",
    );
    expect(p.fetches).toEqual([
      { whereData: "spec.template.spec.containers.*.image ~ ':latest'" },
    ]);
  });

  it('alias + raw path together', () => {
    const p = planOf('SELECT unit FROM resources r WHERE r.spec.replicas > 1');
    expect(p.fetches).toEqual([{ whereData: 'spec.replicas > 1' }]);
  });

  it('still rejects unknown columns on non-raw tables', () => {
    expect(() => planOf('SELECT nope FROM units')).toThrow(/unknown column/);
  });
});

describe('planner — validation', () => {
  it('rejects an unknown table', () => {
    expect(() => planOf('SELECT x FROM widgets')).toThrow(/unknown table/);
  });

  it('rejects an unknown column', () => {
    expect(() => planOf('SELECT nope FROM units')).toThrow(/unknown column/);
  });

  it('rejects a string operator on a number column', () => {
    expect(() => planOf("SELECT unit FROM resources WHERE replicas LIKE '1'")).toThrow(
      /only valid on string/,
    );
  });

  it('rejects a non-grouped column alongside an aggregate', () => {
    expect(() =>
      planOf('SELECT space, COUNT(*) FROM resources'),
    ).toThrow(/must appear in GROUP BY/);
  });

  it('accepts a valid GROUP BY projection', () => {
    expect(() =>
      planOf('SELECT space, COUNT(*) AS n FROM resources GROUP BY space'),
    ).not.toThrow();
  });

  it('allows ORDER BY referencing a SELECT alias (not a base column)', () => {
    expect(() =>
      planOf('SELECT space, COUNT(*) AS n FROM resources GROUP BY space ORDER BY n DESC'),
    ).not.toThrow();
  });

  it('throws FqlError with a position on bad column', () => {
    try {
      planOf('SELECT bogus FROM units');
      expect.unreachable();
    } catch (e) {
      expect(e).toBeInstanceOf(FqlError);
      expect((e as FqlError).pos).not.toBeNull();
    }
  });
});
