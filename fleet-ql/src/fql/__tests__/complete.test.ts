import { describe, expect, it } from 'vitest';

import { completionsAt } from '../complete';

/** Labels of the contextual candidates at the END of `text` (caret at end). */
const at = (text: string): string[] => completionsAt(text, text.length).map((c) => c.label);

describe('completion — clause context', () => {
  it('start of a query offers SELECT', () => {
    expect(at('')).toEqual(['SELECT']);
  });

  it('SELECT offers *, aggregates (and columns once FROM is known)', () => {
    expect(at('SELECT ')).toEqual(expect.arrayContaining(['*', 'COUNT(*)', 'MAX(']));
  });

  it('after a projection offers AS / FROM', () => {
    expect(at('SELECT slug ')).toEqual(expect.arrayContaining(['AS', 'FROM']));
  });

  it('after FROM offers table names only — not keywords or columns', () => {
    const c = at('SELECT slug FROM ');
    expect(c).toEqual(expect.arrayContaining(['units', 'resources', 'grants', 'rbac_findings']));
    expect(c).not.toContain('WHERE');
    expect(c).not.toContain('slug');
  });

  it('after the table offers the following clauses', () => {
    expect(at('SELECT slug FROM units ')).toEqual(
      expect.arrayContaining(['WHERE', 'GROUP BY', 'ORDER BY', 'LIMIT']),
    );
  });
});

describe('completion — WHERE', () => {
  it('predicate start offers columns + NOT', () => {
    const c = at('SELECT unit FROM resources WHERE ');
    expect(c).toEqual(expect.arrayContaining(['kind', 'cluster', 'NOT']));
  });

  it('after a column offers operators', () => {
    expect(at('SELECT unit FROM resources WHERE kind ')).toEqual(
      expect.arrayContaining(['=', 'LIKE', 'IN', 'IS']),
    );
  });

  it('IS offers NULL / NOT NULL', () => {
    expect(at('SELECT slug FROM units WHERE target IS ')).toEqual(['NULL', 'NOT NULL']);
  });

  it('a boolean column after = offers true/false', () => {
    expect(at('SELECT name FROM roles WHERE hasWildcard = ')).toEqual(['true', 'false']);
  });

  it('a timestamp column after > offers now()', () => {
    expect(at('SELECT unit FROM revisions WHERE createdAt > ')).toEqual(['now()']);
  });

  it('after a complete predicate offers AND/OR + clauses', () => {
    expect(at("SELECT slug FROM units WHERE space = 'p' ")).toEqual(
      expect.arrayContaining(['AND', 'OR', 'GROUP BY', 'ORDER BY', 'LIMIT']),
    );
  });

  it('a column-vs-column RHS is treated as a complete predicate', () => {
    // after the RHS column, suggest AND/OR/clauses — NOT operators.
    const c = at('SELECT slug FROM units WHERE headRevisionNum > liveRevisionNum ');
    expect(c).toContain('AND');
    expect(c).not.toContain('LIKE');
  });
});

describe('completion — GROUP BY / ORDER BY scope', () => {
  it('GROUP BY offers only the SELECT base columns', () => {
    // space is a base column; n is an aggregate alias (excluded); slug not selected.
    expect(at('SELECT space, COUNT(*) AS n FROM units GROUP BY ')).toEqual(['space']);
  });

  it('ORDER BY offers the SELECT outputs (columns + aliases)', () => {
    expect(at('SELECT space, COUNT(*) AS n FROM units GROUP BY space ORDER BY ')).toEqual(
      expect.arrayContaining(['space', 'n']),
    );
  });

  it('after an ORDER BY key offers ASC / DESC / LIMIT', () => {
    expect(at('SELECT space FROM units ORDER BY space ')).toEqual(['ASC', 'DESC', 'LIMIT']);
  });

  it('after ASC/DESC offers LIMIT', () => {
    expect(at('SELECT space FROM units ORDER BY space DESC ')).toEqual(['LIMIT']);
  });

  it('SELECT * falls back to all table columns for ORDER BY', () => {
    expect(at('SELECT * FROM spaces ORDER BY ')).toEqual(expect.arrayContaining(['slug', 'environment']));
  });
});

describe('completion — suppression', () => {
  it('returns nothing inside a string literal', () => {
    const q = "SELECT unit FROM resources WHERE kind = 'Dep";
    expect(completionsAt(q, q.length)).toEqual([]);
  });
});
