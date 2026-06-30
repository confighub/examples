import { describe, expect, it } from 'vitest';

import { explainPlan, formatExpr } from '../explain';
import { parse } from '../parser';
import { plan } from '../planner';

const ex = (q: string) => explainPlan(plan(parse(q)));
const titles = (stages: { title: string }[]) => stages.map((s) => s.title);

describe('formatExpr', () => {
  it('renders predicates back to readable FQL', () => {
    const stmt = parse("SELECT slug FROM units WHERE space = 'p' AND headRevisionNum > liveRevisionNum");
    expect(formatExpr(stmt.where!)).toBe("(space = 'p' AND headRevisionNum > liveRevisionNum)");
  });
});

describe('explainPlan — single table', () => {
  it('shows the generated-SDK call with its args, filter, and project', () => {
    const e = ex("SELECT unit, kind FROM resources WHERE space = 'sec-demo-dev' AND kind = 'Deployment'");
    expect(e.inputs).toHaveLength(1);
    expect(e.inputs[0].title).toBe('scan resources');
    const call = e.inputs[0].lines.join('\n');
    expect(call).toContain("cub.POST('/function/invoke'");
    expect(call).toContain('FunctionInvocations: [get-resources]');
    expect(call).toContain('where: "Space.Slug = \'sec-demo-dev\'"');
    expect(call).toContain('where_data: "kind = \'Deployment\'"');
    expect(titles(e.pipeline)).toEqual(['filter · full WHERE (client-side)', 'project · SELECT']);
  });

  it('shows union + de-dupe for an OR that splits into two fetches', () => {
    const e = ex("SELECT unit FROM resources WHERE kind = 'Service' OR kind = 'Ingress'");
    // kind = ... OR kind = ... → two where_data fetches.
    expect(titles(e.pipeline)).toContain('union + de-dupe · OR branches');
  });

  it('shows group + aggregate / order / limit stages', () => {
    const e = ex('SELECT space, COUNT(*) AS n FROM units GROUP BY space ORDER BY n DESC LIMIT 5');
    expect(e.inputs[0].lines.join('\n')).toContain("cub.GET('/unit'");
    expect(titles(e.pipeline)).toEqual([
      'group + aggregate',
      'order by',
      'limit 5',
      'project · SELECT',
    ]);
  });
});

describe('explainPlan — join', () => {
  it('shows both scans as inputs and a hash-join stage with the ON keys', () => {
    const e = ex(
      "SELECT d.component AS c FROM resources d JOIN resources p ON d.name = p.name AND d.kind = p.kind " +
        "WHERE d.space LIKE 'acme-%' AND p.environment = 'Prod'",
    );
    expect(titles(e.inputs)).toEqual(['scan resources d', 'scan resources p']);
    expect(e.inputs[0].lines.join('\n')).toContain('where: "Space.Slug LIKE \'acme-%\'"');
    const join = e.pipeline.find((s) => s.title.startsWith('hash join'));
    expect(join?.title).toBe('hash join · inner');
    expect(join?.lines).toEqual([
      'on d.name = p.name',
      'on d.kind = p.kind',
      'unmatched rows dropped',
    ]);
    expect(titles(e.pipeline)).toContain('filter · full WHERE (client-side)');
  });
});
