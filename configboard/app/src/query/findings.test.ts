import type { ExtendedUnitRead } from '@confighub/rtk-query';
import { describe, expect, it } from 'vitest';

import { aggregate } from './aggregate';
import { findingRows } from './rows';

const BASE = 'https://hub.example.com';

const unit = (over: Partial<NonNullable<ExtendedUnitRead['Unit']>> = {}): ExtendedUnitRead => ({
  Unit: {
    UnitID: 'u1',
    SpaceID: 's1',
    Slug: 'frontend',
    TargetID: 't1',
    HeadRevisionNum: 6,
    LiveRevisionNum: 6,
    // Required by the generated UnitRead type.
    ToolchainType: 'Kubernetes/YAML',
    ...over,
  },
  Space: { Slug: 'apptique-prod', Labels: { Environment: 'prod', Component: 'apptique' } },
  Target: {
    Slug: 'prod-cluster',
    BridgeWorkerID: 'w1',
    ProviderType: 'Kubernetes',
    ToolchainType: 'Kubernetes/YAML',
  },
});

describe('findingRows', () => {
  it('emits one row per failing check, not one per Unit', () => {
    // The real key shape, verified against a live org:
    // <policy-space>/<trigger-slug>/<function>
    const rows = findingRows(
      unit({
        ApplyWarnings: {
          'workload-policy/workload-runs-nonroot/vet-cel': true,
          'workload-policy/workload-termination-message-policy/vet-cel': true,
        },
      }),
      BASE,
    );
    expect(rows).toHaveLength(2);
    expect(rows.map((r) => r.values['Finding.Trigger']).sort()).toEqual([
      'workload-runs-nonroot',
      'workload-termination-message-policy',
    ]);
  });

  it('splits the key into policy Space, trigger, and validator', () => {
    const [row] = findingRows(
      unit({ ApplyWarnings: { 'workload-policy/workload-has-limits/vet-cel': true } }),
      BASE,
    );
    expect(row.values['Finding.PolicySpace']).toBe('workload-policy');
    expect(row.values['Finding.Trigger']).toBe('workload-has-limits');
    expect(row.values['Finding.Function']).toBe('vet-cel');
    expect(row.values['Finding.Kind']).toBe('Warning');
  });

  it('distinguishes gates from warnings', () => {
    const rows = findingRows(
      unit({
        ApplyGates: { 'home/valid-k8s/vet-schemas': true },
        ApplyWarnings: { 'workload-policy/workload-has-limits/vet-cel': true },
      }),
      BASE,
    );
    expect(rows.map((r) => r.values['Finding.Kind'])).toEqual(['Gate', 'Warning']);
  });

  it('carries the Unit, Space, and Target identity onto every finding', () => {
    const [row] = findingRows(
      unit({ ApplyWarnings: { 'p/check/vet-cel': true } }),
      BASE,
    );
    expect(row.values['Unit.Slug']).toBe('frontend');
    expect(row.values['Space.Slug']).toBe('apptique-prod');
    expect(row.values['Space.Labels.Environment']).toBe('prod');
    expect(row.values['Target.Slug']).toBe('prod-cluster');
    expect(row.values['Unit.ApplyState']).toBe('Applied and current');
    expect(row.href).toBe(`${BASE}/units/s1/u1`);
  });

  it('emits nothing for a clean Unit', () => {
    expect(findingRows(unit(), BASE)).toEqual([]);
    expect(findingRows(unit({ ApplyGates: {}, ApplyWarnings: {} }), BASE)).toEqual([]);
  });

  it('keeps a malformed key rather than dropping the finding', () => {
    // A finding that cannot be parsed still matters; losing it would understate the fleet.
    const rows = findingRows(unit({ ApplyWarnings: { 'just-a-slug': true } }), BASE);
    expect(rows).toHaveLength(1);
    expect(rows[0].values['Finding.Trigger']).toBe('just-a-slug');
    expect(rows[0].values['Finding.PolicySpace']).toBeNull();
  });

  it('gives each finding a distinct id so aggregation counts them separately', () => {
    const rows = findingRows(
      unit({
        ApplyWarnings: { 'p/a/vet-cel': true, 'p/b/vet-cel': true },
        ApplyGates: { 'p/a/vet-cel': true },
      }),
      BASE,
    );
    expect(new Set(rows.map((r) => r.id)).size).toBe(3);
  });
});

describe('findings aggregation', () => {
  // The shape of the real fleet result: 36 findings over 24 Units, dominated by one check.
  const units: ExtendedUnitRead[] = [
    ...Array.from({ length: 10 }, (_, i) =>
      unit({
        UnitID: `a${i}`,
        Slug: `svc-${i}`,
        ApplyWarnings: { 'workload-policy/workload-termination-message-policy/vet-cel': true },
      }),
    ),
    ...Array.from({ length: 2 }, (_, i) =>
      unit({
        UnitID: `b${i}`,
        Slug: `ns-${i}`,
        ApplyWarnings: { 'namespace-policy/namespace-has-pod-security/vet-celexpr': true },
      }),
    ),
  ];

  it('counts findings by check, not by Unit', () => {
    const rows = units.flatMap((u) => findingRows(u, BASE));
    const frame = aggregate(rows, {
      groupBy: 'Finding.Trigger',
      aggregate: { fn: 'count' },
      sort: 'value-desc',
    });
    expect(frame.categories).toEqual([
      'workload-termination-message-policy',
      'namespace-has-pod-security',
    ]);
    expect(frame.series[0].points.map((p) => p.value)).toEqual([10, 2]);
    expect(frame.total).toBe(12);
  });

  it('counts affected Units distinctly from findings', () => {
    const rows = units.flatMap((u) => findingRows(u, BASE));
    const frame = aggregate(rows, { aggregate: { fn: 'distinctCount', field: 'Unit.Slug' } });
    expect(frame.total).toBe(12);
  });
});
