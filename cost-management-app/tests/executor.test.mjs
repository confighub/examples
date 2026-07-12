import assert from 'node:assert/strict';
import test from 'node:test';
import { mkdtempSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createPreview, recordReview, commitReviewed, reviewPacketStatus, ALLOWED_FUNCTIONS } from '../src/executor.mjs';

function fakeCub(script) {
  const calls = [];
  return {
    calls,
    cub: cubArgs => {
      calls.push(cubArgs);
      const step = script.shift();
      assert.ok(step, `unexpected cub call: ${cubArgs.join(' ')}`);
      return step(cubArgs);
    },
  };
}

function commandArgs(call) {
  return call[0] === '--context' ? call.slice(2) : call;
}

const ok = stdout => () => ({ok: true, stdout, stderr: ''});
const headResponse = value => ok(`${value}\n`);
const unitResponse = (head = 7, organizationId = 'org-internal-1', spaceId = 'space-id-1', unitId = 'unit-id-1') => ok(JSON.stringify({
  Unit: {HeadRevisionNum: head, OrganizationID: organizationId, SpaceID: spaceId, UnitID: unitId},
}));
const mutationOk = ok('spec.replicas: 3 -> 1\n');
const authOk = ok('Authenticated\n');
const contextResponse = (organizationRef = 'org-external-1', organizationName = 'stub-org') => ok(JSON.stringify({
  name: 'operator-context',
  coordinate: {user: 'operator@example.test', organizationID: organizationRef, serverURL: 'https://hub.example.test'},
  metadata: {organizationName},
}));
const organizationResponse = (
  organizationId = 'org-internal-1',
  externalOrganizationId = 'org-external-1',
  slug = 'stub-org',
) => ok(JSON.stringify({OrganizationID: organizationId, ExternalID: externalOrganizationId, Slug: slug}));

function inTempDir(fn) {
  const prev = process.cwd();
  const dir = mkdtempSync(join(tmpdir(), 'executor-test-'));
  process.chdir(dir);
  try {
    return fn(dir);
  } finally {
    process.chdir(prev);
  }
}

const NOW = () => new Date('2026-07-11T21:00:00Z');
const AUTHORITY = {
  objectUrl: 'https://hub.example.test/units/unit-id-1?org=org-external-1',
  server: 'https://hub.example.test',
  organizationId: 'org-internal-1',
  externalOrganizationId: 'org-external-1',
};
const FINDING = {
  id: 'nonprod-replicas:s-stage:statefulset-redis',
  rule: 'NONPROD_REPLICAS',
  recommendation: {
    action: {space: 's-stage', unit: 'statefulset-redis', function: 'set-replicas', args: ['1']},
  },
};

function preview(cub, overrides = {}) {
  return createPreview({
    finding: FINDING,
    boundAuthority: AUTHORITY,
    cub,
    now: NOW,
    contextResolver: () => ({ok: true, name: 'operator-context', server: AUTHORITY.server}),
    ...overrides,
  });
}

function grant(previewId, cub, overrides = {}) {
  return recordReview({previewId, reason: 'cost finding: non-prod replicas', cub, now: NOW, ...overrides});
}

test('preview accepts only a finding-owned whitelisted action', () => inTempDir(() => {
  const {cub} = fakeCub([]);
  assert.equal(createPreview({finding: {id: 'no-action'}, boundAuthority: AUTHORITY, cub}).reason, 'FINDING_NOT_ACTIONABLE');
  assert.equal(createPreview({
    finding: {...FINDING, recommendation: {action: {...FINDING.recommendation.action, function: 'delete-everything'}}},
    boundAuthority: AUTHORITY,
    cub,
  }).reason, 'FUNCTION_NOT_ALLOWED');
  assert.equal(createPreview({
    finding: {...FINDING, recommendation: {action: {...FINDING.recommendation.action, args: ['0']}}},
    boundAuthority: AUTHORITY,
    cub,
  }).reason, 'FUNCTION_ARGS_INVALID');
}));

test('preview binds the finding, authority, exact revision, and dry-run diff', () => inTempDir(() => {
  const {cub, calls} = fakeCub([unitResponse(7), mutationOk]);
  const created = preview(cub);
  assert.equal(created.verdict, 'PASS');
  assert.equal(created.preview.scope.revision, 'statefulset-redis/7');
  assert.equal(created.preview.findingId, FINDING.id);
  assert.ok(created.preview.fingerprint);
  assert.ok(existsSync(created.path));
  const dryRun = calls[1];
  assert.ok(dryRun.includes('--dry-run'));
  assert.equal(dryRun.includes('--unit'), false);
  assert.deepEqual(dryRun.slice(dryRun.indexOf('--revision'), dryRun.indexOf('--revision') + 2), ['--revision', 'statefulset-redis/7']);
  assert.deepEqual(dryRun.slice(-2), ['set-replicas', '1']);
}));

test('preview resolves and pins the active Cub context before authority reads or dry-run', () => inTempDir(() => {
  const {cub, calls} = fakeCub([contextResponse(), unitResponse(7), mutationOk]);
  const created = createPreview({finding: FINDING, boundAuthority: AUTHORITY, cub, now: NOW});
  assert.equal(created.verdict, 'PASS');
  assert.deepEqual(calls[0].slice(0, 2), ['context', 'get']);
  assert.deepEqual(calls[1].slice(0, 2), ['--context', 'operator-context']);
  assert.deepEqual(calls[2].slice(0, 2), ['--context', 'operator-context']);
  assert.equal(created.preview.authority.context, 'operator-context');
}));

test('preview refuses a target outside the bound organization', () => inTempDir(() => {
  const {cub} = fakeCub([unitResponse(7, 'other-org')]);
  const created = preview(cub);
  assert.equal(created.reason, 'ORG_MISMATCH');
}));

test('a missing cub executable is an ERROR rather than an operational blocker', () => inTempDir(() => {
  const created = preview(() => ({ok: false, commandUnavailable: true, stdout: '', stderr: 'spawn cub ENOENT'}));
  assert.equal(created.verdict, 'ERROR');
  assert.equal(created.reason, 'CUB_COMMAND_UNAVAILABLE');
  assert.equal(created.status, 'PREVIEW_ERROR');
}));

test('local review identity comes from authenticated Cub and stays on the preview revision', () => inTempDir(() => {
  const {cub, calls} = fakeCub([unitResponse(7), mutationOk, authOk, contextResponse(), organizationResponse(), headResponse(7)]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  assert.equal(recorded.verdict, 'WATCH');
  assert.equal(recorded.reason, 'LOCAL_REVIEW_RECORDED');
  assert.equal(recorded.review.recordedBy, 'operator@example.test');
  assert.equal(recorded.review.recordedContext, 'operator-context');
  assert.equal(recorded.review.previewFingerprint, created.preview.fingerprint);
  assert.equal(recorded.review.scope.headRevision, 7);
  assert.equal(recorded.review.executionPolicy, 'explicit-confirmation-plus-verified-receipt');
  assert.equal(recorded.review.expiresAt, '2026-07-11T21:15:00.000Z');
  assert.equal(recorded.review.idempotencyKey, `execute-${created.preview.id}-${created.preview.fingerprint}`);
  assert.ok(calls.every(call => call[0] === '--context' && call[1] === 'operator-context'));
}));

test('duplicate reviews stay uniquely addressable but share one execution idempotency key', () => inTempDir(() => {
  const {cub} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), headResponse(7),
  ]);
  const created = preview(cub);
  const first = grant(created.preview.id, cub);
  const second = grant(created.preview.id, cub);
  assert.notEqual(first.review.id, second.review.id);
  assert.equal(first.review.idempotencyKey, second.review.idempotencyKey);
}));

test('local review refuses auth, org, revision, and preview-integrity failures', () => {
  inTempDir(() => {
    const {cub} = fakeCub([unitResponse(7), mutationOk, () => ({ok: false, stdout: '', stderr: 'expired'})]);
    const created = preview(cub);
    assert.equal(grant(created.preview.id, cub).reason, 'AUTH_REQUIRED');
  });
  inTempDir(() => {
    const {cub} = fakeCub([
      unitResponse(7), mutationOk, authOk,
      contextResponse('other-external', 'other-org'),
      organizationResponse('other-internal', 'other-external', 'other-org'),
    ]);
    const created = preview(cub);
    assert.equal(grant(created.preview.id, cub).reason, 'ORG_MISMATCH');
  });
  inTempDir(() => {
    const {cub} = fakeCub([unitResponse(7), mutationOk, authOk, contextResponse(), organizationResponse(), headResponse(8)]);
    const created = preview(cub);
    assert.equal(grant(created.preview.id, cub).reason, 'PREVIEW_REVISION_DRIFT');
  });
  inTempDir(() => {
    const {cub} = fakeCub([unitResponse(7), mutationOk]);
    const created = preview(cub);
    const tampered = JSON.parse(readFileSync(created.path, 'utf8'));
    tampered.scope.args = ['9'];
    writeFileSync(created.path, JSON.stringify(tampered));
    assert.equal(grant(created.preview.id, cub).reason, 'PREVIEW_TAMPERED');
  });
});

test('local review alone cannot trigger a write', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, cub});
  assert.equal(executed.verdict, 'ASK');
  assert.equal(executed.reason, 'EXECUTION_CONFIRMATION_REQUIRED');
  assert.equal(calls.some(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set' && !call.includes('--dry-run')), false);
  assert.equal(existsSync(join('data', 'receipts', `${recorded.review.id}.json`)), false);
  assert.equal(JSON.parse(readFileSync(recorded.path, 'utf8')).status, 'recorded');
  assert.equal(reviewPacketStatus().reviewId, recorded.review.id);
}));

test('explicit commit writes one revision only when actual mutations match the reviewed dry run', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), unitResponse(7),
    mutationOk, unitResponse(8),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.verdict, 'PASS');
  assert.equal(executed.reason, 'CONFIG_REVISION_COMMITTED');
  assert.equal(executed.revisionBefore, 7);
  assert.equal(executed.revisionAfter, 8);
  assert.equal(executed.atomicity.status, 'WATCH');
  assert.match(executed.atomicity.platformIssue, /confighub\/issues\/4714$/);
  assert.equal(executed.delivery.status, 'WATCH');
  const writes = calls.filter(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set' && !call.includes('--dry-run'));
  assert.equal(writes.length, 1);
  assert.ok(writes[0].includes('--unit'));
  assert.deepEqual(writes[0].slice(0, 2), ['--context', 'operator-context']);
  const receipt = JSON.parse(readFileSync(executed.receipt, 'utf8'));
  assert.equal(receipt.reviewedMutations, 'spec.replicas: 3 -> 1');
  assert.equal(receipt.actualMutations, receipt.reviewedMutations);
  assert.equal(receipt.idempotencyKey, recorded.review.idempotencyKey);
  assert.equal(JSON.parse(readFileSync(recorded.path, 'utf8')).status, 'consumed');
  assert.equal(reviewPacketStatus().reason, 'LOCAL_UNSIGNED_RECEIPT_RECORDED');
  assert.equal(commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW}).reason, 'REVIEW_ALREADY_USED');
  receipt.actualMutations = 'forged mutation output';
  writeFileSync(executed.receipt, JSON.stringify(receipt));
  assert.equal(reviewPacketStatus().reason, 'REVIEW_PACKET_INVALID');
}));

test('commit refuses an expired exact review before calling the mutation', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({
    reviewId: recorded.review.id,
    confirmed: true,
    cub,
    now: () => new Date('2026-07-11T21:15:00.001Z'),
  });
  assert.equal(executed.verdict, 'BLOCK');
  assert.equal(executed.reason, 'REVIEW_EXPIRED');
  assert.equal(calls.some(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set' && !call.includes('--dry-run')), false);
}));

test('commit rechecks expiry immediately before claiming execution', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), unitResponse(7),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  let clockRead = 0;
  const executed = commitReviewed({
    reviewId: recorded.review.id,
    confirmed: true,
    cub,
    now: () => new Date(clockRead++ === 0 ? '2026-07-11T21:14:59.000Z' : '2026-07-11T21:15:00.001Z'),
  });
  assert.equal(executed.reason, 'REVIEW_EXPIRED');
  assert.equal(calls.some(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set' && !call.includes('--dry-run')), false);
}));

test('an existing execution claim prevents a second local process from writing', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), unitResponse(7),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  writeFileSync(join('data', 'reviews', `${recorded.review.idempotencyKey}.execution-claim`), 'claimed\n');
  assert.equal(reviewPacketStatus().reason, 'EXECUTION_RECONCILIATION_REQUIRED');
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.reason, 'REVIEW_EXECUTION_ALREADY_CLAIMED');
  assert.equal(calls.some(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set' && !call.includes('--dry-run')), false);
}));

test('commit refuses when the unit moved after local review', () => inTempDir(() => {
  const {cub} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(),
    unitResponse(9),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.reason, 'REVIEW_REVISION_DRIFT');
  assert.equal(executed.reviewedAtRevision, 7);
  assert.equal(executed.currentRevision, 9);
}));

test('commit refuses a deleted or recreated Unit even when its slug and revision match', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(),
    unitResponse(7, 'org-internal-1', 'space-id-1', 'replacement-unit-id'),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.reason, 'REVIEW_TARGET_REPLACED');
  assert.equal(calls.some(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set' && !call.includes('--dry-run')), false);
}));

test('commit requires the same authenticated identity that recorded the review', () => inTempDir(() => {
  const {cub} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk,
    contextResponse('other-external', 'other-org'),
    organizationResponse('other-internal', 'other-external', 'other-org'),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.reason, 'REVIEWER_IDENTITY_MISMATCH');
}));

test('unexpected revision sequencing creates an unverified receipt, never a false PASS', () => inTempDir(() => {
  const {cub, calls} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), unitResponse(7),
    mutationOk, unitResponse(9),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.status, 'COMMIT_UNVERIFIED');
  assert.equal(executed.reason, 'CONCURRENT_REVISION_DETECTED');
  assert.ok(existsSync(executed.receipt));
  assert.equal(JSON.parse(readFileSync(recorded.path, 'utf8')).status, 'execution-unverified');
  assert.equal(calls.filter(call => commandArgs(call)[0] === 'unit' && commandArgs(call)[1] === 'get').length, 4);
  assert.equal(calls.filter(call => commandArgs(call)[0] === 'function' && commandArgs(call)[1] === 'set').length, 2);
}));

test('actual mutation output must match the reviewed dry run', () => inTempDir(() => {
  const unexpectedMutation = ok('spec.replicas: 3 -> 2\n');
  const {cub} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), unitResponse(7),
    unexpectedMutation, unitResponse(8),
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.reason, 'MUTATION_DIFF_MISMATCH');
  assert.equal(executed.status, 'COMMIT_UNVERIFIED');
  const receipt = JSON.parse(readFileSync(executed.receipt, 'utf8'));
  assert.notEqual(receipt.actualMutations, receipt.reviewedMutations);
}));

test('a failed mutation attempt consumes the review into an unverified receipt', () => inTempDir(() => {
  const mutationFailed = () => ({ok: false, stdout: '', stderr: 'provider refused the write'});
  const {cub} = fakeCub([
    unitResponse(7), mutationOk,
    authOk, contextResponse(), organizationResponse(), headResponse(7),
    authOk, contextResponse(), organizationResponse(), unitResponse(7),
    mutationFailed,
  ]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const executed = commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW});
  assert.equal(executed.verdict, 'BLOCK');
  assert.equal(executed.reason, 'MUTATION_FAILED');
  assert.ok(existsSync(executed.receipt));
  assert.equal(JSON.parse(readFileSync(recorded.path, 'utf8')).status, 'execution-unverified');
  assert.equal(commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW}).reason, 'REVIEW_EXECUTION_UNVERIFIED');
  const retry = fakeCub([unitResponse(7), mutationOk, authOk, contextResponse(), organizationResponse(), headResponse(7)]);
  const retryPreview = preview(retry.cub);
  const retryReview = grant(retryPreview.preview.id, retry.cub);
  assert.notEqual(retryReview.review.idempotencyKey, recorded.review.idempotencyKey);
}));

test('status never hides a newer invalid review behind an older valid result', () => inTempDir(() => {
  const {cub} = fakeCub([unitResponse(7), mutationOk, authOk, contextResponse(), organizationResponse(), headResponse(7)]);
  const created = preview(cub);
  grant(created.preview.id, cub);
  writeFileSync(join('data', 'reviews', 'rev-99999999999999-invalid.json'), '{not-json');
  const status = reviewPacketStatus();
  assert.equal(status.reason, 'REVIEW_PACKET_INVALID');
  assert.equal(status.invalid[0].reason, 'REVIEW_UNPARSEABLE');
}));

test('artifact ids and review-preview linkage cannot traverse or be tampered', () => inTempDir(() => {
  assert.equal(commitReviewed({reviewId: 'rev-../../outside', confirmed: true}).reason, 'REVIEW_ID_INVALID');
  const {cub} = fakeCub([unitResponse(7), mutationOk, authOk, contextResponse(), organizationResponse(), headResponse(7)]);
  const created = preview(cub);
  const recorded = grant(created.preview.id, cub);
  const review = JSON.parse(readFileSync(recorded.path, 'utf8'));
  review.scope.args = ['9'];
  writeFileSync(recorded.path, JSON.stringify(review));
  assert.equal(commitReviewed({reviewId: recorded.review.id, confirmed: true, cub, now: NOW}).reason, 'REVIEW_PREVIEW_MISMATCH');
}));

test('whitelist stays closed', () => {
  assert.deepEqual(Object.keys(ALLOWED_FUNCTIONS).sort(), ['set-container-resources-defaults', 'set-replicas']);
});
