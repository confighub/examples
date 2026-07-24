import { describe, expect, it } from 'vitest';

import type { InvokeResult } from './resources';
import { providerFamily, resourceRows, resourceScope, splitResourceType } from './resources';

describe('splitResourceType', () => {
  it('splits group/version/kind', () => {
    expect(splitResourceType('apps/v1/Deployment')).toEqual({
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
    });
  });

  it('treats a two-part type as core (no group)', () => {
    expect(splitResourceType('v1/Namespace')).toEqual({
      group: '',
      version: 'v1',
      kind: 'Namespace',
    });
  });

  it('handles dotted CRD groups', () => {
    expect(splitResourceType('ec2.services.k8s.aws/v1alpha1/VPC')).toEqual({
      group: 'ec2.services.k8s.aws',
      version: 'v1alpha1',
      kind: 'VPC',
    });
    expect(splitResourceType('rbac.authorization.k8s.io/v1/RoleBinding').kind).toBe('RoleBinding');
  });

  it('does not lose an unparseable type', () => {
    expect(splitResourceType('Weird').kind).toBe('Weird');
  });
});

describe('providerFamily', () => {
  it('recognizes ACK and Crossplane without a hardcoded resource list', () => {
    expect(providerFamily('ec2.services.k8s.aws')).toBe('ACK (AWS)');
    expect(providerFamily('rds.services.k8s.aws')).toBe('ACK (AWS)');
    expect(providerFamily('ec2.aws.upbound.io')).toBe('Crossplane');
    expect(providerFamily('apiextensions.crossplane.io')).toBe('Crossplane');
  });

  it('groups core Kubernetes together', () => {
    expect(providerFamily('')).toBe('Kubernetes core');
    expect(providerFamily('apps')).toBe('Kubernetes core');
    expect(providerFamily('rbac.authorization.k8s.io')).toBe('Kubernetes core');
  });

  it('passes an unknown group through rather than bucketing it as "other"', () => {
    expect(providerFamily('cert-manager.io')).toBe('cert-manager.io');
  });
});

describe('resourceScope', () => {
  it('reads the scope ahead of the slash', () => {
    expect(resourceScope('ack-system/ack-ec2-controller')).toBe('ack-system');
  });

  it('returns null for a cluster-scoped resource', () => {
    // A leading slash means "no scope", not an empty-named namespace.
    expect(resourceScope('/ack-system')).toBeNull();
    expect(resourceScope('plain-name')).toBeNull();
    expect(resourceScope(undefined)).toBeNull();
  });
});

// Two resources in one Unit, exactly as the API returns them: base64-encoded JSON in
// Outputs.ResourceList.
const encode = (value: unknown) => btoa(JSON.stringify(value));

const RESULTS: InvokeResult[] = [
  {
    UnitID: 'u1',
    UnitSlug: 'ack-ec2',
    SpaceID: 's1',
    SpaceSlug: 'prod-platform',
    TargetID: 't1',
    Outputs: {
      ResourceList: encode([
        {
          ResourceName: '/ack-system',
          ResourceNameWithoutScope: 'ack-system',
          ResourceType: 'v1/Namespace',
          ResourceCategory: 'Resource',
        },
        {
          ResourceName: 'ack-system/controller',
          ResourceNameWithoutScope: 'controller',
          ResourceType: 'apps/v1/Deployment',
          ResourceCategory: 'Resource',
        },
      ]),
    },
  },
  { UnitID: 'u2', UnitSlug: 'no-output', Outputs: {} },
];

describe('resourceRows', () => {
  it('emits one row per resource, not per Unit', () => {
    const rows = resourceRows(RESULTS, { t1: 'prod-us-east-2' }, 'https://hub.example.com');
    expect(rows).toHaveLength(2);
  });

  it('carries resource, Unit, Space, and Target identity on each row', () => {
    const [ns, deploy] = resourceRows(RESULTS, { t1: 'prod-us-east-2' }, 'https://hub.example.com');

    expect(ns.values['Resource.Kind']).toBe('Namespace');
    expect(ns.values['Resource.Group']).toBe('core');
    expect(ns.values['Resource.Scope']).toBeNull();

    expect(deploy.values['Resource.Kind']).toBe('Deployment');
    expect(deploy.values['Resource.Family']).toBe('Kubernetes core');
    expect(deploy.values['Resource.Scope']).toBe('ack-system');
    expect(deploy.values['Unit.Slug']).toBe('ack-ec2');
    expect(deploy.values['Space.Slug']).toBe('prod-platform');
    expect(deploy.values['Target.Slug']).toBe('prod-us-east-2');
    expect(deploy.href).toBe('https://hub.example.com/units/s1/u1');
  });

  it('skips Units whose invocation returned no resource list', () => {
    const rows = resourceRows(RESULTS, {}, 'https://hub.example.com');
    expect(rows.every((r) => r.values['Unit.Slug'] !== 'no-output')).toBe(true);
  });

  it('marks a genuinely unresolvable Target rather than dropping the row', () => {
    const rows = resourceRows(RESULTS, {}, 'https://hub.example.com');
    expect(rows[0].values['Target.Slug']).toBe('(unknown)');
  });

  it('reads the zero UUID as "no Target", not as an unresolvable one', () => {
    // Go serializes an unset uuid.UUID as the zero UUID even with omitempty, and
    // /function/invoke does exactly that. Treating it as an id put 119 Units' worth of
    // resources into an "(unknown)" cluster bucket that did not exist.
    const rows = resourceRows(
      [
        {
          UnitID: '00000000-0000-0000-0000-000000000000',
          UnitSlug: 'base-unit',
          SpaceID: '00000000-0000-0000-0000-000000000000',
          TargetID: '00000000-0000-0000-0000-000000000000',
          Outputs: {
            ResourceList: encode([
              { ResourceName: 'ns/thing', ResourceType: 'apps/v1/Deployment' },
            ]),
          },
        },
      ],
      { t1: 'prod-us-east-2' },
      'https://hub.example.com',
    );

    expect(rows).toHaveLength(1);
    expect(rows[0].values['Target.Slug']).toBeNull();
    // And no deep link is built from zero ids.
    expect(rows[0].href).toBeUndefined();
  });

  it('survives a malformed output without throwing', () => {
    const rows = resourceRows(
      [{ UnitID: 'u3', Outputs: { ResourceList: 'not-base64-json' } }],
      {},
      'https://hub.example.com',
    );
    expect(rows).toEqual([]);
  });
});
