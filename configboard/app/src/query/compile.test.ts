import { describe, expect, it } from 'vitest';

import type { Panel } from '../model/types';
import { ALL_VALUE, compilePanel, cubCommand, requestKey, substitute, windowStart } from './compile';

const NOW = Date.parse('2026-07-24T12:00:00Z');

describe('substitute', () => {
  it('fills a variable into its conjunct', () => {
    expect(substitute("Labels.Environment = '${env}'", { env: 'prod' }, NOW)).toBe(
      "Labels.Environment = 'prod'",
    );
  });

  it('drops the conjunct when the variable is All, rather than matching the literal', () => {
    const where = "Space.Labels.Environment = '${env}' AND Target.Slug = '${cluster}'";
    expect(substitute(where, { env: ALL_VALUE, cluster: 'prod-1' }, NOW)).toBe(
      "Target.Slug = 'prod-1'",
    );
  });

  it('returns undefined when every conjunct drops out', () => {
    expect(substitute("Labels.Environment = '${env}'", { env: ALL_VALUE }, NOW)).toBeUndefined();
  });

  it('drops the conjunct when the variable is unset', () => {
    expect(substitute("Labels.Environment = '${env}'", {}, NOW)).toBeUndefined();
  });

  it('resolves a time window to an absolute timestamp', () => {
    const out = substitute("CreatedAt > '${window.start}'", { window: '7d' }, NOW);
    expect(out).toBe(`CreatedAt > '${windowStart('7d', NOW)}'`);
    expect(out).toContain('2026-07-17');
  });

  it('does not split on AND inside a quoted literal', () => {
    const where = "Slug = 'black AND white' AND Labels.Environment = '${env}'";
    expect(substitute(where, { env: 'dev' }, NOW)).toBe(
      "Slug = 'black AND white' AND Labels.Environment = 'dev'",
    );
  });

  it('keeps a time bound even when the variable is unset', () => {
    // Dropping a `.start` conjunct turns "last 30 days" into "everything ever" — on a
    // paginationless endpoint that is the query that hangs the tab. Seen for real when
    // a dashboard switch left the window variable uninitialized.
    const out = substitute("CreatedAt > '${window.start}'", {}, NOW);
    expect(out).toBe(`CreatedAt > '${windowStart('30d', NOW)}'`);
  });

  it('keeps a time bound when the variable is All', () => {
    const out = substitute("CreatedAt > '${window.start}'", { window: ALL_VALUE }, NOW);
    expect(out).toBe(`CreatedAt > '${windowStart('30d', NOW)}'`);
  });

  it('still drops a non-time conjunct alongside a kept time bound', () => {
    const out = substitute(
      "CreatedAt > '${window.start}' AND Space.Labels.Environment = '${env}'",
      { window: '7d', env: ALL_VALUE },
      NOW,
    );
    expect(out).toBe(`CreatedAt > '${windowStart('7d', NOW)}'`);
  });

  it('leaves literal-only expressions alone', () => {
    expect(substitute("Source = 'RestoreRevision'", {}, NOW)).toBe("Source = 'RestoreRevision'");
  });
});

const panel = (over: Partial<Panel> = {}): Panel => ({
  id: 'p',
  title: 'p',
  query: { source: 'Unit' },
  chart: { form: 'bar' },
  ...over,
});

describe('compilePanel', () => {
  it('adds the include a joined dimension needs', () => {
    const spec = compilePanel(
      panel({ transform: { groupBy: 'Target.Slug', aggregate: { fn: 'count' } } }),
      {},
      NOW,
    );
    expect(spec.include).toContain('TargetID');
  });

  it('adds the include a joined where term needs', () => {
    const spec = compilePanel(
      panel({ query: { source: 'Unit', where: "Space.Labels.Environment = '${env}'" } }),
      { env: 'prod' },
      NOW,
    );
    expect(spec.include).toContain('SpaceID');
  });

  it('does not include a joined entity when its clause dropped out', () => {
    const spec = compilePanel(
      panel({ query: { source: 'Unit', where: "Target.Slug = '${cluster}'" } }),
      { cluster: ALL_VALUE },
      NOW,
    );
    expect(spec.where).toBeUndefined();
    expect(spec.include).toBeUndefined();
  });

  it('never selects the config body on a unit query', () => {
    const spec = compilePanel(panel(), {}, NOW);
    expect(spec.select).toBeDefined();
    expect(spec.select).not.toContain('Data');
    expect(spec.select).not.toContain('LiveState');
  });

  it('omits select when a view dictates the projection', () => {
    const spec = compilePanel(panel({ query: { source: 'Unit', view: 'configboard/kinds' } }), {}, NOW);
    expect(spec.select).toBeUndefined();
    expect(spec.view).toBe('configboard/kinds');
  });

  it('asks for summary counts on a space query', () => {
    const spec = compilePanel(panel({ query: { source: 'Space' } }), {}, NOW);
    expect(spec.summary).toBe(true);
  });

  it('treats Space fields as native on a space query, not a join', () => {
    const spec = compilePanel(
      panel({ query: { source: 'Space', where: "Labels.Environment = 'prod'" } }),
      {},
      NOW,
    );
    expect(spec.include).toBeUndefined();
  });

  it('gives panels with identical requests the same cache key', () => {
    const a = compilePanel(panel({ id: 'a', query: { source: 'Space' } }), {}, NOW);
    const b = compilePanel(panel({ id: 'b', query: { source: 'Space' } }), {}, NOW);
    expect(requestKey(a)).toBe(requestKey(b));
  });
});

describe('cubCommand', () => {
  it('renders the equivalent cub invocation', () => {
    const spec = compilePanel(
      panel({ query: { source: 'Unit', where: "Space.Labels.Environment = '${env}'" } }),
      { env: 'prod' },
      NOW,
    );
    expect(cubCommand(spec)).toBe(
      `cub unit list --space "*" --where "Space.Labels.Environment = 'prod'"`,
    );
  });
});
