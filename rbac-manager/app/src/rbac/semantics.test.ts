import { describe, expect, it } from 'vitest';

import {
  build,
  clusterRole,
  clusterRoleBinding,
  group,
  res,
  role,
  roleBinding,
  user,
} from './fixtures';
import {
  bindingScopeMatches,
  effectiveRules,
  nonResourceUrlMatches,
  resolveRoleRef,
  ruleMatches,
} from './semantics';
import { PolicyRule } from './model';

function rule(partial: Partial<PolicyRule>): PolicyRule {
  return {
    apiGroups: [],
    resources: [],
    verbs: [],
    resourceNames: [],
    nonResourceURLs: [],
    ...partial,
  };
}

describe('ruleMatches', () => {
  it('matches exact verb/group/resource', () => {
    const r = rule({ apiGroups: [''], resources: ['pods'], verbs: ['get'] });
    expect(ruleMatches(r, { verb: 'get', resource: 'pods', apiGroup: '' })).toBe(true);
    expect(ruleMatches(r, { verb: 'list', resource: 'pods', apiGroup: '' })).toBe(false);
    expect(ruleMatches(r, { verb: 'get', resource: 'secrets', apiGroup: '' })).toBe(false);
  });

  it('treats empty apiGroup as core and distinguishes named groups', () => {
    const core = rule({ apiGroups: [''], resources: ['pods'], verbs: ['get'] });
    expect(ruleMatches(core, { verb: 'get', resource: 'pods', apiGroup: 'apps' })).toBe(false);
    const apps = rule({ apiGroups: ['apps'], resources: ['deployments'], verbs: ['get'] });
    expect(ruleMatches(apps, { verb: 'get', resource: 'deployments' })).toBe(false); // default core
    expect(ruleMatches(apps, { verb: 'get', resource: 'deployments', apiGroup: 'apps' })).toBe(true);
  });

  it('honors wildcards in verbs, resources, and groups', () => {
    const r = rule({ apiGroups: ['*'], resources: ['*'], verbs: ['*'] });
    expect(ruleMatches(r, { verb: 'deletecollection', resource: 'anything', apiGroup: 'x.io' })).toBe(
      true,
    );
  });

  it('separates resources from subresources', () => {
    const bare = rule({ apiGroups: [''], resources: ['pods'], verbs: ['get'] });
    expect(ruleMatches(bare, { verb: 'get', resource: 'pods/log', apiGroup: '' })).toBe(false);
    const sub = rule({ apiGroups: [''], resources: ['pods/log'], verbs: ['get'] });
    expect(ruleMatches(sub, { verb: 'get', resource: 'pods', apiGroup: '' })).toBe(false);
    expect(ruleMatches(sub, { verb: 'get', resource: 'pods/log', apiGroup: '' })).toBe(true);
  });

  it('supports segment wildcards like */scale', () => {
    const r = rule({ apiGroups: ['apps'], resources: ['*/scale'], verbs: ['update'] });
    expect(
      ruleMatches(r, { verb: 'update', resource: 'deployments/scale', apiGroup: 'apps' }),
    ).toBe(true);
    expect(ruleMatches(r, { verb: 'update', resource: 'deployments', apiGroup: 'apps' })).toBe(
      false,
    );
  });

  it('applies resourceNames only when a name is queried', () => {
    const r = rule({
      apiGroups: [''],
      resources: ['configmaps'],
      verbs: ['get'],
      resourceNames: ['app-config'],
    });
    expect(ruleMatches(r, { verb: 'get', resource: 'configmaps', apiGroup: '' })).toBe(true);
    expect(
      ruleMatches(r, { verb: 'get', resource: 'configmaps', apiGroup: '', name: 'app-config' }),
    ).toBe(true);
    expect(
      ruleMatches(r, { verb: 'get', resource: 'configmaps', apiGroup: '', name: 'other' }),
    ).toBe(false);
  });

  it('never matches resource queries against nonResourceURL-only rules', () => {
    const r = rule({ nonResourceURLs: ['/healthz'], verbs: ['get'] });
    expect(ruleMatches(r, { verb: 'get', resource: 'pods', apiGroup: '' })).toBe(false);
    expect(nonResourceUrlMatches(r, 'get', '/healthz')).toBe(true);
    expect(nonResourceUrlMatches(r, 'get', '/metrics')).toBe(false);
  });

  it('matches nonResourceURL prefixes with trailing *', () => {
    const r = rule({ nonResourceURLs: ['/healthz/*'], verbs: ['get'] });
    expect(nonResourceUrlMatches(r, 'get', '/healthz/ready')).toBe(true);
    expect(nonResourceUrlMatches(r, 'get', '/livez')).toBe(false);
  });
});

describe('effectiveRules / aggregation', () => {
  it('returns own rules for plain roles', () => {
    const clusters = build([
      res('c1', clusterRole('plain', [{ apiGroups: [''], resources: ['pods'], verbs: ['get'] }])),
    ]);
    const c1 = clusters.get('c1')!;
    expect(effectiveRules(c1.roles[0], c1)).toHaveLength(1);
  });

  it('unions matching roles and follows nested aggregation to a fixed point', () => {
    const clusters = build([
      res(
        'c1',
        clusterRole('top', [], {
          aggregationRule: { clusterRoleSelectors: [{ matchLabels: { tier: 'mid' } }] },
        }),
      ),
      res(
        'c1',
        clusterRole('mid', [{ apiGroups: [''], resources: ['pods'], verbs: ['get'] }], {
          labels: { tier: 'mid' },
          aggregationRule: { clusterRoleSelectors: [{ matchLabels: { tier: 'leaf' } }] },
        }),
      ),
      res(
        'c1',
        clusterRole('leaf', [{ apiGroups: [''], resources: ['secrets'], verbs: ['list'] }], {
          labels: { tier: 'leaf' },
        }),
      ),
    ]);
    const c1 = clusters.get('c1')!;
    const top = c1.roles.find((r) => r.name === 'top')!;
    const rules = effectiveRules(top, c1);
    const resources = rules.flatMap((r) => r.resources).sort();
    expect(resources).toEqual(['pods', 'secrets']);
  });
});

describe('binding resolution and scope', () => {
  it('resolves RoleBinding to a Role in its own namespace only', () => {
    const clusters = build([
      res('c1', role('reader', 'team-a', [{ apiGroups: [''], resources: ['pods'], verbs: ['get'] }])),
      res('c1', roleBinding('rb-a', 'team-a', { kind: 'Role', name: 'reader' }, [user('alice')])),
      res('c1', roleBinding('rb-b', 'team-b', { kind: 'Role', name: 'reader' }, [user('bob')])),
    ]);
    const c1 = clusters.get('c1')!;
    const rbA = c1.bindings.find((b) => b.name === 'rb-a')!;
    const rbB = c1.bindings.find((b) => b.name === 'rb-b')!;
    expect(resolveRoleRef(rbA, c1)?.name).toBe('reader');
    expect(resolveRoleRef(rbB, c1)).toBeUndefined();
  });

  it('resolves RoleBinding to a ClusterRole, scoped to the binding namespace', () => {
    const clusters = build([
      res('c1', clusterRole('viewer', [{ apiGroups: [''], resources: ['pods'], verbs: ['get'] }])),
      res(
        'c1',
        roleBinding('rb', 'team-a', { kind: 'ClusterRole', name: 'viewer' }, [group('devs')]),
      ),
    ]);
    const c1 = clusters.get('c1')!;
    const rb = c1.bindings[0];
    expect(resolveRoleRef(rb, c1)?.kind).toBe('ClusterRole');
    expect(bindingScopeMatches(rb, 'team-a')).toBe(true);
    expect(bindingScopeMatches(rb, 'team-b')).toBe(false);
    expect(bindingScopeMatches(rb, undefined)).toBe(true);
  });

  it('treats ClusterRoleBindings as cluster-wide', () => {
    const clusters = build([
      res('c1', clusterRole('viewer', [])),
      res('c1', clusterRoleBinding('crb', 'viewer', [group('devs')])),
    ]);
    const crb = clusters.get('c1')!.bindings[0];
    expect(bindingScopeMatches(crb, 'any-namespace')).toBe(true);
  });
});
