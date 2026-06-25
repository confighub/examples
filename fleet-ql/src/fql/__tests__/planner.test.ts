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
    // A bracket-keyed annotation can't push (dots/slash) → client-side group;
    // the container-image LIKE pushes down soundly.
    const p = planOf(
      "SELECT unit FROM resources WHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL' OR `spec.template.spec.containers.*.image` LIKE '%:latest'",
    );
    expect(p.fetches).toHaveLength(2);
    // the annotation group has no pushdown.
    expect(p.fetches[0]).toEqual({});
    // image LIKE pushes to where_data.
    expect(p.fetches[1]).toEqual({
      whereData: "spec.template.spec.containers.*.image LIKE '%:latest'",
    });
  });

  it('distributes AND over OR correctly', () => {
    // kind='Deployment' AND (replicas>1 OR image LIKE '%:latest')
    const p = planOf(
      "SELECT unit FROM resources WHERE kind = 'Deployment' AND (replicas > 1 OR `spec.template.spec.containers.*.image` LIKE '%:latest')",
    );
    expect(p.fetches).toHaveLength(2);
    expect(p.fetches[0]).toEqual({ whereData: "kind = 'Deployment' AND spec.replicas > 1" });
    expect(p.fetches[1]).toEqual({
      whereData: "kind = 'Deployment' AND spec.template.spec.containers.*.image LIKE '%:latest'",
    });
  });

  it('does NOT push regex down (POSIX/RE2 ≠ JS) — client-side only', () => {
    const p = planOf("SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` ~ ':latest'");
    // Sound: regex stays client-side so JS RegExp is authoritative.
    expect(p.fetches).toEqual([{}]);
    expect(p.residual).not.toBeNull();
  });

  it('does NOT push negation over a data path (missing-field/array semantics)', () => {
    const p = planOf("SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` != 'nginx:1.27-alpine'");
    expect(p.fetches).toEqual([{}]);
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
    // A bracket-keyed annotation can't push down → residual carries it.
    const p = planOf(
      "SELECT unit FROM resources WHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'",
    );
    expect(p.residual).not.toBeNull();
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

  it('pushes a backtick-quoted wildcard path (sound op)', () => {
    const p = planOf(
      "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` LIKE '%:latest'",
    );
    expect(p.fetches).toEqual([
      { whereData: "spec.template.spec.containers.*.image LIKE '%:latest'" },
    ]);
  });

  it('alias + raw path together', () => {
    const p = planOf('SELECT unit FROM resources r WHERE r.spec.replicas > 1');
    expect(p.fetches).toEqual([{ whereData: 'spec.replicas > 1' }]);
  });

  it('still rejects unknown columns on non-raw tables', () => {
    expect(() => planOf('SELECT nope FROM units')).toThrow(/unknown column/);
  });

  it('does NOT push down a bracket key with dots/slash (client-side only)', () => {
    const p = planOf(
      "SELECT unit FROM resources WHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'",
    );
    // The key can't be expressed in ConfigHub's dotted where_data → no pushdown.
    expect(p.fetches).toEqual([{}]);
    expect(p.residual).not.toBeNull();
  });

  it('pushes down a clean bracket-indexed path (sound op)', () => {
    const p = planOf("SELECT unit FROM resources WHERE spec.containers[0].image LIKE 'nginx%'");
    expect(p.fetches).toEqual([
      { whereData: "spec.containers.0.image LIKE 'nginx%'" },
    ]);
  });

  it('pushes a sound column-to-column comparison on entity fields', () => {
    const p = planOf('SELECT slug FROM units WHERE headRevisionNum > liveRevisionNum');
    expect(p.fetches).toEqual([{ where: 'HeadRevisionNum > LiveRevisionNum' }]);
  });

  it('queries any kind (all-kinds resources table)', () => {
    const p = planOf("SELECT unit FROM resources WHERE kind = 'Service'");
    expect(p.fetches).toEqual([{ whereData: "kind = 'Service'" }]);
  });

  it('revisions: splits unit-scope (whereUnit) from revision fields (where)', () => {
    const p = planOf("SELECT revisionNum FROM revisions WHERE unit = 'checkout' AND source = 'Trigger'");
    expect(p.source).toBe('revisions');
    expect(p.fetches).toEqual([
      { where: "Source = 'Trigger'", whereUnit: "Slug = 'checkout'" },
    ]);
  });

  it('revisions: space scopes whereUnit; revisionNum pushes to where', () => {
    const p = planOf('SELECT revisionNum FROM revisions WHERE space = ' + "'prod' AND revisionNum > 5");
    expect(p.fetches).toEqual([
      { where: 'RevisionNum > 5', whereUnit: "Space.Slug = 'prod'" },
    ]);
  });

  it('resources: revision = N becomes a fetch selector and is stripped from residual', () => {
    const p = planOf("SELECT unit FROM resources WHERE unit = 'checkout' AND revision = 5");
    expect(p.fetches).toEqual([{ where: "Slug = 'checkout'", revision: '5' }]);
    // residual keeps unit='checkout' but NOT the revision selector.
    expect(JSON.stringify(p.residual)).toContain('checkout');
    expect(JSON.stringify(p.residual)).not.toContain('revision');
  });

  it('resources: symbolic revision = live carries through and clears the residual', () => {
    const p = planOf("SELECT unit FROM resources WHERE revision = 'live'");
    expect(p.fetches).toEqual([{ revision: 'live' }]);
    // The only predicate was the selector, so the residual is now empty.
    expect(p.residual).toBeNull();
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
