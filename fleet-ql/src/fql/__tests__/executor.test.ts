import { describe, expect, it, vi } from 'vitest';

import type { Row } from '../evaluate';
import { runQuery } from '../index';
import type { ResourceParams, Transport } from '../transport';

// A fixture fleet of resources, each tagged so we can assert pushdown narrowing.
const RESOURCES: (Row & { severity: string })[] = [
  { space: 'sec-demo-dev', unit: 'legacy-frontend', resourceType: 'apps/v1/Deployment', name: 'legacy-frontend', kind: 'Deployment', image: 'nginx:1.16-alpine', severity: 'CRITICAL', cveCount: 13 },
  { space: 'sec-demo-dev', unit: 'legacy-api', resourceType: 'apps/v1/Deployment', name: 'legacy-api', kind: 'Deployment', image: 'python:3.7-alpine3.10', severity: 'CRITICAL', cveCount: 4 },
  { space: 'sec-demo-dev', unit: 'unpinned-web', resourceType: 'apps/v1/Deployment', name: 'unpinned-web', kind: 'Deployment', image: 'nginx:latest', severity: 'MEDIUM', cveCount: 2 },
  { space: 'sec-demo-prod', unit: 'frontend', resourceType: 'apps/v1/Deployment', name: 'frontend', kind: 'Deployment', image: 'nginx:1.27-alpine', severity: 'NONE', cveCount: 0 },
];

/** A mock transport that simulates server-side pushdown so we can verify both
 *  (a) the executor unions OR branches and (b) pushdown actually narrows. */
function mockTransport(): Transport & { calls: ResourceParams[] } {
  const calls: ResourceParams[] = [];
  return {
    calls,
    async units() {
      return [];
    },
    async spaces() {
      return [];
    },
    async targets() {
      return [];
    },
    async resources(params: ResourceParams) {
      calls.push(params);
      // Simulate the server applying a sound whereData LIKE pushdown.
      let rows = RESOURCES;
      if (params.whereData?.includes("containers.*.image LIKE '%:latest'")) {
        rows = rows.filter((r) => (r.image as string).endsWith(':latest'));
      }
      return rows.map((r) => ({ ...r }));
    },
  };
}

describe('executor — end to end via mock transport', () => {
  it('runs a simple pushdown query', async () => {
    const t = mockTransport();
    const res = await runQuery("SELECT unit FROM resources WHERE kind = 'Deployment'", t);
    expect(t.calls).toHaveLength(1);
    expect(res.rows).toHaveLength(4);
  });

  it('splits an OR into two fetches and unions the results', async () => {
    const t = mockTransport();
    const res = await runQuery(
      "SELECT unit, severity FROM resources WHERE severity = 'CRITICAL' OR image LIKE '%:latest'",
      t,
    );
    // Two DNF groups → two fetches.
    expect(t.calls).toHaveLength(2);
    // One fetch is unconstrained (severity client-side), one pushes image LIKE.
    expect(t.calls.some((c) => c.whereData?.includes('image LIKE'))).toBe(true);
    // Final result: 2 criticals + 1 :latest, deduped, exact via client filter.
    expect(res.rows.map((r) => r.unit).sort()).toEqual([
      'legacy-api',
      'legacy-frontend',
      'unpinned-web',
    ]);
    expect(res.stats.fetches).toBe(2);
  });

  it('de-dupes rows that match multiple OR branches', async () => {
    const t = mockTransport();
    // unpinned-web is MEDIUM with :latest — matches the image branch; the other
    // branch (severity='MEDIUM') also matches it. Must appear once.
    const res = await runQuery(
      "SELECT unit FROM resources WHERE severity = 'MEDIUM' OR image ~ ':latest'",
      t,
    );
    expect(res.rows.filter((r) => r.unit === 'unpinned-web')).toHaveLength(1);
  });

  it('aggregates across the unioned set', async () => {
    const t = mockTransport();
    const res = await runQuery(
      "SELECT space, COUNT(*) AS n FROM resources WHERE severity = 'CRITICAL' GROUP BY space",
      t,
    );
    expect(res.rows).toEqual([{ space: 'sec-demo-dev', n: 2 }]);
  });

  it('reports stats', async () => {
    const t = mockTransport();
    const res = await runQuery("SELECT unit FROM resources WHERE kind = 'Deployment' LIMIT 2", t);
    expect(res.stats.fetches).toBe(1);
    expect(res.stats.fetchedRows).toBe(4);
    expect(res.stats.resultRows).toBe(2);
  });
});
