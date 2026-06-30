import { describe, expect, it } from 'vitest';

import type { CompareExpr, LiteralExpr } from '../ast';
import { FqlError } from '../errors';
import { parse } from '../parser';
import { plan } from '../planner';

// A fixed instant so now() folds deterministically: 2026-06-24T12:00:00.000Z.
const NOW = new Date('2026-06-24T12:00:00.000Z');

/** Pull the RHS literal value of a top-level `col <op> value` WHERE. */
function rhsLiteral(query: string): LiteralExpr {
  const stmt = parse(query, { now: NOW });
  const cmp = stmt.where as CompareExpr;
  expect(cmp.kind).toBe('compare');
  expect(cmp.right.kind).toBe('literal');
  return cmp.right as LiteralExpr;
}

describe('now() — temporal literal folding', () => {
  it('folds bare now() to the current instant as an RFC3339 string', () => {
    const lit = rhsLiteral('SELECT unit FROM revisions WHERE createdAt > now()');
    expect(lit.type).toBe('string');
    expect(lit.value).toBe('2026-06-24T12:00:00.000Z');
  });

  it('subtracts an interval (the "last 24h" idiom)', () => {
    const lit = rhsLiteral(
      "SELECT unit FROM revisions WHERE createdAt > now() - interval '24h'",
    );
    expect(lit.value).toBe('2026-06-23T12:00:00.000Z');
  });

  it('adds an interval', () => {
    const lit = rhsLiteral("SELECT unit FROM revisions WHERE createdAt < now() + interval '1h'");
    expect(lit.value).toBe('2026-06-24T13:00:00.000Z');
  });

  it('accepts the interval keyword as optional', () => {
    const a = rhsLiteral("SELECT unit FROM revisions WHERE createdAt > now() - '7d'");
    const b = rhsLiteral("SELECT unit FROM revisions WHERE createdAt > now() - interval '7d'");
    expect(a.value).toBe('2026-06-17T12:00:00.000Z');
    expect(a.value).toBe(b.value);
  });

  it.each([
    ["30m", '2026-06-24T11:30:00.000Z'],
    ["90s", '2026-06-24T11:58:30.000Z'],
    ["2w", '2026-06-10T12:00:00.000Z'],
    ["1 day", '2026-06-23T12:00:00.000Z'],
    ["3 hours", '2026-06-24T09:00:00.000Z'],
  ])("understands interval '%s'", (interval, expected) => {
    const lit = rhsLiteral(
      `SELECT unit FROM revisions WHERE createdAt > now() - interval '${interval}'`,
    );
    expect(lit.value).toBe(expected);
  });

  it('rejects a malformed interval', () => {
    expect(() => parse("SELECT unit FROM revisions WHERE createdAt > now() - interval 'soon'", { now: NOW })).toThrow(
      FqlError,
    );
  });

  it('rejects an unsupported unit (months/years are not fixed-length)', () => {
    expect(() =>
      parse("SELECT unit FROM revisions WHERE createdAt > now() - interval '2mo'", { now: NOW }),
    ).toThrow(/unknown interval unit/);
  });

  it('folds the same when no now is injected (just non-deterministic value)', () => {
    // Without {now}, it folds to the real clock — still a valid RFC3339 string.
    const stmt = parse('SELECT unit FROM revisions WHERE createdAt > now()');
    const lit = (stmt.where as CompareExpr).right as LiteralExpr;
    expect(lit.type).toBe('string');
    expect(() => new Date(lit.value as string).toISOString()).not.toThrow();
  });
});

describe('now() — pushes down as a constant timestamp', () => {
  it('compiles createdAt > now() - interval to a where clause with the folded literal', () => {
    const p = plan(
      parse(
        "SELECT unit, revisionNum FROM revisions WHERE space = 'sec-demo-dev' AND createdAt > now() - interval '24h'",
        { now: NOW },
      ),
    );
    expect(p.fetches).toHaveLength(1);
    // space scopes which units (whereUnit); the timestamp pushes to where.
    expect(p.fetches[0].whereUnit).toBe("Space.Slug = 'sec-demo-dev'");
    expect(p.fetches[0].where).toBe("CreatedAt > '2026-06-23T12:00:00.000Z'");
  });

  it('a folded now() literal is just a string — no special residual handling', () => {
    const p = plan(parse('SELECT unit FROM revisions WHERE createdAt > now()', { now: NOW }));
    // The whole WHERE survives as residual (re-checked client-side).
    expect(p.residual).not.toBeNull();
  });
});
