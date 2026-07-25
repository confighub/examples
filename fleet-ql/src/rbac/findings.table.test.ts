import { describe, expect, it } from 'vitest';

import {
  clusterRole,
  clusterRoleBinding,
  res,
  roleBinding,
  serviceAccount,
  user,
} from './fixtures';
import type { FleetResource } from './model';
import { materializeFindings } from './structural';

const FLEET: FleetResource[] = [
  // wildcard + escalation verbs
  res('prod', clusterRole('legacy-admin', [{ apiGroups: ['*'], resources: ['*'], verbs: ['*'] }])),
  res('prod', clusterRole('escalator', [{ apiGroups: ['rbac.authorization.k8s.io'], resources: ['clusterroles'], verbs: ['escalate', 'bind'] }])),
  // risky grant: write to secrets
  res('prod', clusterRole('secret-writer', [{ apiGroups: [''], resources: ['secrets'], verbs: ['get', 'update'] }])),
  // cluster-admin binding + an orphaned binding
  res('prod', clusterRoleBinding('superusers', 'cluster-admin', [user('root')])),
  res('prod', roleBinding('dangling', 'web', { kind: 'Role', name: 'ghost' }, [user('nobody')])),
  // an unbound ServiceAccount
  res('prod', serviceAccount('idle-robot', 'web')),
];

describe('materializeFindings', () => {
  const rows = materializeFindings(FLEET);
  const analyzers = new Set(rows.map((r) => r.analyzer));

  it('runs every analyzer the boolean flags do not cover', () => {
    expect(analyzers).toContain('wildcard-rules');
    expect(analyzers).toContain('privilege-escalation-verbs');
    expect(analyzers).toContain('risky-grants');
    expect(analyzers).toContain('cluster-admin-bindings');
    expect(analyzers).toContain('orphaned-bindings');
    expect(analyzers).toContain('unbound-service-accounts');
  });

  it('carries severity, cluster, resource identity and a message', () => {
    const esc = rows.find((r) => r.analyzer === 'privilege-escalation-verbs');
    expect(esc?.severity).toBe('high');
    expect(esc?.cluster).toBe('prod');
    expect(esc?.resourceName).toBe('escalator');
    expect(String(esc?.message)).toMatch(/escalate|bind/);
  });

  it('flags the unbound ServiceAccount as low severity', () => {
    const sa = rows.find((r) => r.analyzer === 'unbound-service-accounts');
    expect(sa?.severity).toBe('low');
    expect(sa?.resourceName).toBe('idle-robot');
    expect(sa?.namespace).toBe('web');
  });
});
