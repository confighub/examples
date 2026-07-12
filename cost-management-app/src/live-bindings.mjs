export const LIVE_BINDINGS_SCHEMA = 'confighub.live-bindings.v1';
export const ATOMIC_EXECUTION_AVAILABLE = false;

function hasPlaceholder(value) {
  if (typeof value === 'string') return value.includes('<') || value.includes('>') || value.includes('example-fill');
  if (Array.isArray(value)) return value.some(hasPlaceholder);
  if (value && typeof value === 'object') return Object.values(value).some(hasPlaceholder);
  return false;
}

function isBlockedValue(value) {
  return typeof value === 'string' && value.startsWith('blocked:');
}

function hasBlockedValue(value) {
  if (typeof value === 'string') return isBlockedValue(value);
  if (Array.isArray(value)) return value.some(hasBlockedValue);
  if (value && typeof value === 'object') return Object.values(value).some(hasBlockedValue);
  return false;
}

function outcome({status, verdict = 'WATCH', reason = status, reviewReady = false, executionReadyWithConfirmation = false, commitReady = false, liveReady = false, ...extra}) {
  return {
    status,
    verdict,
    reason,
    reviewReady,
    executionReadyWithConfirmation,
    commitReady,
    liveReady,
    readyForReview: reviewReady,
    readyForCommit: commitReady,
    readyForLive: liveReady,
    ...extra,
  };
}

export function classifyLiveBindings(bindings, {atomicExecutionAvailable = ATOMIC_EXECUTION_AVAILABLE} = {}) {
  if (!bindings) {
    return outcome({
      status: 'LIVE_BINDINGS_MISSING',
      message: 'data/live-bindings.json is not present.',
    });
  }
  if (bindings.schema !== LIVE_BINDINGS_SCHEMA) {
    return outcome({
      status: 'LIVE_BINDINGS_MIGRATION_REQUIRED',
      currentSchema: bindings.schema || '(missing)',
      expectedSchema: LIVE_BINDINGS_SCHEMA,
      nextGate: 'Run node lifecycle.mjs migrate --json before using these bindings.',
    });
  }

  const reviewFields = [
    ['configHub.objectUrl', bindings.configHub?.objectUrl],
    ['configHub.server', bindings.configHub?.server],
    ['configHub.organizationId', bindings.configHub?.organizationId],
    ['configHub.externalOrganizationId', bindings.configHub?.externalOrganizationId],
    ['action.endpoint', bindings.action?.endpoint],
  ];
  const missing = reviewFields.filter(([, value]) => !value).map(([field]) => field);
  const contract = bindings.action?.contract;
  if (!contract) missing.push('action.contract');
  if (missing.length) return outcome({status: 'LIVE_BINDINGS_INCOMPLETE', missing});

  const blocked = reviewFields.filter(([, value]) => isBlockedValue(value)).map(([field, value]) => `${field}=${value}`);
  if (blocked.length) {
    return outcome({
      status: 'LIVE_BINDINGS_BLOCKED',
      blocked,
      message: 'The ConfigHub review authority is explicitly blocked.',
    });
  }
  const placeholders = reviewFields.filter(([, value]) => hasPlaceholder(value)).map(([field]) => field);
  if (placeholders.length) {
    return outcome({
      status: 'LIVE_BINDINGS_PLACEHOLDER',
      placeholders,
      message: 'The ConfigHub review authority still contains example placeholders.',
    });
  }

  const contractProblems = [];
  if (contract.kind !== 'ConfigHub-governed-action.v0') contractProblems.push('action.contract.kind must be ConfigHub-governed-action.v0');
  if (!contract.operation) contractProblems.push('action.contract.operation is required');
  if (hasPlaceholder(contract.operation) || hasBlockedValue(contract.operation)) contractProblems.push('action.contract.operation must be a resolved value');
  if (!Array.isArray(contract.scopeFields) || contract.scopeFields.length === 0) contractProblems.push('action.contract.scopeFields must be a non-empty array');
  if (Array.isArray(contract.scopeFields) && contract.scopeFields.some(value => !String(value).trim() || hasPlaceholder(value) || hasBlockedValue(value))) contractProblems.push('action.contract.scopeFields must contain only resolved values');
  if (!Array.isArray(contract.requires) || contract.requires.length === 0) contractProblems.push('action.contract.requires must be a non-empty array');
  if (Array.isArray(contract.requires) && contract.requires.some(value => !String(value).trim() || hasPlaceholder(value) || hasBlockedValue(value))) contractProblems.push('action.contract.requires must contain only resolved values');
  if (contractProblems.length) return outcome({status: 'LIVE_BINDINGS_CONTRACT_INVALID', problems: contractProblems});

  const deliveryFields = [
    ['approval.objectId', bindings.approval?.objectId],
    ['proof.receiptObjectId', bindings.proof?.receiptObjectId],
    ['runtime.evidenceSource', bindings.runtime?.evidenceSource],
  ];
  const deliveryGaps = deliveryFields
    .filter(([, value]) => typeof value !== 'string' || !value.trim() || isBlockedValue(value) || hasPlaceholder(value))
    .map(([field, value]) => `${field}=${value || 'missing'}`);
  if (!atomicExecutionAvailable) {
    return outcome({
      status: 'LIVE_BINDINGS_REVIEW_READY',
      reviewReady: true,
      executionReadyWithConfirmation: true,
      deliveryGaps,
      atomicity: {
        status: 'WATCH',
        reason: 'PROVIDER_ATOMIC_EXPECTED_REVISION_UNAVAILABLE',
        platformIssue: 'https://github.com/confighubai/confighub/issues/4714',
      },
      message: deliveryGaps.length
        ? 'The exact review and explicitly confirmed CLI write path are bound. Provider atomicity and delivery proof remain separately visible.'
        : 'The exact review and explicitly confirmed CLI write path are bound. Provider-native atomic expected-revision enforcement remains WATCH.',
    });
  }
  if (deliveryGaps.length) {
    return outcome({
      status: 'LIVE_BINDINGS_DELIVERY_PENDING',
      reviewReady: true,
      executionReadyWithConfirmation: true,
      commitReady: true,
      deliveryGaps,
      message: 'Atomic execution is available, but approval, receipt, or runtime delivery proof remains open.',
    });
  }
  return outcome({
    status: 'LIVE_BINDINGS_EVIDENCE_UNVERIFIED',
    reviewReady: true,
    executionReadyWithConfirmation: true,
    commitReady: true,
    checked: [...reviewFields.map(([field]) => field), ...deliveryFields.map(([field]) => field)],
    message: 'The binding values are present, but raw references are not controller/runtime proof. A typed proof validator must verify them before this app is live.',
  });
}
