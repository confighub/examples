// FQL tokenizer. Produces a flat token stream with source positions; the parser
// consumes it. Whitespace and `--` line comments are skipped. Keywords are
// case-insensitive (SELECT == select) but identifiers preserve their case.

import { FqlError, type Pos } from './errors';

export type TokenType =
  | 'keyword'
  | 'ident' // column / table name (may be dotted)
  | 'string' // 'literal'
  | 'number' // 42, 3.14
  | 'op' // = != < > <= >= ~ ~* !~ !~* etc.
  | 'comma'
  | 'lparen'
  | 'rparen'
  | 'star'
  | 'eof';

export interface Token {
  type: TokenType;
  /** Raw text. For keywords this is upper-cased; for strings it's the decoded value. */
  value: string;
  /** True for backtick-quoted idents — a verbatim path that bypasses keyword
   *  matching and may contain `*`, `/`, `-`, etc. (e.g. a YAML data path). */
  quoted?: boolean;
  pos: Pos;
}

// Reserved words. Stored upper-case; matched case-insensitively.
const KEYWORDS = new Set([
  'SELECT',
  'FROM',
  'WHERE',
  'GROUP',
  'BY',
  'ORDER',
  'LIMIT',
  'AS',
  'AND',
  'OR',
  'NOT',
  'IN',
  'IS',
  'NULL',
  'LIKE',
  'ILIKE',
  'ASC',
  'DESC',
  'TRUE',
  'FALSE',
  'COUNT',
  'MAX',
  'MIN',
  'SUM',
  'AVG',
]);

const isDigit = (c: string) => c >= '0' && c <= '9';
const isIdentStart = (c: string) => /[A-Za-z_]/.test(c);
const isIdentPart = (c: string) => /[A-Za-z0-9_.]/.test(c); // dots allowed for paths

/** Tokenize a query. Throws FqlError on an illegal character or unterminated string. */
export function lex(src: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  let line = 1;
  let lineStart = 0; // offset of the current line's first char

  const posAt = (start: number, end: number): Pos => ({
    start,
    end,
    line,
    col: start - lineStart + 1,
  });

  const fail = (msg: string, start: number, end: number): never => {
    throw new FqlError(msg, posAt(start, end), 'lex');
  };

  while (i < src.length) {
    const c = src[i];

    // Newline — track line/col.
    if (c === '\n') {
      i++;
      line++;
      lineStart = i;
      continue;
    }
    // Other whitespace.
    if (c === ' ' || c === '\t' || c === '\r') {
      i++;
      continue;
    }
    // Line comment: -- to end of line.
    if (c === '-' && src[i + 1] === '-') {
      while (i < src.length && src[i] !== '\n') i++;
      continue;
    }

    const start = i;

    // String literal with '' escaping.
    if (c === "'") {
      i++; // opening quote
      let value = '';
      let closed = false;
      while (i < src.length) {
        if (src[i] === "'") {
          if (src[i + 1] === "'") {
            value += "'";
            i += 2;
            continue;
          }
          i++; // closing quote
          closed = true;
          break;
        }
        if (src[i] === '\n') break; // strings don't span lines
        value += src[i];
        i++;
      }
      if (!closed) fail('unterminated string literal', start, i);
      tokens.push({ type: 'string', value, pos: posAt(start, i) });
      continue;
    }

    // Backtick-quoted identifier: a verbatim path (any char but backtick).
    // Lets a column be a full YAML data path with `*`, `/`, `-`, dots, etc.,
    // e.g. `spec.template.spec.containers.*.image`.
    if (c === '`') {
      i++; // opening backtick
      let value = '';
      let closed = false;
      while (i < src.length) {
        if (src[i] === '`') {
          i++; // closing backtick
          closed = true;
          break;
        }
        if (src[i] === '\n') break; // quoted idents don't span lines
        value += src[i];
        i++;
      }
      if (!closed) fail('unterminated backtick-quoted identifier', start, i);
      if (value === '') fail('empty backtick-quoted identifier', start, i);
      tokens.push({ type: 'ident', value, quoted: true, pos: posAt(start, i) });
      continue;
    }

    // Number (integer or decimal). No leading-dot, no signs (unary handled elsewhere if needed).
    if (isDigit(c)) {
      i++;
      while (i < src.length && isDigit(src[i])) i++;
      if (src[i] === '.' && isDigit(src[i + 1])) {
        i++;
        while (i < src.length && isDigit(src[i])) i++;
      }
      tokens.push({ type: 'number', value: src.slice(start, i), pos: posAt(start, i) });
      continue;
    }

    // Identifier / keyword (dotted paths allowed: labels.env).
    if (isIdentStart(c)) {
      i++;
      while (i < src.length && isIdentPart(src[i])) i++;
      const text = src.slice(start, i);
      const upper = text.toUpperCase();
      if (KEYWORDS.has(upper) && !text.includes('.')) {
        tokens.push({ type: 'keyword', value: upper, pos: posAt(start, i) });
      } else {
        tokens.push({ type: 'ident', value: text, pos: posAt(start, i) });
      }
      continue;
    }

    // Operators & punctuation.
    switch (c) {
      case ',':
        tokens.push({ type: 'comma', value: ',', pos: posAt(start, i + 1) });
        i++;
        continue;
      case '(':
        tokens.push({ type: 'lparen', value: '(', pos: posAt(start, i + 1) });
        i++;
        continue;
      case ')':
        tokens.push({ type: 'rparen', value: ')', pos: posAt(start, i + 1) });
        i++;
        continue;
      case '*':
        tokens.push({ type: 'star', value: '*', pos: posAt(start, i + 1) });
        i++;
        continue;
      case '=':
        tokens.push({ type: 'op', value: '=', pos: posAt(start, i + 1) });
        i++;
        continue;
      case '<':
        if (src[i + 1] === '=') {
          tokens.push({ type: 'op', value: '<=', pos: posAt(start, i + 2) });
          i += 2;
        } else {
          tokens.push({ type: 'op', value: '<', pos: posAt(start, i + 1) });
          i++;
        }
        continue;
      case '>':
        if (src[i + 1] === '=') {
          tokens.push({ type: 'op', value: '>=', pos: posAt(start, i + 2) });
          i += 2;
        } else {
          tokens.push({ type: 'op', value: '>', pos: posAt(start, i + 1) });
          i++;
        }
        continue;
      case '!':
        // != or !~ or !~*
        if (src[i + 1] === '=') {
          tokens.push({ type: 'op', value: '!=', pos: posAt(start, i + 2) });
          i += 2;
        } else if (src[i + 1] === '~') {
          if (src[i + 2] === '*') {
            tokens.push({ type: 'op', value: '!~*', pos: posAt(start, i + 3) });
            i += 3;
          } else {
            tokens.push({ type: 'op', value: '!~', pos: posAt(start, i + 2) });
            i += 2;
          }
        } else {
          fail(`unexpected character '!'`, start, i + 1);
        }
        continue;
      case '~':
        if (src[i + 1] === '*') {
          tokens.push({ type: 'op', value: '~*', pos: posAt(start, i + 2) });
          i += 2;
        } else {
          tokens.push({ type: 'op', value: '~', pos: posAt(start, i + 1) });
          i++;
        }
        continue;
      default:
        fail(`unexpected character '${c}'`, start, i + 1);
    }
  }

  tokens.push({ type: 'eof', value: '', pos: posAt(i, i) });
  return tokens;
}
