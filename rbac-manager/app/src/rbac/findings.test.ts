import { describe, expect, it } from 'vitest';

import {
  build,
  clusterRole,
  clusterRoleBinding,
  group,
  res,
  roleBinding,
  serviceAccount,
  sa,
  user,
} from './fixtures';
import { analyzeFleet } from './findings';

function analyzers(findings: ReturnType<typeof analyzeFleet>): string[] {
  return [...new Set(findings.map((f) => f.analyzer))];
}

describe('analyzeFleet', () => {
  it('flags wildcard rules', () => {
    const findings = analyzeFleet(
      build([res('c1', clusterRole('legacy-admin', [
        { apiGroups: ['*'], resources: ['*'], verbs: ['*'] },
      ]))]),
    );
    const wild = findings.filter((f) => f.analyzer === 'wildcard-rules');
    expect(wild).toHaveLength(1);
    expect(wild[0].severity).toBe('high');
    expect(wild[0].resourceName).toBe('legacy-admin');
  });

  it('flags privilege-escalation verbs', () => {
    const findings = analyzeFleet(
      build([res('c1', clusterRole('escalator', [
        { apiGroups: ['rbac.authorization.k8s.io'], resources: ['clusterroles'], verbs: ['bind', 'escalate'] },
      ]))]),
    );
    const esc = findings.filter((f) => f.analyzer === 'privilege-escalation-verbs');
    expect(esc).toHaveLength(1);
    expect(esc[0].message).toContain('escalate');
  });

  it('flags secrets and exec as risky grants', () => {
    const findings = analyzeFleet(
      build([res('c1', clusterRole('ops', [
        { apiGroups: [''], resources: ['secrets'], verbs: ['get', 'list'] },
        { apiGroups: [''], resources: ['pods/exec'], verbs: ['create'] },
      ]))]),
    );
    const risky = findings.filter((f) => f.analyzer === 'risky-grants');
    expect(risky).toHaveLength(1);
    expect(risky[0].message).toContain('secrets read');
    expect(risky[0].message).toContain('pod exec');
  });

  it('flags cluster-admin bindings, including equivalent custom superuser roles', () => {
    const findings = analyzeFleet(
      build([
        res('c1', clusterRoleBinding('breakglass', 'cluster-admin', [group('oncall')])),
        res('c1', clusterRole('shadow-admin', [
          { apiGroups: ['*'], resources: ['*'], verbs: ['*'] },
        ])),
        res('c1', clusterRoleBinding('shadow', 'shadow-admin', [user('bob')])),
      ]),
    );
    const admin = findings.filter((f) => f.analyzer === 'cluster-admin-bindings');
    expect(admin.map((f) => f.resourceName).sort()).toEqual(['breakglass', 'shadow']);
  });

  it('flags orphaned bindings but not builtins', () => {
    const findings = analyzeFleet(
      build([
        res('c1', roleBinding('orphan', 'monitoring', { kind: 'Role', name: 'grafana-viewer' }, [
          user('alice@example.com'),
        ])),
        res('c1', clusterRoleBinding('ok-builtin', 'view', [group('devs')])),
        res('c1', clusterRoleBinding('ok-system', 'system:metrics-reader', [group('devs')])),
      ]),
    );
    const orphans = findings.filter((f) => f.analyzer === 'orphaned-bindings');
    expect(orphans).toHaveLength(1);
    expect(orphans[0].resourceName).toBe('orphan');
    expect(orphans[0].message).toContain('grafana-viewer');
  });

  it('flags ServiceAccounts bound to nothing', () => {
    const findings = analyzeFleet(
      build([
        res('c1', serviceAccount('used', 'apps')),
        res('c1', serviceAccount('unused', 'apps')),
        res('c1', roleBinding('rb', 'apps', { kind: 'ClusterRole', name: 'edit' }, [
          sa('used', 'apps'),
        ])),
      ]),
    );
    const unbound = findings.filter((f) => f.analyzer === 'unbound-service-accounts');
    expect(unbound).toHaveLength(1);
    expect(unbound[0].resourceName).toBe('unused');
  });

  it('produces no findings for a clean persona-style cluster', () => {
    const findings = analyzeFleet(
      build([
        res('c1', clusterRole('developer', [
          { apiGroups: ['', 'apps'], resources: ['pods', 'deployments'], verbs: ['get', 'list', 'watch', 'create', 'update', 'patch'] },
        ])),
        res('c1', clusterRoleBinding('developer', 'developer', [group('oidc:developers')])),
      ]),
    );
    expect(analyzers(findings)).toEqual([]);
  });

  it('sorts by severity then cluster', () => {
    const findings = analyzeFleet(
      build([
        res('b-cluster', serviceAccount('unused', 'apps')),
        res('a-cluster', clusterRole('legacy', [
          { apiGroups: ['*'], resources: ['*'], verbs: ['*'] },
        ])),
      ]),
    );
    expect(findings[0].severity).toBe('high');
    expect(findings[findings.length - 1].severity).toBe('low');
  });
});
