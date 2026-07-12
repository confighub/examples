import assert from 'node:assert/strict';
import test from 'node:test';
import { mkdtempSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { grantApproval, executeApproved, ALLOWED_FUNCTIONS } from '../src/executor.mjs';

// The executor is exercised against a scripted cub: each entry answers the
// next cub call. No network, no real org.
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

const headResponse = value => () => ({ok: true, stdout: `${value}\n`, stderr: ''});
const mutationOk = () => ({ok: true, stdout: 'spec.replicas: 3 -> 1\n', stderr: ''});

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

function grant(cub, overrides = {}) {
  return grantApproval({
    space: 's-stage',
    unit: 'statefulset-redis',
    functionName: 'set-replicas',
    args: ['1'],
    actor: 'operator-a',
    reason: 'cost finding: non-prod replicas',
    cub,
    now: NOW,
    ...overrides,
  });
}

test('grant refuses unknown functions and bad args', () => inTempDir(() => {
  const {cub} = fakeCub([]);
  assert.equal(grant(cub, {functionName: 'delete-everything'}).reason, 'FUNCTION_NOT_ALLOWED');
  assert.equal(grant(cub, {args: ['1', '2']}).reason, 'FUNCTION_ARGS_INVALID');
  assert.equal(grant(cub, {args: ['-3']}).reason, 'FUNCTION_ARGS_INVALID');
  assert.equal(grant(cub, {actor: ''}).reason, 'ACTOR_REQUIRED');
}));

test('grant pins the head revision it was approved against', () => inTempDir(() => {
  const {cub} = fakeCub([headResponse(7)]);
  const granted = grant(cub);
  assert.equal(granted.verdict, 'PASS');
  assert.equal(granted.approval.scope.headRevisionAtApproval, 7);
  assert.ok(existsSync(granted.path));
}));

test('execute runs the approved scope exactly and verifies the new revision', () => inTempDir(() => {
  const {cub, calls} = fakeCub([headResponse(7), headResponse(7), mutationOk, headResponse(8)]);
  const granted = grant(cub);
  const executed = executeApproved({approvalId: granted.approval.id, cub, now: NOW});
  assert.equal(executed.verdict, 'PASS');
  assert.equal(executed.reason, 'MUTATION_COMMITTED');
  assert.equal(executed.revisionBefore, 7);
  assert.equal(executed.revisionAfter, 8);
  const setCall = calls[2];
  assert.deepEqual(setCall.slice(0, 2), ['function', 'set']);
  assert.ok(setCall.includes('--change-desc'), 'mutations must carry a change description');
  assert.ok(setCall.includes('-o') && setCall.includes('mutations'), 'mutations must print their diff');
  assert.deepEqual(setCall.slice(-2), ['set-replicas', '1']);
  const receipt = JSON.parse(readFileSync(executed.receipt, 'utf8'));
  assert.equal(receipt.revisionAfter, 8);
  assert.ok(receipt.deliveryEvidence.startsWith('blocked:'), 'no runtime bound means no delivery claim');
  // Single use: a second execute refuses.
  const again = executeApproved({approvalId: granted.approval.id, cub, now: NOW});
  assert.equal(again.reason, 'APPROVAL_ALREADY_CONSUMED');
}));

test('execute refuses when the unit moved after approval', () => inTempDir(() => {
  const {cub} = fakeCub([headResponse(7), headResponse(9)]);
  const granted = grant(cub);
  const executed = executeApproved({approvalId: granted.approval.id, cub, now: NOW});
  assert.equal(executed.reason, 'APPROVAL_REVISION_DRIFT');
  assert.equal(executed.approvedAtRevision, 7);
  assert.equal(executed.currentRevision, 9);
}));

test('a mutation that makes no new revision is a silent skip, not a success', () => inTempDir(() => {
  const {cub} = fakeCub([headResponse(7), headResponse(7), mutationOk, headResponse(7)]);
  const granted = grant(cub);
  const executed = executeApproved({approvalId: granted.approval.id, cub, now: NOW});
  assert.equal(executed.verdict, 'BLOCK');
  assert.equal(executed.reason, 'MUTATION_SILENT_SKIP');
  assert.ok(!existsSync('data/receipts'), 'a silent skip must not produce a receipt');
}));

test('execute refuses unknown approvals and whitelist bypass attempts', () => inTempDir(() => {
  const {cub} = fakeCub([headResponse(7)]);
  const granted = grant(cub);
  assert.equal(executeApproved({approvalId: 'apr-nope', cub}).reason, 'APPROVAL_NOT_FOUND');
  // Tamper the stored approval to smuggle a non-whitelisted function.
  const path = granted.path;
  const tampered = JSON.parse(readFileSync(path, 'utf8'));
  tampered.scope.function = 'yq-i';
  writeFileSync(path, JSON.stringify(tampered));
  assert.equal(executeApproved({approvalId: granted.approval.id, cub}).reason, 'FUNCTION_NOT_ALLOWED');
}));

test('whitelist stays closed', () => {
  assert.deepEqual(Object.keys(ALLOWED_FUNCTIONS).sort(), ['set-container-resources-defaults', 'set-replicas']);
});
