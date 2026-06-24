// FQL error type and a caret-renderer for the REPL. Every lexer/parser/planner
// failure carries a source position so the console can point at the offending
// token instead of just saying "syntax error".

/** A character span in the source query (0-based offsets, 1-based line/col). */
export interface Pos {
  /** 0-based offset of the first character. */
  start: number;
  /** 0-based offset just past the last character. */
  end: number;
  /** 1-based line number. */
  line: number;
  /** 1-based column number. */
  col: number;
}

/** An error raised while lexing, parsing, planning, or compiling a query. */
export class FqlError extends Error {
  readonly pos: Pos | null;
  /** Which phase produced the error — useful for the UI to label it. */
  readonly phase: 'lex' | 'parse' | 'plan' | 'compile' | 'run';

  constructor(message: string, pos: Pos | null, phase: FqlError['phase'] = 'parse') {
    super(message);
    this.name = 'FqlError';
    this.pos = pos;
    this.phase = phase;
    // Restore prototype chain (TS target < ES2015 interop safety).
    Object.setPrototypeOf(this, FqlError.prototype);
  }
}

/**
 * Render a query + error as a two-line excerpt with a caret under the offending
 * span, e.g.:
 *
 *   SELECT slug FORM units
 *               ^^^^
 *   unexpected keyword "FORM" at line 1, col 13
 *
 * Falls back to just the message when there is no position. The caret line
 * spans the error token (at least one `^`).
 */
export function renderError(query: string, err: FqlError): string {
  if (!err.pos) return err.message;
  const { start, end, line, col } = err.pos;

  // Extract the source line containing `start`.
  const lines = query.split('\n');
  const srcLine = lines[line - 1] ?? '';

  // Caret width = span length clamped to the rest of the line, min 1.
  const width = Math.max(1, Math.min(end - start, srcLine.length - (col - 1)) || 1);
  const caret = ' '.repeat(Math.max(0, col - 1)) + '^'.repeat(width);

  return `${srcLine}\n${caret}\n${err.message} (line ${line}, col ${col})`;
}
