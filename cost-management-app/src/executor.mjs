// Governed action reviewer: one finding, one exact preview,
// one authenticated, expiring local review record, and one explicit execution request.
// Provider atomicity remains visible as WATCH until ConfigHub exposes compare-and-swap.
import { spawnSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

export const PREVIEW_SCHEMA = 'confighub.action-preview.v0';
export const REVIEW_SCHEMA = 'confighub.local-review.v0';
export const RECEIPT_SCHEMA = 'confighub.receipt.v0';
export const PREVIEWS_DIR = 'data/previews';
export const REVIEWS_DIR = 'data/reviews';
export const RECEIPTS_DIR = 'data/receipts';
export const REVIEW_TTL_MS = 15 * 60 * 1000;

// Only functions earned by a receipted scenario class ship here. Operator
// input can choose a generated finding; it cannot expand this whitelist.
export const ALLOWED_FUNCTIONS = Object.freeze({
  'set-replicas': {argCount: 1, argPattern: /^[1-9]\d*$/},
  'set-container-resources-defaults': {argCount: 0, argPattern: null},
});

function result(verdict, reason, status, extra = {}) {
  return {verdict, reason, status, ...extra};
}

function safeArtifactId(value, prefix) {
  return typeof value === 'string'
    && value.startsWith(prefix)
    && /^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$/.test(value);
}

function normalized(value) {
  return String(value || '')
    .replace(/\r\n/g, '\n')
    .split('\n')
    .map(line => line.replace(/\s+$/g, ''))
    .join('\n')
    .trim();
}

function digest(value) {
  return createHash('sha256').update(String(value)).digest('hex');
}

function actionValidation(functionName, args) {
  const spec = ALLOWED_FUNCTIONS[functionName];
  if (!spec) {
    return result('BLOCK', 'FUNCTION_NOT_ALLOWED', 'ACTION_REFUSED', {
      message: `${functionName || '(missing function)'} is not in this app's whitelist: ${Object.keys(ALLOWED_FUNCTIONS).join(', ') || '(empty: no governed mutations are proven for this scenario class yet)'}`,
    });
  }
  const argList = Array.isArray(args) ? args.map(String) : [];
  if (argList.length !== spec.argCount || (spec.argPattern && !argList.every(value => spec.argPattern.test(value)))) {
    return result('BLOCK', 'FUNCTION_ARGS_INVALID', 'ACTION_REFUSED', {
      message: `${functionName} expects ${spec.argCount} argument(s)${spec.argPattern ? ` matching ${spec.argPattern}` : ''}`,
    });
  }
  return {verdict: 'PASS', spec, args: argList};
}

function previewFingerprint(preview) {
  return digest(JSON.stringify({
    schema: preview.schema,
    findingId: preview.findingId,
    authority: preview.authority,
    scope: preview.scope,
    expectedMutations: normalized(preview.expectedMutations),
  }));
}

function receiptFingerprint(receipt) {
  return digest(JSON.stringify({
    schema: receipt.schema,
    verdict: receipt.verdict,
    reason: receipt.reason,
    status: receipt.status,
    evidenceClass: receipt.evidenceClass,
    reviewId: receipt.reviewId,
    idempotencyKey: receipt.idempotencyKey,
    previewId: receipt.previewId,
    actor: receipt.actor,
    scope: receipt.scope,
    reviewedMutations: normalized(receipt.reviewedMutations),
    actualMutations: normalized(receipt.actualMutations),
    revisionBefore: receipt.revisionBefore,
    revisionAfter: receipt.revisionAfter,
    targetIdentity: receipt.targetIdentity,
    atomicity: receipt.atomicity,
    delivery: receipt.delivery,
    executedAt: receipt.executedAt,
  }));
}

export function runCub(cubArgs) {
  const proc = spawnSync('cub', cubArgs, {encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, timeout: 30_000});
  return {
    ok: !proc.error && proc.status === 0,
    commandUnavailable: Boolean(proc.error && proc.error.code === 'ENOENT'),
    stdout: proc.stdout || '',
    stderr: String(proc.stderr || proc.error || ''),
  };
}

export function readHeadRevision(space, unit, cub = runCub) {
  const read = cub(['unit', 'get', '--space', space, unit, '-o', 'jq=.Unit.HeadRevisionNum']);
  if (!read.ok) return {ok: false, reason: read.commandUnavailable ? 'CUB_COMMAND_UNAVAILABLE' : 'UNIT_READ_FAILED', detail: read.stderr.slice(0, 300)};
  const head = Number(String(read.stdout).trim());
  if (!Number.isInteger(head) || head < 1) return {ok: false, detail: `unparseable head revision: ${read.stdout.slice(0, 80)}`};
  return {ok: true, head};
}

export function readUnitAuthority(space, unit, cub = runCub) {
  const read = cub(['unit', 'get', '--space', space, unit, '-o', 'json']);
  if (!read.ok) return {ok: false, reason: read.commandUnavailable ? 'CUB_COMMAND_UNAVAILABLE' : 'UNIT_AUTHORITY_READ_FAILED', detail: read.stderr.slice(0, 300)};
  let payload;
  try {
    payload = JSON.parse(read.stdout);
  } catch {
    return {ok: false, detail: `unparseable Unit response: ${read.stdout.slice(0, 120)}`};
  }
  const entity = payload.Unit || payload.unit || payload;
  const head = Number(entity.HeadRevisionNum);
  const organizationId = String(entity.OrganizationID || payload.OrganizationID || '');
  const spaceId = String(entity.SpaceID || payload.SpaceID || '');
  const unitId = String(entity.UnitID || entity.ID || payload.UnitID || '');
  if (!Number.isInteger(head) || head < 1 || !organizationId || !spaceId || !unitId) {
    return {ok: false, detail: 'Unit response lacks HeadRevisionNum, OrganizationID, SpaceID, or UnitID'};
  }
  return {
    ok: true,
    head,
    organizationId,
    spaceId,
    unitId,
  };
}

function authenticatedIdentity(cub = runCub) {
  const auth = cub(['auth', 'status']);
  if (!auth.ok) return {ok: false, reason: auth.commandUnavailable ? 'CUB_COMMAND_UNAVAILABLE' : 'AUTH_REQUIRED', detail: auth.stderr.slice(0, 300)};
  const context = cub(['context', 'get', '-o', 'json']);
  if (!context.ok) return {ok: false, reason: 'CONTEXT_READ_FAILED', detail: context.stderr.slice(0, 300)};
  let payload;
  try {
    payload = JSON.parse(context.stdout);
  } catch {
    return {ok: false, reason: 'CONTEXT_UNPARSEABLE', detail: context.stdout.slice(0, 120)};
  }
  const user = String(payload.coordinate?.user || '');
  const contextOrgRef = String(payload.coordinate?.organizationID || '');
  const contextOrgName = String(payload.metadata?.organizationName || '');
  const server = String(payload.coordinate?.serverURL || '').replace(/\/+$/, '');
  const orgRef = contextOrgName || contextOrgRef;
  if (!user || !orgRef || !server) {
    return {ok: false, reason: 'CONTEXT_IDENTITY_MISSING', detail: 'active context lacks user, organization reference, or server URL'};
  }
  const orgRead = cub(['organization', 'get', orgRef, '-o', 'json']);
  if (!orgRead.ok) return {ok: false, reason: 'ORG_READ_FAILED', detail: orgRead.stderr.slice(0, 300)};
  let orgPayload;
  try {
    orgPayload = JSON.parse(orgRead.stdout);
  } catch {
    return {ok: false, reason: 'ORG_UNPARSEABLE', detail: orgRead.stdout.slice(0, 120)};
  }
  const org = orgPayload.Organization || orgPayload.organization || orgPayload;
  const organizationId = String(org.OrganizationID || org.ID || '');
  const externalOrganizationId = String(org.ExternalID || '');
  const orgSlug = String(org.Slug || '');
  if (!organizationId || !externalOrganizationId) {
    return {ok: false, reason: 'ORG_IDENTITY_INCOMPLETE', detail: 'organization response lacks internal or external id'};
  }
  const contextMatches = [organizationId, externalOrganizationId].includes(contextOrgRef)
    || [orgSlug, String(org.DisplayName || '')].filter(Boolean).includes(contextOrgName);
  if (!contextMatches) {
    return {ok: false, reason: 'ORG_MISMATCH', detail: 'active context does not match the resolved organization'};
  }
  return {
    ok: true,
    user,
    organizationId,
    externalOrganizationId,
    orgSlug,
    context: String(payload.name || ''),
    server,
  };
}

function activeContext(cub = runCub) {
  const context = cub(['context', 'get', '-o', 'json']);
  if (!context.ok) return {ok: false, reason: context.commandUnavailable ? 'CUB_COMMAND_UNAVAILABLE' : 'CONTEXT_READ_FAILED', detail: context.stderr.slice(0, 300)};
  let payload;
  try {
    payload = JSON.parse(context.stdout);
  } catch {
    return {ok: false, reason: 'CONTEXT_UNPARSEABLE', detail: context.stdout.slice(0, 120)};
  }
  const name = String(payload.name || '');
  const server = String(payload.coordinate?.serverURL || '').replace(/\/+$/, '');
  if (!name || !server) return {ok: false, reason: 'CONTEXT_IDENTITY_MISSING', detail: 'active context lacks name or server URL'};
  return {ok: true, name, server};
}

function loadPreview(previewId) {
  if (!safeArtifactId(previewId, 'prv-')) return {ok: false, reason: 'PREVIEW_ID_INVALID'};
  const path = join(PREVIEWS_DIR, `${previewId}.json`);
  let preview;
  try {
    preview = JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    return {ok: false, reason: 'PREVIEW_NOT_FOUND', path};
  }
  if (preview.schema !== PREVIEW_SCHEMA || preview.id !== previewId || !preview.scope) {
    return {ok: false, reason: 'PREVIEW_INVALID', path};
  }
  if (preview.fingerprint !== previewFingerprint(preview)) {
    return {ok: false, reason: 'PREVIEW_TAMPERED', path};
  }
  return {ok: true, preview, path};
}

export function createPreview({finding, boundAuthority, cub = runCub, now = () => new Date(), contextResolver = activeContext}) {
  const action = finding?.recommendation?.action;
  if (!finding?.id || !action) {
    return result('BLOCK', 'FINDING_NOT_ACTIONABLE', 'PREVIEW_BLOCKED', {
      message: 'This finding has no governed action. Keep it as review guidance.',
    });
  }
  const validated = actionValidation(action.function, action.args);
  if (validated.verdict !== 'PASS') return {...validated, status: 'PREVIEW_BLOCKED'};
  const authorityFields = ['objectUrl', 'server', 'organizationId', 'externalOrganizationId'];
  const authorityMissing = authorityFields.filter(field => {
    const value = boundAuthority?.[field];
    return !value || String(value).startsWith('blocked:') || String(value).includes('<') || String(value).includes('>');
  });
  if (authorityMissing.length) {
    return result('BLOCK', 'LIVE_AUTHORITY_REQUIRED', 'PREVIEW_BLOCKED', {missing: authorityMissing});
  }
  const context = contextResolver(cub);
  if (!context.ok) return result(context.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'ERROR' : 'BLOCK', context.reason, context.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'PREVIEW_ERROR' : 'PREVIEW_BLOCKED', {detail: context.detail});
  if (context.server !== String(boundAuthority.server).replace(/\/+$/, '')) {
    return result('BLOCK', 'SERVER_MISMATCH', 'PREVIEW_BLOCKED', {
      expectedServer: boundAuthority.server,
      actualServer: context.server,
    });
  }
  const pinnedCub = cubArgs => cub(['--context', context.name, ...cubArgs]);
  const target = readUnitAuthority(action.space, action.unit, pinnedCub);
  if (!target.ok) return result(
    target.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'ERROR' : 'BLOCK',
    target.reason || 'UNIT_AUTHORITY_READ_FAILED',
    target.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'PREVIEW_ERROR' : 'PREVIEW_BLOCKED',
    {detail: target.detail},
  );
  if (target.organizationId !== String(boundAuthority.organizationId)) {
    return result('BLOCK', 'ORG_MISMATCH', 'PREVIEW_BLOCKED', {
      expectedOrganizationId: String(boundAuthority.organizationId),
      actualOrganizationId: target.organizationId,
      message: 'The finding points at a Unit outside the bound ConfigHub organization.',
    });
  }
  const revision = `${action.unit}/${target.head}`;
  const dryRun = pinnedCub([
    'function', 'set',
    '--space', action.space,
    '--revision', revision,
    '--dry-run',
    '-o', 'mutations',
    '--', action.function, ...validated.args,
  ]);
  if (!dryRun.ok) return result('BLOCK', 'PREVIEW_FAILED', 'PREVIEW_BLOCKED', {detail: dryRun.stderr.slice(0, 500)});
  const expectedMutations = normalized(dryRun.stdout);
  if (!expectedMutations) return result('BLOCK', 'PREVIEW_EMPTY', 'PREVIEW_BLOCKED', {message: 'The dry-run produced no mutation to review.'});

  const createdAt = now().toISOString();
  const preview = {
    schema: PREVIEW_SCHEMA,
    id: '',
    findingId: finding.id,
    findingRule: finding.rule,
    createdAt,
    authority: {
      objectUrl: boundAuthority.objectUrl,
      server: boundAuthority.server,
      organizationId: String(boundAuthority.organizationId),
      externalOrganizationId: String(boundAuthority.externalOrganizationId),
      context: context.name,
      spaceId: target.spaceId,
      unitId: target.unitId,
    },
    scope: {
      space: action.space,
      unit: action.unit,
      function: action.function,
      args: validated.args,
      headRevision: target.head,
      revision,
    },
    expectedMutations,
  };
  preview.fingerprint = previewFingerprint(preview);
  preview.id = `prv-${createdAt.replace(/[-:TZ.]/g, '')}-${String(action.unit).slice(0, 24)}-${preview.fingerprint.slice(0, 8)}-${randomUUID().slice(0, 8)}`;
  mkdirSync(PREVIEWS_DIR, {recursive: true});
  const path = join(PREVIEWS_DIR, `${preview.id}.json`);
  writeFileSync(path, `${JSON.stringify(preview, null, 2)}\n`);
  return result('PASS', 'PREVIEW_CREATED', 'PREVIEW_READY', {preview, path});
}

export function recordReview({previewId, reason, cub = runCub, now = () => new Date()}) {
  if (!previewId) return result('BLOCK', 'PREVIEW_REQUIRED', 'REVIEW_NOT_RECORDED', {message: 'pass --preview <id>'});
  const loaded = loadPreview(previewId);
  if (!loaded.ok) return result('BLOCK', loaded.reason, 'REVIEW_NOT_RECORDED', {previewId, path: loaded.path});
  const {preview} = loaded;
  const pinnedCub = cubArgs => cub(['--context', preview.authority.context, ...cubArgs]);
  const identity = authenticatedIdentity(pinnedCub);
  if (!identity.ok) return result(identity.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'ERROR' : 'BLOCK', identity.reason, identity.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'REVIEW_ERROR' : 'REVIEW_NOT_RECORDED', {detail: identity.detail});
  if (identity.organizationId !== preview.authority.organizationId) {
    return result('BLOCK', 'ORG_MISMATCH', 'REVIEW_NOT_RECORDED', {
      expectedOrganizationId: preview.authority.organizationId,
      actualOrganizationId: identity.organizationId,
    });
  }
  if (identity.server !== String(preview.authority.server).replace(/\/+$/, '')) {
    return result('BLOCK', 'SERVER_MISMATCH', 'REVIEW_NOT_RECORDED', {
      expectedServer: preview.authority.server,
      actualServer: identity.server,
    });
  }
  if (identity.context !== preview.authority.context) {
    return result('BLOCK', 'CONTEXT_MISMATCH', 'REVIEW_NOT_RECORDED', {
      expectedContext: preview.authority.context,
      actualContext: identity.context,
    });
  }
  const current = readHeadRevision(preview.scope.space, preview.scope.unit, pinnedCub);
  if (!current.ok) return result(current.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'ERROR' : 'BLOCK', current.reason || 'UNIT_READ_FAILED', current.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'REVIEW_ERROR' : 'REVIEW_NOT_RECORDED', {detail: current.detail});
  if (current.head !== preview.scope.headRevision) {
    return result('BLOCK', 'PREVIEW_REVISION_DRIFT', 'REVIEW_NOT_RECORDED', {
      previewRevision: preview.scope.headRevision,
      currentRevision: current.head,
      message: 'The Unit changed after preview. Generate and review a new preview.',
    });
  }

  const recordedAt = now().toISOString();
  const expiresAt = new Date(Date.parse(recordedAt) + REVIEW_TTL_MS).toISOString();
  const id = `rev-${recordedAt.replace(/[-:TZ.]/g, '')}-${preview.scope.unit.slice(0, 24)}-${preview.fingerprint.slice(0, 8)}-${randomUUID().slice(0, 8)}`;
  const review = {
    schema: REVIEW_SCHEMA,
    id,
    kind: 'local-review-evidence',
    status: 'recorded',
    recordedBy: identity.user,
    recordedContext: identity.context,
    server: identity.server,
    organizationId: identity.organizationId,
    reviewReason: reason || '',
    recordedAt,
    expiresAt,
    previewId: preview.id,
    previewFingerprint: preview.fingerprint,
    idempotencyKey: `execute-${preview.id}-${preview.fingerprint}`,
    scope: preview.scope,
    executionPolicy: 'explicit-confirmation-plus-verified-receipt',
    boundReceipt: null,
  };
  mkdirSync(REVIEWS_DIR, {recursive: true});
  const path = join(REVIEWS_DIR, `${id}.json`);
  writeFileSync(path, `${JSON.stringify(review, null, 2)}\n`);
  return result('WATCH', 'LOCAL_REVIEW_RECORDED', 'LOCAL_REVIEW_RECORDED', {
    review,
    path,
    message: 'The exact preview was recorded as local review evidence. This is not ConfigHub approval or permission to mutate.',
  });
}

export function reviewPacketStatus() {
  let entries = [];
  try {
    entries = readdirSync(REVIEWS_DIR)
      .filter(name => name.endsWith('.json'))
      .map(name => ({name, order: statSync(join(REVIEWS_DIR, name), {bigint: true}).mtimeNs}))
      .sort((left, right) => right.order > left.order ? 1 : (right.order < left.order ? -1 : right.name.localeCompare(left.name)))
      .map(entry => entry.name);
  } catch {
    entries = [];
  }
  const invalid = [];
  // Status follows the newest review attempt. Never skip a newer invalid or
  // interrupted attempt and surface an older success instead.
  for (const name of entries.slice(0, 1)) {
    const path = join(REVIEWS_DIR, name);
    let review;
    try {
      review = JSON.parse(readFileSync(path, 'utf8'));
    } catch {
      invalid.push({path, reason: 'REVIEW_UNPARSEABLE'});
      continue;
    }
    if (review.schema !== REVIEW_SCHEMA || review.kind !== 'local-review-evidence'
        || !['recorded', 'execution-in-progress', 'consumed', 'execution-unverified'].includes(review.status) || !review.scope || !review.previewId
        || !review.recordedContext || !review.server || !review.expiresAt || Number.isNaN(Date.parse(review.expiresAt))
        || review.idempotencyKey !== `execute-${review.previewId}-${review.previewFingerprint}`
        || review.id !== name.slice(0, -5)) {
      invalid.push({path, reason: 'REVIEW_INVALID'});
      continue;
    }
    const loaded = loadPreview(review.previewId);
    if (!loaded.ok || review.previewFingerprint !== loaded.preview?.fingerprint
        || JSON.stringify(review.scope) !== JSON.stringify(loaded.preview?.scope)) {
      invalid.push({path, reason: loaded.reason || 'REVIEW_PREVIEW_MISMATCH'});
      continue;
    }
    const executionClaimPath = join(REVIEWS_DIR, `${review.idempotencyKey}.execution-claim`);
    if (review.status === 'recorded' && existsSync(executionClaimPath)) {
      return result('BLOCK', 'EXECUTION_RECONCILIATION_REQUIRED', 'COMMIT_UNVERIFIED', {
        reviewId: review.id,
        previewId: review.previewId,
        executionClaimPath,
        path,
        message: 'An execution claim exists without a terminal receipt. Reconcile the ConfigHub Unit before any retry.',
      });
    }
    if (review.status === 'execution-in-progress') {
      return result('BLOCK', 'EXECUTION_RECONCILIATION_REQUIRED', 'COMMIT_UNVERIFIED', {
        reviewId: review.id,
        previewId: review.previewId,
        reviewedBy: review.recordedBy,
        expectedRevision: review.scope.headRevision,
        path,
        attemptedAt: review.attemptedAt,
        message: 'Execution started but no terminal receipt is linked. Reconcile the ConfigHub Unit before creating another preview.',
      });
    }
    if (review.status !== 'recorded') {
      const expectedReceiptPath = join(RECEIPTS_DIR, `${review.id}.json`);
      if (review.boundReceipt !== expectedReceiptPath) {
        invalid.push({path, reason: 'REVIEW_RECEIPT_LINK_INVALID'});
        continue;
      }
      let receipt;
      try {
        receipt = JSON.parse(readFileSync(expectedReceiptPath, 'utf8'));
      } catch {
        invalid.push({path, reason: 'REVIEW_RECEIPT_MISSING'});
        continue;
      }
      const receiptMatchesReview = receipt.schema === RECEIPT_SCHEMA
        && receipt.evidenceClass === 'local-unsigned-execution-receipt'
        && receipt.reviewId === review.id
        && receipt.previewId === review.previewId
        && receipt.idempotencyKey === review.idempotencyKey
        && receipt.actor === review.recordedBy
        && JSON.stringify(receipt.scope) === JSON.stringify(review.scope)
        && normalized(receipt.reviewedMutations) === normalized(loaded.preview.expectedMutations)
        && receipt.fingerprint === receiptFingerprint(receipt)
        && ['PASS', 'BLOCK'].includes(receipt.verdict);
      const passReceiptValid = receipt.verdict !== 'PASS'
        || (receipt.revisionBefore === review.scope.headRevision
          && receipt.revisionAfter === receipt.revisionBefore + 1
          && normalized(receipt.actualMutations) === normalized(receipt.reviewedMutations));
      if (!receiptMatchesReview || !passReceiptValid) {
        invalid.push({path, reason: 'REVIEW_RECEIPT_INVALID'});
        continue;
      }
      const reloadedVerdict = receipt.verdict === 'PASS' ? 'WATCH' : receipt.verdict;
      const reloadedReason = receipt.verdict === 'PASS' ? 'LOCAL_UNSIGNED_RECEIPT_RECORDED' : receipt.reason;
      const reloadedStatus = receipt.verdict === 'PASS' ? 'RECEIPT_RECORDED' : receipt.status;
      return result(reloadedVerdict, reloadedReason, reloadedStatus, {
        recordedOutcome: {verdict: receipt.verdict, reason: receipt.reason, status: receipt.status},
        reviewId: review.id,
        previewId: review.previewId,
        reviewedBy: review.recordedBy,
        expectedRevision: review.scope.headRevision,
        reviewedMutations: normalized(loaded.preview.expectedMutations),
        path,
        receiptPath: expectedReceiptPath,
        receipt,
        message: receipt.verdict === 'PASS'
          ? 'The unsigned local receipt records a ConfigHub revision and mutation parity from the execution run. Reloading it is WATCH, not fresh server proof; atomic provider enforcement and delivery remain separately visible.'
          : 'An execution attempt occurred but did not satisfy the reviewed receipt contract. Inspect before any further action.',
      });
    }
    return result('WATCH', 'LOCAL_REVIEW_RECORDED', 'LOCAL_REVIEW_RECORDED', {
      reviewId: review.id,
      previewId: review.previewId,
      reviewedBy: review.recordedBy,
      expectedRevision: review.scope.headRevision,
      reviewedMutations: normalized(loaded.preview.expectedMutations),
      path,
      message: 'A valid local review record exists. It is evidence only; explicit execution confirmation, provider atomicity, and live proof remain open.',
    });
  }
  const reason = entries.length ? 'REVIEW_PACKET_INVALID' : 'REVIEW_PACKET_MISSING';
  return result('WATCH', reason, reason, {
    invalid,
    message: entries.length
      ? 'No stored local review record matches an intact exact preview.'
      : 'No exact preview has been recorded as local review evidence.',
  });
}

export function commitReviewed({reviewId, confirmed = false, cub = runCub, now = () => new Date()}) {
  if (!reviewId) return result('BLOCK', 'LOCAL_REVIEW_REQUIRED', 'COMMIT_BLOCKED', {message: 'pass --review <id>'});
  if (!confirmed) return result('ASK', 'EXECUTION_CONFIRMATION_REQUIRED', 'COMMIT_AWAITING_CONFIRMATION', {
    reviewId,
    message: 'Review the exact diff, then rerun with --confirm-execute. That confirmation requests the write; the local review record alone does not.',
  });
  if (!safeArtifactId(reviewId, 'rev-')) return result('BLOCK', 'REVIEW_ID_INVALID', 'COMMIT_BLOCKED', {reviewId});
  const path = join(REVIEWS_DIR, `${reviewId}.json`);
  let review;
  try {
    review = JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    return result('BLOCK', 'REVIEW_NOT_FOUND', 'COMMIT_BLOCKED', {reviewId, path});
  }
  if (review.schema !== REVIEW_SCHEMA || review.id !== reviewId || review.kind !== 'local-review-evidence'
      || !review.scope || !review.previewId || !review.recordedContext || !review.server || !review.expiresAt || Number.isNaN(Date.parse(review.expiresAt))
      || review.idempotencyKey !== `execute-${review.previewId}-${review.previewFingerprint}`) {
    return result('BLOCK', 'REVIEW_INVALID', 'COMMIT_BLOCKED', {reviewId});
  }
  if (review.status !== 'recorded') {
    const usedReason = review.status === 'consumed'
      ? 'REVIEW_ALREADY_USED'
      : (review.status === 'execution-in-progress' ? 'EXECUTION_RECONCILIATION_REQUIRED' : 'REVIEW_EXECUTION_UNVERIFIED');
    return result('BLOCK', usedReason, 'COMMIT_BLOCKED', {
      reviewId,
      boundReceipt: review.boundReceipt,
      message: 'This review is single-use. Create a new preview and review before another execution request.',
    });
  }
  const executionTime = now();
  if (executionTime.getTime() > Date.parse(review.expiresAt)) {
    return result('BLOCK', 'REVIEW_EXPIRED', 'COMMIT_BLOCKED', {
      reviewId,
      expiresAt: review.expiresAt,
      message: 'The exact review expired. Re-read the Unit, create a new preview, and review the current state.',
    });
  }
  const loaded = loadPreview(review.previewId);
  if (!loaded.ok) return result('BLOCK', loaded.reason, 'COMMIT_BLOCKED', {reviewId, previewId: review.previewId});
  const {preview} = loaded;
  if (review.previewFingerprint !== preview.fingerprint
      || JSON.stringify(review.scope) !== JSON.stringify(preview.scope)) {
    return result('BLOCK', 'REVIEW_PREVIEW_MISMATCH', 'COMMIT_BLOCKED', {reviewId, previewId: preview.id});
  }
  const pinnedCub = cubArgs => cub(['--context', review.recordedContext, ...cubArgs]);
  const executorIdentity = authenticatedIdentity(pinnedCub);
  if (!executorIdentity.ok) {
    return result(executorIdentity.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'ERROR' : 'BLOCK', executorIdentity.reason, executorIdentity.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'COMMIT_ERROR' : 'COMMIT_BLOCKED', {reviewId, detail: executorIdentity.detail});
  }
  if (executorIdentity.user !== review.recordedBy || executorIdentity.organizationId !== review.organizationId
      || executorIdentity.server !== review.server || executorIdentity.context !== review.recordedContext) {
    return result('BLOCK', 'REVIEWER_IDENTITY_MISMATCH', 'COMMIT_BLOCKED', {
      reviewId,
      reviewedBy: review.recordedBy,
      executingAs: executorIdentity.user,
      reviewedOrganizationId: review.organizationId,
      executingOrganizationId: executorIdentity.organizationId,
      reviewedServer: review.server,
      executingServer: executorIdentity.server,
      executingContext: executorIdentity.context,
    });
  }

  const {space, unit, function: functionName, args, headRevision} = review.scope;
  const validated = actionValidation(functionName, args);
  if (validated.verdict !== 'PASS') return {...validated, status: 'COMMIT_BLOCKED', reviewId};
  const before = readUnitAuthority(space, unit, pinnedCub);
  if (!before.ok) return result(before.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'ERROR' : 'BLOCK', before.reason || 'UNIT_AUTHORITY_READ_FAILED', before.reason === 'CUB_COMMAND_UNAVAILABLE' ? 'COMMIT_ERROR' : 'COMMIT_BLOCKED', {detail: before.detail});
  if (before.organizationId !== preview.authority.organizationId
      || before.spaceId !== preview.authority.spaceId
      || before.unitId !== preview.authority.unitId) {
    return result('BLOCK', 'REVIEW_TARGET_REPLACED', 'COMMIT_BLOCKED', {
      reviewId,
      reviewedIdentity: {
        organizationId: preview.authority.organizationId,
        spaceId: preview.authority.spaceId,
        unitId: preview.authority.unitId,
      },
      currentIdentity: {
        organizationId: before.organizationId,
        spaceId: before.spaceId,
        unitId: before.unitId,
      },
      message: 'The reviewed Unit identity changed. Create a new preview for the current object before any write.',
    });
  }
  if (before.head !== headRevision) {
    return result('BLOCK', 'REVIEW_REVISION_DRIFT', 'COMMIT_BLOCKED', {
      reviewId,
      reviewedAtRevision: headRevision,
      currentRevision: before.head,
      message: 'The Unit changed after local review; preview and review the current revision again.',
    });
  }

  const changeDesc = [
    `${functionName} ${args.join(' ')} on ${space}/${unit} from exact review ${review.id}`,
    '',
    `Execution request: explicit commit confirmation by authenticated user ${review.recordedBy}${review.reviewReason ? ` — ${review.reviewReason}` : ''}`,
    `Clarifications: previewed revision ${headRevision}; reviewed diff ${preview.fingerprint}; closed executor whitelist ${Object.keys(ALLOWED_FUNCTIONS).join(', ')}`,
  ].join('\n');
  const finalExecutionTime = now();
  if (finalExecutionTime.getTime() > Date.parse(review.expiresAt)) {
    return result('BLOCK', 'REVIEW_EXPIRED', 'COMMIT_BLOCKED', {
      reviewId,
      expiresAt: review.expiresAt,
      message: 'The exact review expired during final checks. Create and review a fresh preview before execution.',
    });
  }
  const reviewedMutations = normalized(preview.expectedMutations);
  let actualMutations = '';
  let after = {ok: false, reason: 'NOT_READ'};
  const executedAt = finalExecutionTime.toISOString();
  const writeReceipt = (verdict, reason, status, extra = {}) => {
    let deliveryEvidence = 'blocked:no-controller-or-runtime-target-bound';
    try {
      const bindings = JSON.parse(readFileSync('data/live-bindings.json', 'utf8'));
      const candidate = bindings?.runtime?.evidenceSource;
      deliveryEvidence = typeof candidate === 'string' && candidate.trim()
        ? candidate
        : 'blocked:runtime-evidence-invalid';
    } catch {
      // The config revision can still be proven while delivery remains open.
    }
      const receipt = {
        schema: RECEIPT_SCHEMA,
        evidenceClass: 'local-unsigned-execution-receipt',
      verdict,
      reason,
      status,
      reviewId: review.id,
      idempotencyKey: review.idempotencyKey,
      previewId: preview.id,
      actor: review.recordedBy,
      scope: review.scope,
      reviewedMutations,
      actualMutations,
      revisionBefore: before.head,
      revisionAfter: after.ok ? after.head : null,
      targetIdentity: {
        organizationId: before.organizationId,
        spaceId: before.spaceId,
        unitId: before.unitId,
      },
      changeDesc,
      executedAt,
      atomicity: {
        status: 'WATCH',
        reason: 'PROVIDER_ATOMIC_EXPECTED_REVISION_UNAVAILABLE',
        protection: 'pre-read immutable identity and revision match, single-use local intent, exact mutation parity, and post-read revision sequence',
        platformIssue: 'https://github.com/confighubai/confighub/issues/4714',
      },
      delivery: {
        status: 'WATCH',
        reason: deliveryEvidence.startsWith('blocked:')
          ? 'CONTROLLER_OR_RUNTIME_NOT_BOUND'
          : 'DELIVERY_EVIDENCE_UNVERIFIED',
        evidence: deliveryEvidence,
      },
      ...extra,
    };
    receipt.fingerprint = receiptFingerprint(receipt);
    mkdirSync(RECEIPTS_DIR, {recursive: true});
    const receiptPath = join(RECEIPTS_DIR, `${review.id}.json`);
    writeFileSync(receiptPath, `${JSON.stringify(receipt, null, 2)}\n`);
    review.status = verdict === 'PASS' ? 'consumed' : 'execution-unverified';
    review.boundReceipt = receiptPath;
    writeFileSync(path, `${JSON.stringify(review, null, 2)}\n`);
    return {receipt, receiptPath};
  };

  // Claim this single-use intent before invoking Cub. If this process dies,
  // later attempts stop for reconciliation instead of replaying the write.
  const executionClaimPath = join(REVIEWS_DIR, `${review.idempotencyKey}.execution-claim`);
  try {
    writeFileSync(executionClaimPath, `${JSON.stringify({reviewId: review.id, idempotencyKey: review.idempotencyKey, attemptedAt: executedAt})}\n`, {flag: 'wx'});
  } catch {
    return result('BLOCK', 'REVIEW_EXECUTION_ALREADY_CLAIMED', 'COMMIT_BLOCKED', {
      reviewId,
      executionClaimPath,
      message: 'Another execution already claimed this review. Reconcile the ConfigHub Unit before any retry.',
    });
  }
  review.status = 'execution-in-progress';
  review.attemptedAt = executedAt;
  review.executionClaimPath = executionClaimPath;
  review.boundReceipt = null;
  writeFileSync(path, `${JSON.stringify(review, null, 2)}\n`);

  const exec = pinnedCub([
    'function', 'set',
    '--space', space,
    '--unit', unit,
    '--change-desc', changeDesc,
    '-o', 'mutations',
    '--', functionName, ...validated.args,
  ]);
  if (!exec.ok) {
    const reason = exec.commandUnavailable ? 'CUB_COMMAND_UNAVAILABLE' : 'MUTATION_FAILED';
    const recorded = writeReceipt('BLOCK', reason, 'COMMIT_UNVERIFIED', {detail: exec.stderr.slice(0, 500)});
    return result(exec.commandUnavailable ? 'ERROR' : 'BLOCK', reason, exec.commandUnavailable ? 'COMMIT_ERROR' : 'COMMIT_UNVERIFIED', {
      reviewId,
      receipt: recorded.receiptPath,
      detail: exec.stderr.slice(0, 500),
      message: 'The write command was attempted but did not return a verified result. Reconcile the Unit before creating another preview.',
    });
  }

  actualMutations = normalized(exec.stdout);
  after = readUnitAuthority(space, unit, pinnedCub);

  if (!after.ok) {
    const recorded = writeReceipt('BLOCK', 'POST_MUTATION_READ_FAILED', 'COMMIT_UNVERIFIED', {detail: after.detail});
    return result('BLOCK', 'POST_MUTATION_READ_FAILED', 'COMMIT_UNVERIFIED', {
      reviewId: review.id,
      receipt: recorded.receiptPath,
      message: 'The mutation command ran, but the new revision could not be verified. Do not retry this review automatically.',
    });
  }
  if (after.organizationId !== before.organizationId
      || after.spaceId !== before.spaceId
      || after.unitId !== before.unitId) {
    const recorded = writeReceipt('BLOCK', 'TARGET_IDENTITY_CHANGED', 'COMMIT_UNVERIFIED');
    return result('BLOCK', 'TARGET_IDENTITY_CHANGED', 'COMMIT_UNVERIFIED', {
      reviewId: review.id,
      receipt: recorded.receiptPath,
      message: 'The target identity changed during execution. Reconcile the ConfigHub object before any further action.',
    });
  }
  if (after.head !== before.head + 1) {
    const recorded = writeReceipt('BLOCK', 'CONCURRENT_REVISION_DETECTED', 'COMMIT_UNVERIFIED');
    return result('BLOCK', 'CONCURRENT_REVISION_DETECTED', 'COMMIT_UNVERIFIED', {
      reviewId: review.id,
      revisionBefore: before.head,
      revisionAfter: after.head,
      receipt: recorded.receiptPath,
      message: 'The head did not advance by exactly one. A concurrent revision may have entered the sequence; inspect the receipt before any further action.',
    });
  }
  if (actualMutations !== reviewedMutations) {
    const recorded = writeReceipt('BLOCK', 'MUTATION_DIFF_MISMATCH', 'COMMIT_UNVERIFIED');
    return result('BLOCK', 'MUTATION_DIFF_MISMATCH', 'COMMIT_UNVERIFIED', {
      reviewId: review.id,
      revisionBefore: before.head,
      revisionAfter: after.head,
      receipt: recorded.receiptPath,
      message: 'The function ran, but its mutation output differs from the reviewed dry run. Do not treat this as an approved result.',
    });
  }

  const recorded = writeReceipt('PASS', 'CONFIG_REVISION_COMMITTED', 'COMMIT_COMPLETE');
  return result('PASS', 'CONFIG_REVISION_COMMITTED', 'COMMIT_COMPLETE', {
    reviewId: review.id,
    previewId: preview.id,
    revisionBefore: before.head,
    revisionAfter: after.head,
    receipt: recorded.receiptPath,
    atomicity: recorded.receipt.atomicity,
    delivery: recorded.receipt.delivery,
    message: 'ConfigHub created exactly one revision and the actual mutations match the reviewed dry run. Controller and runtime proof remain a separate gate.',
  });
}
