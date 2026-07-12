import assert from 'node:assert/strict';
import test from 'node:test';
import { classifyLiveBindings } from '../src/live-bindings.mjs';

function bound(overrides = {}) {
  return {
    schema: 'confighub.live-bindings.v1',
    configHub: {
      objectUrl: 'https://hub.example.test/units/unit-id?org=org-external',
      server: 'https://hub.example.test',
      organizationId: 'org-internal',
      externalOrganizationId: 'org-external',
    },
    action: {
      endpoint: 'function:set-replicas',
      contract: {
        kind: 'ConfigHub-governed-action.v0',
        operation: 'set non-production replicas',
        scopeFields: ['space', 'unit', 'revision'],
        requires: ['exact dry-run preview', 'ConfigHub approval'],
      },
    },
    approval: {objectId: 'change-set-1'},
    proof: {receiptObjectId: 'receipt-1'},
    runtime: {evidenceSource: 'controller-and-kubernetes-readback'},
    ...overrides,
  };
}

test('one classifier owns missing, migration, placeholder, and blocked states', () => {
  assert.equal(classifyLiveBindings(null).reason, 'LIVE_BINDINGS_MISSING');
  assert.equal(classifyLiveBindings({schema: 'confighub.live-bindings.v0'}).reason, 'LIVE_BINDINGS_MIGRATION_REQUIRED');
  assert.equal(classifyLiveBindings(bound({configHub: {...bound().configHub, objectUrl: '<unit-url>'}})).reason, 'LIVE_BINDINGS_PLACEHOLDER');
  assert.equal(classifyLiveBindings(bound({action: {...bound().action, endpoint: 'blocked:no-executor'}})).reason, 'LIVE_BINDINGS_BLOCKED');
});

test('contract values must be resolved, not merely non-empty', () => {
  for (const contract of [
    {...bound().action.contract, operation: 'blocked:no-operation'},
    {...bound().action.contract, scopeFields: ['space', '<unit>']},
    {...bound().action.contract, requires: ['exact preview', 'blocked:no-approval']},
  ]) {
    const result = classifyLiveBindings(bound({action: {...bound().action, contract}}));
    assert.equal(result.reason, 'LIVE_BINDINGS_CONTRACT_INVALID');
    assert.equal(result.reviewReady, false);
  }
});

test('review, commit, and live readiness remain distinct states', () => {
  const review = classifyLiveBindings(bound());
  assert.equal(review.reason, 'LIVE_BINDINGS_REVIEW_READY');
  assert.equal(review.reviewReady, true);
  assert.equal(review.executionReadyWithConfirmation, true);
  assert.equal(review.commitReady, false);
  assert.equal(review.liveReady, false);
  assert.equal(review.atomicity.reason, 'PROVIDER_ATOMIC_EXPECTED_REVISION_UNAVAILABLE');

  const pending = classifyLiveBindings(bound({approval: {}, proof: {}, runtime: {}}), {atomicExecutionAvailable: true});
  assert.equal(pending.reason, 'LIVE_BINDINGS_DELIVERY_PENDING');
  assert.equal(pending.commitReady, true);
  assert.equal(pending.liveReady, false);

  const malformed = classifyLiveBindings(bound({runtime: {evidenceSource: {claim: 'not-a-proof-reference'}}}), {atomicExecutionAvailable: true});
  assert.equal(malformed.reason, 'LIVE_BINDINGS_DELIVERY_PENDING');
  assert.ok(malformed.deliveryGaps.some(item => item.startsWith('runtime.evidenceSource=')));

  const unverified = classifyLiveBindings(bound(), {atomicExecutionAvailable: true});
  assert.equal(unverified.reason, 'LIVE_BINDINGS_EVIDENCE_UNVERIFIED');
  assert.equal(unverified.commitReady, true);
  assert.equal(unverified.liveReady, false);
});
