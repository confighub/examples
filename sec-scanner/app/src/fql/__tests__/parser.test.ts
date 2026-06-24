import { describe, expect, it } from 'vitest';

import type { CompareExpr, LogicalExpr, SelectStmt } from '../ast';
import { FqlError, renderError } from '../errors';
import { parse } from '../parser';

describe('parser — statement shape', () => {
  it('parses SELECT cols FROM table', () => {
    const s = parse('SELECT slug, space FROM units');
    expect(s.from.name).toBe('units');
    expect(s.projections.map((p) => (p.expr.kind === 'column' ? p.expr.name : '?'))).toEqual([
      'slug',
      'space',
    ]);
    expect(s.where).toBeNull();
  });

  it('parses SELECT *', () => {
    const s = parse('SELECT * FROM units');
    expect(s.projections).toHaveLength(1);
    expect(s.projections[0].expr.kind).toBe('star');
  });

  it('parses projection aliases', () => {
    const s = parse('SELECT COUNT(*) AS n FROM resources');
    expect(s.projections[0].alias).toBe('n');
    expect(s.projections[0].expr.kind).toBe('agg');
  });

  it('parses GROUP BY, ORDER BY, LIMIT', () => {
    const s = parse(
      "SELECT space, COUNT(*) AS n FROM resources WHERE severity = 'CRITICAL' GROUP BY space ORDER BY n DESC LIMIT 10",
    );
    expect(s.groupBy.map((c) => c.name)).toEqual(['space']);
    expect(s.orderBy[0].dir).toBe('DESC');
    expect(s.limit).toBe(10);
  });

  it('parses dotted column paths', () => {
    const s = parse("SELECT slug FROM spaces WHERE labels.env = 'prod'");
    const cmp = s.where as CompareExpr;
    expect(cmp.left.path).toEqual(['labels', 'env']);
  });
});

describe('parser — expression precedence', () => {
  it('binds AND tighter than OR', () => {
    // a OR b AND c  =>  a OR (b AND c)
    const s = parse("SELECT slug FROM units WHERE a = '1' OR b = '2' AND c = '3'");
    const top = s.where as LogicalExpr;
    expect(top.op).toBe('OR');
    expect((top.right as LogicalExpr).op).toBe('AND');
  });

  it('parentheses override precedence', () => {
    // (a OR b) AND c  =>  AND at top
    const s = parse("SELECT slug FROM units WHERE (a = '1' OR b = '2') AND c = '3'");
    const top = s.where as LogicalExpr;
    expect(top.op).toBe('AND');
    expect((top.left as LogicalExpr).op).toBe('OR');
  });

  it('parses IN and NOT IN lists', () => {
    const s = parse("SELECT slug FROM units WHERE space IN ('a', 'b', 'c')");
    const cmp = s.where as CompareExpr;
    expect(cmp.op).toBe('IN');
    expect(cmp.right.kind).toBe('list');
  });

  it('parses IS NULL / IS NOT NULL', () => {
    const a = parse('SELECT slug FROM units WHERE target IS NULL');
    expect((a.where as { kind: string }).kind).toBe('isnull');
    const b = parse('SELECT slug FROM units WHERE target IS NOT NULL');
    expect((b.where as { negated: boolean }).negated).toBe(true);
  });

  it('parses regex and LIKE operators', () => {
    for (const op of ['~', '~*', '!~', '!~*']) {
      const s = parse(`SELECT slug FROM units WHERE image ${op} ':latest'`);
      expect((s.where as CompareExpr).op).toBe(op);
    }
    expect((parse("SELECT slug FROM units WHERE slug LIKE 'a%'").where as CompareExpr).op).toBe(
      'LIKE',
    );
    expect((parse("SELECT slug FROM units WHERE slug NOT LIKE 'a%'").where as CompareExpr).op).toBe(
      'NOT LIKE',
    );
  });
});

describe('parser — errors', () => {
  it('reports a missing FROM with a position', () => {
    try {
      parse('SELECT slug units');
      expect.unreachable();
    } catch (e) {
      expect(e).toBeInstanceOf(FqlError);
      expect((e as FqlError).pos).not.toBeNull();
    }
  });

  it('renders a caret under the offending token', () => {
    const q = 'SELECT slug FORM units';
    try {
      parse(q);
      expect.unreachable();
    } catch (e) {
      const out = renderError(q, e as FqlError);
      // "FORM" is parsed as an ident table?? No: SELECT slug, then expects FROM.
      expect(out).toContain('^');
      expect(out).toMatch(/line 1/);
    }
  });

  it('rejects trailing garbage', () => {
    expect(() => parse('SELECT slug FROM units extra')).toThrow(FqlError);
  });

  it('rejects a non-integer LIMIT', () => {
    expect(() => parse('SELECT slug FROM units LIMIT 1.5')).toThrow(/integer/);
  });
});
