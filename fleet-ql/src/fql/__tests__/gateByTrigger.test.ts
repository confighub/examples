import { describe, expect, it } from 'vitest';

import type { Row } from '../evaluate';
import { planQuery, runQuery } from '../index';
import type { ListParams, Transport } from '../transport';

describe('gate[<trigger>] — planner', () => {
  it('is client-side (no pushdown); pair with a pushed space scope', () => {
    const p = planQuery("SELECT slug FROM units WHERE gate['no-critical-cves'] = true");
    expect(p.fetches).toEqual([{}]); // trigger match can't push
    expect(p.residual).not.toBeNull();
  });

  it('combines a pushed space with the client-side gate filter', () => {
    const p = planQuery("SELECT slug FROM units WHERE space = 'prod' AND gate['no-wildcards'] = true");
    expect(p.fetches).toEqual([{ where: "Space.Slug = 'prod'" }]);
  });

  it('the exact full-key form still pushes to where', () => {
    const p = planQuery("SELECT slug FROM units WHERE applyGates['p/no-critical-cves/vet-celexpr'] = true");
    expect(p.fetches[0].where).toBe('ApplyGates.p/no-critical-cves/vet-celexpr = true');
  });
});

// Rows as the transport would emit them: gates re-keyed by trigger slug.
const UNITS: Row[] = [
  { __id: '1', slug: 'blocked', space: 'prod', 'gate.no-critical-cves': true },
  { __id: '2', slug: 'clean', space: 'prod', 'gate.no-critical-cves': false },
  { __id: '3', slug: 'untouched', space: 'dev' },
];

function mockTransport(): Transport {
  return {
    async units(_p: ListParams) {
      return UNITS.map((r) => ({ ...r }));
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
    async grants() {
      return [];
    },
    async roles() {
      return [];
    },
    async bindings() {
      return [];
    },
  };
}

describe('gate[<trigger>] — end to end', () => {
  it('finds units blocked by a trigger, by slug', async () => {
    const res = await runQuery("SELECT slug FROM units WHERE gate['no-critical-cves'] = true", mockTransport());
    expect(res.rows.map((r) => r.slug)).toEqual(['blocked']);
  });

  it('IS NULL distinguishes "no such gate" from "gate present and false"', async () => {
    const res = await runQuery("SELECT slug FROM units WHERE gate['no-critical-cves'] IS NULL", mockTransport());
    expect(res.rows.map((r) => r.slug)).toEqual(['untouched']);
  });
});
