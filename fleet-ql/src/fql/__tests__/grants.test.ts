import { describe, expect, it } from 'vitest';

import type { Row } from '../evaluate';
import { planQuery, runQuery } from '../index';
import type { GrantsParams, Transport } from '../transport';

describe('grants — planner (access-query selector capture + residual strip)', () => {
  it('captures verb/resource into accessQuery, not where', () => {
    const p = planQuery("SELECT subject, cluster FROM grants WHERE verb = 'delete' AND resource = 'pods'");
    expect(p.source).toBe('grants');
    expect(p.fetches).toEqual([{ accessQuery: { verb: 'delete', resource: 'pods' } }]);
    // Both selector atoms are stripped → nothing left to re-check client-side.
    expect(p.residual).toBeNull();
  });

  it('pushes space to where but keeps subject/cluster as client-side residual', () => {
    const p = planQuery(
      "SELECT subject FROM grants WHERE space = 'rbac-demo-prod' AND verb = 'get' AND subject = 'User:alice'",
    );
    expect(p.fetches[0].where).toBe("Space.Slug = 'rbac-demo-prod'");
    expect(p.fetches[0].accessQuery).toEqual({ verb: 'get' });
    // subject survives as residual (a real output column, re-checked client-side).
    expect(p.residual).not.toBeNull();
  });

  it('namespace and name are access selectors too', () => {
    const p = planQuery(
      "SELECT subject FROM grants WHERE resource = 'secrets' AND namespace = 'web' AND name = 'db-creds'",
    );
    expect(p.fetches[0].accessQuery).toEqual({ resource: 'secrets', namespace: 'web', name: 'db-creds' });
  });
});

// A mock materializer: records the access query it received and returns a fixed
// grant set, so we can assert the executor forwards the selector and applies the
// client-side residual on output columns.
function mockTransport(): Transport & { lastGrantQuery?: GrantsParams } {
  const t: Transport & { lastGrantQuery?: GrantsParams } = {
    async units() {
      return [];
    },
    async resources() {
      return [];
    },
    async spaces() {
      return [];
    },
    async revisions() {
      return [];
    },
    async roles() {
      return [];
    },
    async bindings() {
      return [];
    },
    async rbacFindings() {
      return [];
    },
    async grants(params: GrantsParams): Promise<Row[]> {
      t.lastGrantQuery = params;
      // Pretend the materializer already applied the access query: two subjects
      // can delete pods on prod, one on dev.
      return [
        { subject: 'User:alice', cluster: 'prod', space: 'rbac-demo-prod', unit: 'u1', binding: 'b1', scope: null, role: 'editor', viaBuiltin: false },
        { subject: 'Group:devs', cluster: 'prod', space: 'rbac-demo-prod', unit: 'u1', binding: 'b1', scope: null, role: 'editor', viaBuiltin: false },
        { subject: 'User:bob', cluster: 'dev', space: 'rbac-demo-dev', unit: 'u2', binding: 'b2', scope: null, role: 'editor', viaBuiltin: false },
      ];
    },
  };
  return t;
}

describe('grants — executor (end to end via mock materializer)', () => {
  it('forwards the access query and projects output columns', async () => {
    const t = mockTransport();
    const res = await runQuery(
      "SELECT subject, cluster FROM grants WHERE verb = 'delete' AND resource = 'pods'",
      t,
    );
    expect(t.lastGrantQuery?.accessQuery).toEqual({ verb: 'delete', resource: 'pods' });
    expect(res.rows).toHaveLength(3);
    expect(res.columns).toEqual(['subject', 'cluster']);
  });

  it('re-checks an output-column predicate (cluster) client-side', async () => {
    const t = mockTransport();
    const res = await runQuery(
      "SELECT subject FROM grants WHERE verb = 'delete' AND resource = 'pods' AND cluster = 'prod'",
      t,
    );
    // The materializer returned 3; the client-side residual cluster='prod' keeps 2.
    expect(res.rows.map((r) => r.subject).sort()).toEqual(['Group:devs', 'User:alice']);
  });

  it('subject filter (inverse view) is applied client-side', async () => {
    const t = mockTransport();
    const res = await runQuery("SELECT cluster, role FROM grants WHERE subject = 'User:alice'", t);
    expect(t.lastGrantQuery?.accessQuery).toBeUndefined(); // no access question
    expect(res.rows).toEqual([{ cluster: 'prod', role: 'editor' }]);
  });
});
