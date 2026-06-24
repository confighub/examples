// FQL parser: recursive descent for the statement shape, precedence-climbing
// for WHERE expressions. Grammar (v1):
//
//   select   := SELECT projlist FROM ident
//               [WHERE expr] [GROUP BY collist] [ORDER BY orderlist] [LIMIT int]
//   projlist := '*' | proj (',' proj)*
//   proj     := (agg | column) [AS ident]
//   agg      := AGGFN '(' ('*' | column) ')'
//   expr     := orexpr
//   orexpr   := andexpr (OR andexpr)*
//   andexpr  := notexpr (AND notexpr)*
//   notexpr  := NOT notexpr | primary
//   primary  := '(' expr ')' | predicate
//   predicate:= column ( cmp (literal | list)
//                       | IS [NOT] NULL
//                       | [NOT] IN list
//                       | [NOT] LIKE string | ILIKE string )

import type {
  AggExpr,
  AggFn,
  ColumnExpr,
  CompareExpr,
  CompareOp,
  Expr,
  IsNullExpr,
  ListExpr,
  LiteralExpr,
  OrderKey,
  Projection,
  SelectStmt,
  SortDir,
  StarExpr,
} from './ast';
import { FqlError, type Pos } from './errors';
import { lex, type Token, type TokenType } from './lexer';

const AGG_FNS: ReadonlySet<string> = new Set(['COUNT', 'MAX', 'MIN', 'SUM', 'AVG']);
const CMP_OPS: ReadonlySet<string> = new Set([
  '=',
  '!=',
  '<',
  '>',
  '<=',
  '>=',
  '~',
  '~*',
  '!~',
  '!~*',
]);

/** Span covering two positions (start of a, end of b). */
function span(a: Pos, b: Pos): Pos {
  return { start: a.start, end: b.end, line: a.line, col: a.col };
}

class Parser {
  private toks: Token[];
  private i = 0;

  constructor(src: string) {
    this.toks = lex(src);
  }

  private peek(): Token {
    return this.toks[this.i];
  }
  private next(): Token {
    return this.toks[this.i++];
  }
  private atEof(): boolean {
    return this.peek().type === 'eof';
  }

  /** True if the current token is a keyword equal to `kw`. */
  private isKw(kw: string): boolean {
    const t = this.peek();
    return t.type === 'keyword' && t.value === kw;
  }

  private fail(msg: string, pos?: Pos): never {
    throw new FqlError(msg, pos ?? this.peek().pos, 'parse');
  }

  private expect(type: TokenType, what: string): Token {
    const t = this.peek();
    if (t.type !== type) {
      this.fail(`expected ${what} but found ${describe(t)}`, t.pos);
    }
    return this.next();
  }

  private expectKw(kw: string): Token {
    if (!this.isKw(kw)) {
      this.fail(`expected ${kw} but found ${describe(this.peek())}`, this.peek().pos);
    }
    return this.next();
  }

  // ─── Statement ────────────────────────────────────────────────────────────

  parseSelect(): SelectStmt {
    const startTok = this.expectKw('SELECT');
    const projections = this.parseProjections();
    this.expectKw('FROM');
    const fromTok = this.expect('ident', 'a table name after FROM');
    // Optional table alias: `FROM resources r` or `FROM resources AS r`.
    let alias: string | null = null;
    if (this.isKw('AS')) {
      this.next();
      alias = this.expect('ident', 'an alias after AS').value;
    } else if (this.peek().type === 'ident') {
      alias = this.next().value;
    }
    const from = { name: fromTok.value, alias, pos: fromTok.pos };

    let where: Expr | null = null;
    if (this.isKw('WHERE')) {
      this.next();
      where = this.parseExpr();
    }

    const groupBy: ColumnExpr[] = [];
    if (this.isKw('GROUP')) {
      this.next();
      this.expectKw('BY');
      groupBy.push(this.parseColumn());
      while (this.peek().type === 'comma') {
        this.next();
        groupBy.push(this.parseColumn());
      }
    }

    const orderBy: OrderKey[] = [];
    if (this.isKw('ORDER')) {
      this.next();
      this.expectKw('BY');
      orderBy.push(this.parseOrderKey());
      while (this.peek().type === 'comma') {
        this.next();
        orderBy.push(this.parseOrderKey());
      }
    }

    let limit: number | null = null;
    if (this.isKw('LIMIT')) {
      this.next();
      const n = this.expect('number', 'an integer after LIMIT');
      if (n.value.includes('.')) this.fail('LIMIT must be an integer', n.pos);
      limit = Number.parseInt(n.value, 10);
    }

    if (!this.atEof()) {
      this.fail(`unexpected ${describe(this.peek())} after end of query`, this.peek().pos);
    }

    const endPos = this.toks[this.i - 1]?.pos ?? startTok.pos;
    return {
      kind: 'select',
      projections,
      from,
      where,
      groupBy,
      orderBy,
      limit,
      pos: span(startTok.pos, endPos),
    };
  }

  private parseProjections(): Projection[] {
    // Bare `SELECT *`
    if (this.peek().type === 'star') {
      const star = this.next();
      return [{ expr: { kind: 'star', pos: star.pos } as StarExpr, alias: null, pos: star.pos }];
    }
    const out: Projection[] = [this.parseProjection()];
    while (this.peek().type === 'comma') {
      this.next();
      out.push(this.parseProjection());
    }
    return out;
  }

  private parseProjection(): Projection {
    const expr = this.parseProjExpr();
    let alias: string | null = null;
    if (this.isKw('AS')) {
      this.next();
      alias = this.expect('ident', 'an alias after AS').value;
    }
    return { expr, alias, pos: expr.pos };
  }

  /** Projection expression: an aggregate or a column. */
  private parseProjExpr(): Expr {
    const t = this.peek();
    if (t.type === 'keyword' && AGG_FNS.has(t.value)) {
      return this.parseAgg();
    }
    return this.parseColumn();
  }

  private parseAgg(): AggExpr {
    const fnTok = this.next(); // AGGFN keyword
    this.expect('lparen', `'(' after ${fnTok.value}`);
    let arg: ColumnExpr | StarExpr | null;
    if (this.peek().type === 'star') {
      const star = this.next();
      arg = { kind: 'star', pos: star.pos };
    } else {
      arg = this.parseColumn();
    }
    const close = this.expect('rparen', `')' to close ${fnTok.value}(`);
    return { kind: 'agg', fn: fnTok.value as AggFn, arg, pos: span(fnTok.pos, close.pos) };
  }

  /** Consume a column-name head: an ident, or a keyword reused as a column
   *  (ConfigHub has fields like `From`/`Source` that collide with keywords).
   *  Safe because this is only called where a column is structurally required. */
  private expectColumnHead(): Token {
    const t = this.peek();
    if (t.type === 'ident') return this.next();
    if (t.type === 'keyword') {
      this.next();
      // Re-cast as an ident token carrying the original (cased) source text.
      return { type: 'ident', value: t.raw ?? t.value, pos: t.pos };
    }
    this.fail(`expected a column name but found ${describe(t)}`, t.pos);
  }

  private parseColumn(): ColumnExpr {
    const head = this.expectColumnHead();
    // A column is a head ident followed by zero or more subscripts/continuations:
    //   metadata.annotations['sec-scanner.confighub.com/max-severity']
    //   spec.template.spec.containers[0].image
    // The head (bare or backtick) is a dotted path and splits on dots; only a
    // bracket key is an atomic segment (never re-split on its dots/slash). So
    // `spec.containers.*.image` splits, but ['a.b/c'] stays one segment.
    const path: string[] = head.value.split('.');
    let lastPos = head.pos;

    for (;;) {
      const t = this.peek();
      if (t.type === 'lbracket') {
        this.next();
        const key = this.peek();
        if (key.type === 'string') {
          this.next();
          path.push(key.value); // atomic — keep dots/slashes verbatim
        } else if (key.type === 'number') {
          this.next();
          path.push(key.value); // array index
        } else {
          this.fail(`expected a quoted key or index inside [...] but found ${describe(key)}`, key.pos);
        }
        lastPos = this.expect('rbracket', "']' to close the subscript").pos;
      } else if (t.type === 'dot') {
        this.next();
        const seg = this.peek();
        if (seg.type === 'ident') {
          this.next();
          // A bare ident after a dot may itself be dotted (a.b) — flatten it.
          for (const s of seg.value.split('.')) path.push(s);
          lastPos = seg.pos;
        } else if (seg.type === 'star') {
          this.next();
          path.push('*');
          lastPos = seg.pos;
        } else {
          this.fail(`expected a path segment after '.' but found ${describe(seg)}`, seg.pos);
        }
      } else {
        break;
      }
    }

    return {
      kind: 'column',
      name: pathToName(path),
      path,
      quoted: head.quoted === true,
      pos: span(head.pos, lastPos),
    };
  }

  private parseOrderKey(): OrderKey {
    const expr = this.parseProjExpr();
    let dir: SortDir = 'ASC';
    if (this.isKw('ASC')) {
      this.next();
    } else if (this.isKw('DESC')) {
      this.next();
      dir = 'DESC';
    }
    return { expr, dir, pos: expr.pos };
  }

  // ─── Expressions (precedence climbing) ──────────────────────────────────────

  private parseExpr(): Expr {
    return this.parseOr();
  }

  private parseOr(): Expr {
    let left = this.parseAnd();
    while (this.isKw('OR')) {
      const opTok = this.next();
      const right = this.parseAnd();
      left = { kind: 'logical', op: 'OR', left, right, pos: span(left.pos, right.pos) };
      void opTok;
    }
    return left;
  }

  private parseAnd(): Expr {
    let left = this.parseNot();
    while (this.isKw('AND')) {
      this.next();
      const right = this.parseNot();
      left = { kind: 'logical', op: 'AND', left, right, pos: span(left.pos, right.pos) };
    }
    return left;
  }

  private parseNot(): Expr {
    if (this.isKw('NOT')) {
      const notTok = this.next();
      const expr = this.parseNot();
      return { kind: 'not', expr, pos: span(notTok.pos, expr.pos) };
    }
    return this.parsePrimary();
  }

  private parsePrimary(): Expr {
    if (this.peek().type === 'lparen') {
      this.next();
      const e = this.parseExpr();
      this.expect('rparen', "')' to close the group");
      return e;
    }
    return this.parsePredicate();
  }

  /** A leaf predicate: column followed by an operator clause. */
  private parsePredicate(): Expr {
    const column = this.parseColumn();
    const t = this.peek();

    // IS [NOT] NULL
    if (t.type === 'keyword' && t.value === 'IS') {
      this.next();
      let negated = false;
      if (this.isKw('NOT')) {
        this.next();
        negated = true;
      }
      const nullTok = this.expectKw('NULL');
      const e: IsNullExpr = {
        kind: 'isnull',
        negated,
        column,
        pos: span(column.pos, nullTok.pos),
      };
      return e;
    }

    // [NOT] IN (...) / NOT LIKE
    if (t.type === 'keyword' && t.value === 'NOT') {
      this.next();
      if (this.isKw('IN')) {
        this.next();
        const list = this.parseList();
        return mkCompare('NOT IN', column, list);
      }
      if (this.isKw('LIKE')) {
        this.next();
        const lit = this.parseStringLiteral('NOT LIKE');
        return mkCompare('NOT LIKE', column, lit);
      }
      this.fail(`expected IN or LIKE after NOT but found ${describe(this.peek())}`);
    }

    // IN (...)
    if (t.type === 'keyword' && t.value === 'IN') {
      this.next();
      const list = this.parseList();
      return mkCompare('IN', column, list);
    }

    // LIKE / ILIKE 'pat'
    if (t.type === 'keyword' && (t.value === 'LIKE' || t.value === 'ILIKE')) {
      this.next();
      const lit = this.parseStringLiteral(t.value);
      return mkCompare(t.value as CompareOp, column, lit);
    }

    // Comparison / regex operator. RHS is a literal, or another column for
    // column-to-column comparison (e.g. HeadRevisionNum > LiveRevisionNum).
    // TRUE/FALSE are value literals, not columns, so exclude them here.
    if (t.type === 'op' && CMP_OPS.has(t.value)) {
      this.next();
      const r = this.peek();
      const rhsIsColumn =
        r.type === 'ident' || (r.type === 'keyword' && r.value !== 'TRUE' && r.value !== 'FALSE');
      const rhs = rhsIsColumn ? this.parseColumn() : this.parseValue();
      return mkCompare(t.value as CompareOp, column, rhs);
    }

    this.fail(`expected an operator after column "${column.name}" but found ${describe(t)}`, t.pos);
  }

  private parseValue(): LiteralExpr {
    const t = this.peek();
    if (t.type === 'string') {
      this.next();
      return { kind: 'literal', value: t.value, type: 'string', pos: t.pos };
    }
    if (t.type === 'number') {
      this.next();
      return {
        kind: 'literal',
        value: Number(t.value),
        type: 'number',
        pos: t.pos,
      };
    }
    if (t.type === 'keyword' && (t.value === 'TRUE' || t.value === 'FALSE')) {
      this.next();
      return { kind: 'literal', value: t.value === 'TRUE', type: 'boolean', pos: t.pos };
    }
    this.fail(`expected a literal value but found ${describe(t)}`, t.pos);
  }

  private parseStringLiteral(after: string): LiteralExpr {
    const t = this.peek();
    if (t.type !== 'string') {
      this.fail(`expected a quoted pattern after ${after} but found ${describe(t)}`, t.pos);
    }
    this.next();
    return { kind: 'literal', value: t.value, type: 'string', pos: t.pos };
  }

  private parseList(): ListExpr {
    const open = this.expect('lparen', "'(' to start a value list");
    const items: LiteralExpr[] = [this.parseValue()];
    while (this.peek().type === 'comma') {
      this.next();
      items.push(this.parseValue());
    }
    const close = this.expect('rparen', "')' to close the value list");
    return { kind: 'list', items, pos: span(open.pos, close.pos) };
  }
}

function mkCompare(
  op: CompareOp,
  left: ColumnExpr,
  right: LiteralExpr | ListExpr | ColumnExpr,
): CompareExpr {
  return { kind: 'compare', op, left, right, pos: span(left.pos, right.pos) };
}

/** Reconstruct a readable column name from path segments, bracket-quoting any
 *  segment that isn't a plain dotted identifier (contains a dot, slash, etc.). */
function pathToName(path: string[]): string {
  let out = '';
  for (const seg of path) {
    const simple = /^[A-Za-z0-9_*]+$/.test(seg);
    if (out === '') out += simple ? seg : `['${seg}']`;
    else out += simple ? `.${seg}` : `['${seg}']`;
  }
  return out;
}

function describe(t: Token): string {
  switch (t.type) {
    case 'eof':
      return 'end of query';
    case 'string':
      return `string '${t.value}'`;
    case 'keyword':
      return `keyword ${t.value}`;
    default:
      return `"${t.value}"`;
  }
}

/** Parse a query string into a SelectStmt, or throw FqlError. */
export function parse(src: string): SelectStmt {
  return new Parser(src).parseSelect();
}
