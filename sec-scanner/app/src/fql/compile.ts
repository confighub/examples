// Compile a single pushdown predicate to a ConfigHub clause fragment
// (where / where_data / whereResource). Literal handling mirrors ConfigHub's
// own parser: single-quoted strings with '' doubling, bare numbers/bools, and
// IN-value validation that rejects anything that could break out of the quoted
// literal (the server applies the same gate in ValidateInClauseValues).

import type { CompareExpr, CompareOp, ListExpr, LiteralExpr } from './ast';
import { FqlError } from './errors';
import type { ResolvedColumn } from './schema';

/** Quote a string literal the way ConfigHub does: wrap in '', double interior '. */
function quoteString(v: string): string {
  return `'${v.replaceAll("'", "''")}'`;
}

/**
 * Validate that a string IN/literal value is safe to interpolate. ConfigHub
 * rejects embedded quotes/backslashes in IN values at parse time; we apply the
 * same rule so a compiled clause can never carry an injection. (Plain `=`
 * strings are safe via '' doubling, but IN values are concatenated raw, so the
 * stricter rule matches the server.)
 */
function assertSafeInValue(v: string, pos: CompareExpr['pos']): void {
  if (v.includes('\\') || v.includes("'")) {
    throw new FqlError(
      `value ${JSON.stringify(v)} contains a quote or backslash and cannot be used in IN(...)`,
      pos,
      'compile',
    );
  }
}

/** Render a scalar literal as a ConfigHub SQL literal. */
function literalSql(lit: LiteralExpr): string {
  switch (lit.type) {
    case 'string':
      return quoteString(lit.value as string);
    case 'number':
      return String(lit.value);
    case 'boolean':
      return (lit.value as boolean) ? 'true' : 'false';
  }
}

/** Render an IN/NOT IN list, validating each member. */
function listSql(list: ListExpr, pos: CompareExpr['pos']): string {
  const parts = list.items.map((it) => {
    if (it.type === 'string') {
      assertSafeInValue(it.value as string, pos);
      return quoteString(it.value as string);
    }
    if (it.type === 'number') return String(it.value);
    return (it.value as boolean) ? 'true' : 'false';
  });
  return `(${parts.join(', ')})`;
}

// ConfigHub uses the same operator spellings FQL does, so the op passes through
// verbatim. (Validation that the op is legal for the column type happens in the
// planner before we get here.)
function opSql(op: CompareOp): string {
  return op;
}

/**
 * Compile one comparison predicate to a ConfigHub clause fragment, given the
 * resolved column (which carries the server-side expression). The column MUST
 * have a pushdown; callers only invoke this for pushable predicates.
 */
export function compileCompare(cmp: CompareExpr, col: ResolvedColumn): string {
  if (!col.pushdown) {
    throw new FqlError(
      `internal: tried to push down non-pushable column "${col.name}"`,
      cmp.pos,
      'compile',
    );
  }
  const field = col.pushdown.expr;

  if (cmp.op === 'IN' || cmp.op === 'NOT IN') {
    if (cmp.right.kind !== 'list') {
      throw new FqlError(`${cmp.op} requires a value list`, cmp.pos, 'compile');
    }
    return `${field} ${opSql(cmp.op)} ${listSql(cmp.right, cmp.pos)}`;
  }

  if (cmp.right.kind !== 'literal') {
    throw new FqlError(`operator ${cmp.op} requires a scalar value`, cmp.pos, 'compile');
  }
  return `${field} ${opSql(cmp.op)} ${literalSql(cmp.right)}`;
}

/** Join AND-group fragments with ConfigHub's only connective. */
export function joinAnd(fragments: string[]): string {
  return fragments.join(' AND ');
}
