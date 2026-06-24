import { describe, expect, it } from 'vitest';

import { evaluate, evalPredicate, type Row } from '../evaluate';
import { parse } from '../parser';

const ROWS: Row[] = [
  { unit: 'legacy-frontend', space: 'sec-demo-dev', image: 'nginx:1.16-alpine', severity: 'CRITICAL', cveCount: 13 },
  { unit: 'legacy-api', space: 'sec-demo-dev', image: 'python:3.7-alpine3.10', severity: 'CRITICAL', cveCount: 4 },
  { unit: 'unpinned-web', space: 'sec-demo-dev', image: 'nginx:latest', severity: 'MEDIUM', cveCount: 2 },
  { unit: 'frontend', space: 'sec-demo-prod', image: 'nginx:1.27-alpine', severity: 'NONE', cveCount: 0 },
  { unit: 'api', space: 'sec-demo-prod', image: 'python:3.12-alpine', severity: 'NONE', cveCount: 0 },
];

const cols = ['unit', 'space', 'image', 'severity', 'cveCount'];
const run = (q: string) => evaluate(parse(q), ROWS, cols);

describe('evaluate — filter', () => {
  it('filters by equality', () => {
    const r = run("SELECT unit FROM resources WHERE severity = 'CRITICAL'");
    expect(r.rows.map((x) => x.unit)).toEqual(['legacy-frontend', 'legacy-api']);
  });

  it('filters by OR', () => {
    const r = run("SELECT unit FROM resources WHERE severity = 'CRITICAL' OR image ~ ':latest'");
    expect(r.rows.map((x) => x.unit).sort()).toEqual(['legacy-api', 'legacy-frontend', 'unpinned-web']);
  });

  it('filters by numeric comparison', () => {
    const r = run('SELECT unit FROM resources WHERE cveCount > 2');
    expect(r.rows.map((x) => x.unit)).toEqual(['legacy-frontend', 'legacy-api']);
  });

  it('filters by IN', () => {
    const r = run("SELECT unit FROM resources WHERE space IN ('sec-demo-prod')");
    expect(r.rows.map((x) => x.unit)).toEqual(['frontend', 'api']);
  });

  it('regex matches :latest', () => {
    const r = run("SELECT unit FROM resources WHERE image ~ ':latest$'");
    expect(r.rows.map((x) => x.unit)).toEqual(['unpinned-web']);
  });

  it('evalPredicate works standalone', () => {
    const stmt = parse("SELECT unit FROM resources WHERE cveCount >= 4");
    expect(evalPredicate(stmt.where!, ROWS[0])).toBe(true);
    expect(evalPredicate(stmt.where!, ROWS[2])).toBe(false);
  });

  it('evaluates column-to-column comparison (drift idiom)', () => {
    // head > live means "has unapplied changes" — true for both a partially
    // applied unit and a never-applied one (live=0); equal means synced.
    const rows: Row[] = [
      { slug: 'drifted', HeadRevisionNum: 8, LiveRevisionNum: 5 },
      { slug: 'synced', HeadRevisionNum: 5, LiveRevisionNum: 5 },
      { slug: 'never', HeadRevisionNum: 3, LiveRevisionNum: 0 },
    ];
    const q = 'SELECT slug FROM units WHERE HeadRevisionNum > LiveRevisionNum';
    const r = evaluate(parse(q), rows, ['slug']);
    expect(r.rows.map((x) => x.slug).sort()).toEqual(['drifted', 'never']);
  });
});

describe('evaluate — raw YAML paths over __doc (array wildcard, existential)', () => {
  // Rows carry the raw resource doc under __doc for data-path traversal.
  const docRows: Row[] = [
    {
      unit: 'multi',
      __doc: {
        kind: 'Deployment',
        spec: {
          replicas: 3,
          template: { spec: { containers: [{ image: 'nginx:1.27-alpine' }, { image: 'busybox:latest' }] } },
        },
      },
    },
    {
      unit: 'single',
      __doc: {
        kind: 'Deployment',
        spec: { replicas: 1, template: { spec: { containers: [{ image: 'redis:7.2-alpine' }] } } },
      },
    },
  ];
  const run = (q: string) => evaluate(parse(q), docRows, ['unit']);

  it('matches existentially when ANY array element matches (* wildcard)', () => {
    const r = run("SELECT unit FROM resources WHERE `spec.template.spec.containers.*.image` ~ ':latest'");
    expect(r.rows.map((x) => x.unit)).toEqual(['multi']); // only multi has a :latest container
  });

  it('traverses a scalar data path with a numeric comparison', () => {
    const r = run('SELECT unit FROM resources WHERE `spec.replicas` > 1');
    expect(r.rows.map((x) => x.unit)).toEqual(['multi']);
  });

  it('indexes a specific array element', () => {
    const r = run("SELECT unit FROM resources WHERE `spec.template.spec.containers.0.image` LIKE 'nginx%'");
    expect(r.rows.map((x) => x.unit)).toEqual(['multi']);
  });

  it('resolves an alias-qualified bare path via __doc traversal', () => {
    // `d.kind` → strip alias → "kind"; not a flat field, found in __doc.
    const r = run("SELECT unit FROM resources d WHERE d.kind = 'Deployment'");
    expect(r.rows.map((x) => x.unit).sort()).toEqual(['multi', 'single']);
  });

  it('reads a bracket-keyed annotation (dots/slash key) via traversal', () => {
    const annoRows: Row[] = [
      {
        unit: 'crit',
        __doc: {
          kind: 'Deployment',
          metadata: { annotations: { 'sec-scanner.confighub.com/max-severity': 'CRITICAL' } },
        },
      },
      {
        unit: 'clean',
        __doc: {
          kind: 'Deployment',
          metadata: { annotations: { 'sec-scanner.confighub.com/max-severity': 'NONE' } },
        },
      },
    ];
    const q =
      "SELECT unit FROM resources WHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'";
    const r = evaluate(parse(q), annoRows, ['unit']);
    expect(r.rows.map((x) => x.unit)).toEqual(['crit']);
  });

  it('projects a bracket-keyed annotation with an alias', () => {
    const annoRows: Row[] = [
      {
        unit: 'a',
        __doc: { metadata: { annotations: { 'example.com/team': 'payments' } } },
      },
    ];
    const q = "SELECT unit, metadata.annotations['example.com/team'] AS team FROM resources";
    const r = evaluate(parse(q), annoRows, ['unit']);
    expect(r.columns).toEqual(['unit', 'team']);
    expect(r.rows[0]).toEqual({ unit: 'a', team: 'payments' });
  });
});

describe('evaluate — projection / order / limit', () => {
  it('projects selected columns only', () => {
    const r = run("SELECT unit, image FROM resources WHERE severity = 'MEDIUM'");
    expect(r.columns).toEqual(['unit', 'image']);
    expect(r.rows[0]).toEqual({ unit: 'unpinned-web', image: 'nginx:latest' });
  });

  it('SELECT * returns all fetched columns', () => {
    const r = run("SELECT * FROM resources WHERE unit = 'api'");
    expect(r.columns).toEqual(cols);
    expect(r.rows[0].cveCount).toBe(0);
  });

  it('applies aliases', () => {
    const r = run("SELECT unit AS u FROM resources WHERE unit = 'api'");
    expect(r.columns).toEqual(['u']);
    expect(r.rows[0]).toEqual({ u: 'api' });
  });

  it('orders DESC and limits', () => {
    const r = run('SELECT unit, cveCount FROM resources ORDER BY cveCount DESC LIMIT 2');
    expect(r.rows.map((x) => x.cveCount)).toEqual([13, 4]);
  });

  it('orders ASC by string', () => {
    const r = run('SELECT unit FROM resources ORDER BY unit ASC LIMIT 1');
    expect(r.rows[0].unit).toBe('api');
  });
});

describe('evaluate — group by + aggregates', () => {
  it('counts rows per group', () => {
    const r = run('SELECT space, COUNT(*) AS n FROM resources GROUP BY space');
    const bySpace = Object.fromEntries(r.rows.map((x) => [x.space, x.n]));
    expect(bySpace).toEqual({ 'sec-demo-dev': 3, 'sec-demo-prod': 2 });
  });

  it('counts only criticals per group with a filter', () => {
    const r = run(
      "SELECT space, COUNT(*) AS n FROM resources WHERE severity = 'CRITICAL' GROUP BY space",
    );
    expect(r.rows).toEqual([{ space: 'sec-demo-dev', n: 2 }]);
  });

  it('MAX and SUM aggregate', () => {
    const r = run('SELECT MAX(cveCount) AS m, SUM(cveCount) AS s FROM resources');
    expect(r.rows[0]).toEqual({ m: 13, s: 19 });
  });

  it('orders by an aggregate alias', () => {
    const r = run('SELECT space, COUNT(*) AS n FROM resources GROUP BY space ORDER BY n DESC');
    expect(r.rows.map((x) => x.space)).toEqual(['sec-demo-dev', 'sec-demo-prod']);
  });
});
