import { describe, expect, it } from 'vitest';

import type { CompareExpr } from '../ast';
import { compileCompare, joinAnd } from '../compile';
import { FqlError } from '../errors';
import { parse } from '../parser';
import { resolveColumn, TABLES } from '../schema';

/** Pull the single top-level comparison out of a WHERE clause and compile it
 *  against the named table. */
function compileWhere(query: string, table: keyof typeof TABLES): string {
  const stmt = parse(query);
  const cmp = stmt.where as CompareExpr;
  const col = resolveColumn(TABLES[table], cmp.left.path);
  if (!col) throw new Error(`unknown column in test: ${cmp.left.name}`);
  return compileCompare(cmp, col);
}

describe('compile — clause fragments', () => {
  it('compiles a string equality on a where column', () => {
    expect(compileWhere("SELECT slug FROM units WHERE space = 'prod'", 'units')).toBe(
      "Space.Slug = 'prod'",
    );
  });

  it('maps image to the container array wildcard path (where_data)', () => {
    expect(compileWhere("SELECT unit FROM resources WHERE image ~ ':latest'", 'resources')).toBe(
      "spec.template.spec.containers.*.image ~ ':latest'",
    );
  });

  it('compiles a number comparison without quotes', () => {
    expect(compileWhere('SELECT unit FROM resources WHERE replicas > 1', 'resources')).toBe(
      'spec.replicas > 1',
    );
  });

  it('compiles a dynamic label map column', () => {
    expect(compileWhere("SELECT slug FROM spaces WHERE labels.env = 'prod'", 'spaces')).toBe(
      "Labels.env = 'prod'",
    );
  });

  it('compiles IN lists', () => {
    expect(
      compileWhere("SELECT slug FROM units WHERE space IN ('a', 'b', 'c')", 'units'),
    ).toBe("Space.Slug IN ('a', 'b', 'c')");
  });

  it('compiles LIKE', () => {
    expect(compileWhere("SELECT slug FROM units WHERE slug LIKE 'sec-demo-%'", 'units')).toBe(
      "Slug LIKE 'sec-demo-%'",
    );
  });
});

describe('compile — escaping & injection', () => {
  it("doubles single quotes in string literals", () => {
    expect(compileWhere("SELECT slug FROM units WHERE slug = 'it''s'", 'units')).toBe(
      "Slug = 'it''s'",
    );
  });

  it('rejects an IN value containing a quote (injection gate)', () => {
    // The parser decodes '' to a literal quote; that value is illegal in IN().
    expect(() =>
      compileWhere("SELECT slug FROM units WHERE space IN ('a'')')", 'units'),
    ).toThrow(FqlError);
  });

  it('joinAnd composes fragments with AND', () => {
    expect(joinAnd(["Slug = 'x'", "ToolchainType = 'Kubernetes/YAML'"])).toBe(
      "Slug = 'x' AND ToolchainType = 'Kubernetes/YAML'",
    );
  });
});
