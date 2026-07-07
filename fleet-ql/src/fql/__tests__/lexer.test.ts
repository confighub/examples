import { describe, expect, it } from 'vitest';

import { FqlError } from '../errors';
import { lex, type Token } from '../lexer';

/** Compact view of tokens for assertions: "type:value". */
const shape = (src: string): string[] =>
  lex(src)
    .filter((t) => t.type !== 'eof')
    .map((t: Token) => `${t.type}:${t.value}`);

describe('lexer', () => {
  it('tokenizes a basic SELECT', () => {
    expect(shape("SELECT slug FROM units WHERE space = 'prod'")).toEqual([
      'keyword:SELECT',
      'ident:slug',
      'keyword:FROM',
      'ident:units',
      'keyword:WHERE',
      'ident:space',
      'op:=',
      'string:prod',
    ]);
  });

  it('keywords are case-insensitive and upper-cased', () => {
    expect(shape('select Slug from Units')).toEqual([
      'keyword:SELECT',
      'ident:Slug',
      'keyword:FROM',
      'ident:Units',
    ]);
  });

  it('keeps dotted identifiers as one ident (not keyword)', () => {
    // "in" inside a path must not become the IN keyword.
    expect(shape('labels.env')).toEqual(['ident:labels.env']);
    expect(shape('metadata.name')).toEqual(['ident:metadata.name']);
  });

  it('lexes all comparison/regex operators', () => {
    expect(shape('= != < > <= >= ~ ~* !~ !~*')).toEqual([
      'op:=',
      'op:!=',
      'op:<',
      'op:>',
      'op:<=',
      'op:>=',
      'op:~',
      'op:~*',
      'op:!~',
      'op:!~*',
    ]);
  });

  it("decodes '' escaping in strings", () => {
    const toks = lex("'it''s'");
    expect(toks[0]).toMatchObject({ type: 'string', value: "it's" });
  });

  it('lexes integers and decimals', () => {
    expect(shape('1 42 3.14')).toEqual(['number:1', 'number:42', 'number:3.14']);
  });

  it('skips -- line comments', () => {
    expect(shape('SELECT slug -- a comment\nFROM units')).toEqual([
      'keyword:SELECT',
      'ident:slug',
      'keyword:FROM',
      'ident:units',
    ]);
  });

  it('handles star, parens, comma', () => {
    expect(shape('COUNT(*), x')).toEqual([
      'keyword:COUNT',
      'lparen:(',
      'star:*',
      'rparen:)',
      'comma:,',
      'ident:x',
    ]);
  });

  it('tracks line and column positions', () => {
    const toks = lex('SELECT\n  slug');
    const slug = toks.find((t) => t.value === 'slug')!;
    expect(slug.pos).toMatchObject({ line: 2, col: 3 });
  });

  it('throws on an unterminated string with a position', () => {
    try {
      lex("SELECT 'oops");
      expect.unreachable('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(FqlError);
      expect((e as FqlError).pos?.line).toBe(1);
    }
  });

  it('throws on an illegal character', () => {
    expect(() => lex('a & b')).toThrow(FqlError);
  });

  it('lexes a backtick-quoted path verbatim (with * / -)', () => {
    const toks = lex('`spec.template.spec.containers.*.image`');
    expect(toks[0]).toMatchObject({
      type: 'ident',
      value: 'spec.template.spec.containers.*.image',
      quoted: true,
    });
    const anno = lex('`metadata.annotations.sec-scanner.confighub.com/max-severity`');
    expect(anno[0].value).toBe('metadata.annotations.sec-scanner.confighub.com/max-severity');
  });

  it('does not keyword-match inside a backtick ident', () => {
    // `from` would normally be the FROM keyword; quoted it stays an ident.
    expect(lex('`from`')[0]).toMatchObject({ type: 'ident', value: 'from', quoted: true });
  });

  it('throws on an unterminated backtick ident', () => {
    expect(() => lex('`spec.replicas')).toThrow(FqlError);
  });
});
