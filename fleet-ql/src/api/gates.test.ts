import { describe, expect, it } from 'vitest';

import type { Row } from '../fql';
import { spreadGatesByTrigger, triggerOfGateKey } from './gates';

describe('triggerOfGateKey', () => {
  it('takes the middle of a 3-part key', () => {
    expect(triggerOfGateKey('sec-demo-policy/no-critical-cves/vet-celexpr')).toBe('no-critical-cves');
  });

  it('takes the first of a legacy 2-part key', () => {
    expect(triggerOfGateKey('no-wildcards/vet-celexpr')).toBe('no-wildcards');
  });

  it('returns null for a key with no separator', () => {
    expect(triggerOfGateKey('lonely')).toBeNull();
  });
});

describe('spreadGatesByTrigger', () => {
  it('re-keys gates by trigger slug', () => {
    const row: Row = {};
    spreadGatesByTrigger(row, 'gate', {
      'sec-demo-policy/no-critical-cves/vet-celexpr': true,
      'rbac-demo-policy/no-wildcards/vet-celexpr': false,
    });
    expect(row['gate.no-critical-cves']).toBe(true);
    expect(row['gate.no-wildcards']).toBe(false);
  });

  it('ORs values when one trigger has several gates (any blocking → true)', () => {
    const row: Row = {};
    spreadGatesByTrigger(row, 'gate', {
      'space-a/approval/vet-approvedby': false,
      'space-b/approval/vet-celexpr': true, // same trigger slug, different fn/space
    });
    expect(row['gate.approval']).toBe(true);
  });

  it('is a no-op for an undefined map', () => {
    const row: Row = { slug: 'x' };
    spreadGatesByTrigger(row, 'gate', undefined);
    expect(row).toEqual({ slug: 'x' });
  });
});
