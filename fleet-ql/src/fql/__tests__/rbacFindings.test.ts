import { describe, expect, it } from 'vitest';

import type { Row } from '../evaluate';
import { planQuery, runQuery } from '../index';
import type { ListParams, Transport } from '../transport';

describe('rbac_findings — planner', () => {
  it('pushes space to where; analyzer/severity are client-side residual', () => {
    const p = planQuery("SELECT resourceName FROM rbac_findings WHERE space = 'rbac-demo-prod' AND severity = 'high'");
    expect(p.source).toBe('rbac_findings');
    expect(p.fetches).toEqual([{ where: "Space.Slug = 'rbac-demo-prod'" }]);
    expect(p.residual).not.toBeNull();
  });
});

const FINDINGS: Row[] = [
  { __id: 'wildcard-rules:prod:ClusterRole::legacy-admin', analyzer: 'wildcard-rules', severity: 'high', cluster: 'prod', space: 'p', resourceKind: 'ClusterRole', resourceName: 'legacy-admin', namespace: null, message: 'Wildcard permissions.' },
  { __id: 'unbound-service-accounts:prod:ServiceAccount:web:idle', analyzer: 'unbound-service-accounts', severity: 'low', cluster: 'prod', space: 'p', resourceKind: 'ServiceAccount', resourceName: 'idle', namespace: 'web', message: 'No bindings.' },
];

function mockTransport(): Transport & { findingCalls: ListParams[] } {
  const t = {
    findingCalls: [] as ListParams[],
    async units() { return []; },
    async resources() { return []; },
    async spaces() { return []; },
    async revisions() { return []; },
    async grants() { return []; },
    async roles() { return []; },
    async bindings() { return []; },
    async rbacFindings(p: ListParams) {
      t.findingCalls.push(p);
      return FINDINGS.map((r) => ({ ...r }));
    },
  };
  return t;
}

describe('rbac_findings — end to end', () => {
  it('filters by severity client-side and counts by analyzer', async () => {
    const t = mockTransport();
    const res = await runQuery(
      "SELECT analyzer, COUNT(*) AS n FROM rbac_findings WHERE severity = 'high' GROUP BY analyzer",
      t,
    );
    expect(res.rows).toEqual([{ analyzer: 'wildcard-rules', n: 1 }]);
  });

  it('forwards the space scope to the materializer', async () => {
    const t = mockTransport();
    await runQuery("SELECT resourceName FROM rbac_findings WHERE space = 'p'", t);
    expect(t.findingCalls[0].where).toBe("Space.Slug = 'p'");
  });
});
