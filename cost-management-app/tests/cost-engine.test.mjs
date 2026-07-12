import assert from 'node:assert/strict';
import test from 'node:test';
import { analyze, defaultRateCard, parseCpu, parseMemGiB, monthlyCost, NIL_UUID } from '../src/cost-engine.mjs';
import { splitJsonObjects } from '../scripts/cost-sweep.mjs';

const RATE = {...defaultRateCard, cpuPerCoreMonth: 20, memPerGiBMonth: 4};

test('quantity parsing', () => {
  assert.equal(parseCpu('100m'), 0.1);
  assert.equal(parseCpu('1'), 1);
  assert.equal(parseCpu('0.5'), 0.5);
  assert.equal(parseCpu(''), null);
  assert.equal(parseMemGiB('1Gi'), 1);
  assert.equal(parseMemGiB('512Mi'), 0.5);
  assert.equal(parseMemGiB(''), null);
  assert.ok(Math.abs(parseMemGiB('1G') - 0.9313) < 0.001);
  assert.equal(monthlyCost(1, 1, RATE), 24);
});

test('splitJsonObjects handles a pretty-printed stream with braces in strings', () => {
  const rows = splitJsonObjects('{\n "a": "x{y}"\n}\n{\n "b": 2\n}\n');
  assert.deepEqual(rows, [{a: 'x{y}'}, {b: 2}]);
});

const bound = {space: 's-dev', unit: 'u1', target: 'aaaa1111-0000-0000-0000-000000000000', live: 3, head: 3, updated: ''};
const unbound = {space: 's-dev', unit: 'u2', target: NIL_UUID, live: 0, head: 1, updated: ''};

function row(overrides) {
  return {
    space: 's-dev', unit: 'u1', kind: 'Deployment', workload: 'w', container: 'c',
    replicas: 1, cpu_req: '100m', cpu_lim: '', mem_req: '256Mi', mem_lim: '', eph_lim: '',
    ...overrides,
  };
}

test('missing requests is found and never priced as savings', () => {
  const report = analyze({containers: [row({cpu_req: '', mem_req: '', cpu_lim: '500m'})], units: [bound]}, RATE);
  const finding = report.findings.find(f => f.rule === 'MISSING_REQUESTS');
  assert.ok(finding);
  assert.deepEqual(finding.evidence.missing, ['cpu', 'memory']);
  assert.equal(finding.priced.claim, 'exposure-at-limits-not-savings');
  assert.equal(report.totals.claimedMonthlySavings, 0);
});

test('non-prod replicas priced from requests only when the unit is bound', () => {
  const containers = [
    row({unit: 'u1', replicas: 3}),
    row({unit: 'u2', replicas: 3}),
  ];
  const report = analyze({containers, units: [bound, unbound]}, RATE);
  const rows = report.findings.filter(f => f.rule === 'NONPROD_REPLICAS');
  assert.equal(rows.length, 2);
  const boundFinding = rows.find(f => f.unit === 'u1');
  const unboundFinding = rows.find(f => f.unit === 'u2');
  // per replica: 0.1cpu*20 + 0.25GiB*4 = 3; two removable replicas = 6
  assert.equal(boundFinding.priced.monthly, 6);
  assert.equal(boundFinding.priced.claim, 'monthly-savings-if-scaled-to-1');
  assert.equal(unboundFinding.priced.claim, 'configured-cost-only-unit-not-bound');
  assert.equal(report.totals.claimedMonthlySavings, 6);
});

test('prod-named spaces are exempt from the replica rule', () => {
  const report = analyze({containers: [row({space: 'shop-prod-us', replicas: 3})], units: []}, RATE);
  assert.equal(report.findings.filter(f => f.rule === 'NONPROD_REPLICAS').length, 0);
});

test('leftover space detected with honest bound/unbound claims', () => {
  const leftBound = {...bound, space: 'redis-live-20260705', unit: 'u1'};
  const containers = [row({space: 'redis-live-20260705', unit: 'u1', replicas: 2})];
  const report = analyze({containers, units: [leftBound]}, RATE);
  const finding = report.findings.find(f => f.rule === 'LEFTOVER_SPACE');
  assert.ok(finding);
  assert.equal(finding.severity, 'high');
  assert.equal(finding.priced.claim, 'reclaimable-if-decommissioned');
  // 2 replicas * 3/month
  assert.equal(finding.priced.monthly, 6);

  const reportUnbound = analyze({containers, units: [{...leftBound, target: NIL_UUID}]}, RATE);
  const findingUnbound = reportUnbound.findings.find(f => f.rule === 'LEFTOVER_SPACE');
  assert.equal(findingUnbound.severity, 'medium');
  assert.equal(findingUnbound.priced.claim, 'configured-cost-only-no-units-bound');
});

test('limit headroom is hygiene, never priced', () => {
  const report = analyze({containers: [row({cpu_req: '100m', cpu_lim: '1'})], units: [bound]}, RATE);
  const finding = report.findings.find(f => f.rule === 'LIMIT_HEADROOM');
  assert.ok(finding);
  assert.equal(finding.priced, null);
});

test('stack copies surface as evidence-only from three copies up', () => {
  const containers = ['dev', 'base', '1-2-3-default'].map(suffix =>
    row({space: `redis-stack-${suffix}`, unit: `u-${suffix}`}));
  const report = analyze({containers, units: []}, RATE);
  const finding = report.findings.find(f => f.rule === 'STACK_COPIES');
  assert.ok(finding);
  assert.equal(finding.evidence.copies, 3);
  assert.equal(finding.priced, null);
});

test('every priced figure carries the rate-card basis', () => {
  const containers = [row({replicas: 4}), row({space: 'x-live-20260101', unit: 'u1', replicas: 2})];
  const report = analyze({containers, units: [bound, {...bound, space: 'x-live-20260101'}]}, RATE);
  for (const finding of report.findings) {
    if (finding.priced) assert.equal(finding.priced.basis, RATE.basis);
  }
});
