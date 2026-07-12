// Governed action executor: the write path of this app.
//
// commit means one approved, scoped ConfigHub mutation. The executor refuses,
// with a typed reason at exit 0, anything that is not exactly what a human
// approved: unknown approval, consumed approval, function off the whitelist,
// scope mismatch, or a Unit whose head revision moved after approval. After
// executing it verifies the revision actually advanced — a mutation that
// reports success without a new revision is the silent-skip failure class,
// and it is treated as a BLOCK, not a success.
import { spawnSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

export const APPROVAL_SCHEMA = 'confighub.approval.v0';
export const RECEIPT_SCHEMA = 'confighub.receipt.v0';
export const APPROVALS_DIR = 'data/approvals';
export const RECEIPTS_DIR = 'data/receipts';

// The only mutations this scenario is allowed to run. The whitelist is part
// of the app contract, not operator input.
export const ALLOWED_FUNCTIONS = Object.freeze({
  'set-replicas': {argCount: 1, argPattern: /^\d+$/},
  'set-container-resources-defaults': {argCount: 0, argPattern: null},
});

function result(verdict, reason, status, extra = {}) {
  return {verdict, reason, status, ...extra};
}

export function runCub(cubArgs) {
  const proc = spawnSync('cub', cubArgs, {encoding: 'utf8', maxBuffer: 16 * 1024 * 1024});
  return {
    ok: !proc.error && proc.status === 0,
    stdout: proc.stdout || '',
    stderr: String(proc.stderr || proc.error || ''),
  };
}

export function readHeadRevision(space, unit, cub = runCub) {
  const read = cub(['unit', 'get', '--space', space, unit, '-o', 'jq=.Unit.HeadRevisionNum']);
  if (!read.ok) return {ok: false, detail: read.stderr.slice(0, 300)};
  const head = Number(String(read.stdout).trim());
  if (!Number.isInteger(head) || head < 1) return {ok: false, detail: `unparseable head revision: ${read.stdout.slice(0, 80)}`};
  return {ok: true, head};
}

export function grantApproval({space, unit, functionName, args, actor, reason, cub = runCub, now = () => new Date()}) {
  if (!space || !unit) return result('BLOCK', 'SCOPE_REQUIRED', 'APPROVAL_NOT_GRANTED', {message: '--space and --unit are required'});
  if (!actor) return result('BLOCK', 'ACTOR_REQUIRED', 'APPROVAL_NOT_GRANTED', {message: 'an approval records who granted it: pass --actor'});
  const spec = ALLOWED_FUNCTIONS[functionName];
  if (!spec) {
    return result('BLOCK', 'FUNCTION_NOT_ALLOWED', 'APPROVAL_NOT_GRANTED', {
      message: `${functionName} is not in this app's whitelist: ${Object.keys(ALLOWED_FUNCTIONS).join(', ')}`,
    });
  }
  const argList = args || [];
  if (argList.length !== spec.argCount || (spec.argPattern && !argList.every(a => spec.argPattern.test(a)))) {
    return result('BLOCK', 'FUNCTION_ARGS_INVALID', 'APPROVAL_NOT_GRANTED', {
      message: `${functionName} expects ${spec.argCount} argument(s)${spec.argPattern ? ` matching ${spec.argPattern}` : ''}`,
    });
  }
  const head = readHeadRevision(space, unit, cub);
  if (!head.ok) return result('BLOCK', 'UNIT_READ_FAILED', 'APPROVAL_NOT_GRANTED', {detail: head.detail});

  const grantedAt = now().toISOString();
  const id = `apr-${grantedAt.replace(/[-:TZ.]/g, '').slice(0, 14)}-${unit.slice(0, 24)}`;
  const approval = {
    schema: APPROVAL_SCHEMA,
    id,
    status: 'granted',
    actor,
    reason: reason || '',
    grantedAt,
    scope: {
      space,
      unit,
      function: functionName,
      args: argList,
      headRevisionAtApproval: head.head,
    },
    singleUse: true,
    consumedByReceipt: null,
  };
  mkdirSync(APPROVALS_DIR, {recursive: true});
  const path = join(APPROVALS_DIR, `${id}.json`);
  writeFileSync(path, `${JSON.stringify(approval, null, 2)}\n`);
  return result('PASS', 'APPROVAL_GRANTED', 'APPROVAL_GRANTED', {approval, path});
}

export function executeApproved({approvalId, cub = runCub, now = () => new Date()}) {
  if (!approvalId) return result('BLOCK', 'APPROVAL_REQUIRED', 'COMMIT_BLOCKED', {message: 'pass --approval <id>'});
  const path = join(APPROVALS_DIR, `${approvalId}.json`);
  let approval;
  try {
    approval = JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    return result('BLOCK', 'APPROVAL_NOT_FOUND', 'COMMIT_BLOCKED', {approvalId, path});
  }
  if (approval.schema !== APPROVAL_SCHEMA || !approval.scope) {
    return result('BLOCK', 'APPROVAL_INVALID', 'COMMIT_BLOCKED', {approvalId});
  }
  if (approval.status !== 'granted') {
    return result('BLOCK', 'APPROVAL_ALREADY_CONSUMED', 'COMMIT_BLOCKED', {
      approvalId,
      consumedByReceipt: approval.consumedByReceipt,
      message: 'approvals are single-use: grant a new one for a new change',
    });
  }
  const {space, unit, function: functionName, args, headRevisionAtApproval} = approval.scope;
  const spec = ALLOWED_FUNCTIONS[functionName];
  if (!spec) return result('BLOCK', 'FUNCTION_NOT_ALLOWED', 'COMMIT_BLOCKED', {approvalId, functionName});

  const before = readHeadRevision(space, unit, cub);
  if (!before.ok) return result('BLOCK', 'UNIT_READ_FAILED', 'COMMIT_BLOCKED', {detail: before.detail});
  if (before.head !== headRevisionAtApproval) {
    return result('BLOCK', 'APPROVAL_REVISION_DRIFT', 'COMMIT_BLOCKED', {
      approvalId,
      approvedAtRevision: headRevisionAtApproval,
      currentRevision: before.head,
      message: 'the Unit changed after approval was granted; review and re-approve against the current revision',
    });
  }

  const changeDesc = [
    `${functionName} ${args.join(' ')} on ${space}/${unit} via approved cost recommendation`,
    '',
    `User prompt: approval ${approval.id} granted by ${approval.actor}${approval.reason ? ` — ${approval.reason}` : ''}`,
    `Clarifications: approved against revision ${headRevisionAtApproval}; single-use; executor whitelist ${Object.keys(ALLOWED_FUNCTIONS).join(', ')}`,
  ].join('\n');

  const exec = cub([
    'function', 'set',
    '--space', space,
    '--unit', unit,
    '--change-desc', changeDesc,
    '-o', 'mutations',
    '--',
    functionName, ...args,
  ]);
  if (!exec.ok) {
    return result('BLOCK', 'MUTATION_FAILED', 'COMMIT_BLOCKED', {approvalId, detail: exec.stderr.slice(0, 500)});
  }

  const after = readHeadRevision(space, unit, cub);
  if (!after.ok) {
    return result('BLOCK', 'POST_MUTATION_READ_FAILED', 'COMMIT_UNVERIFIED', {
      approvalId,
      detail: after.detail,
      message: 'the mutation ran but the revision could not be verified; do not treat this as success',
    });
  }
  if (after.head <= before.head) {
    // The silent-skip class: a mutation that claims success without a new
    // revision delivered nothing. Typed BLOCK, never a success.
    return result('BLOCK', 'MUTATION_SILENT_SKIP', 'COMMIT_BLOCKED', {
      approvalId,
      revisionBefore: before.head,
      revisionAfter: after.head,
      message: 'the function reported success but no new revision exists; nothing changed',
    });
  }

  let deliveryEvidence = 'blocked:no-controller-or-runtime-target-bound';
  try {
    const bindings = JSON.parse(readFileSync('data/live-bindings.json', 'utf8'));
    deliveryEvidence = bindings?.runtime?.evidenceSource || deliveryEvidence;
  } catch {
    // keep the honest default
  }

  const executedAt = now().toISOString();
  const receipt = {
    schema: RECEIPT_SCHEMA,
    approvalId: approval.id,
    actor: approval.actor,
    scope: approval.scope,
    revisionBefore: before.head,
    revisionAfter: after.head,
    changeDesc,
    mutations: exec.stdout.slice(0, 4000),
    executedAt,
    deliveryEvidence,
    nextGate: deliveryEvidence.startsWith('blocked:')
      ? 'The ConfigHub revision is real; delivery to a runtime still needs an apply and its own verification.'
      : 'Verify delivery through the bound runtime evidence source.',
  };
  mkdirSync(RECEIPTS_DIR, {recursive: true});
  const receiptPath = join(RECEIPTS_DIR, `${approval.id}.json`);
  writeFileSync(receiptPath, `${JSON.stringify(receipt, null, 2)}\n`);

  approval.status = 'consumed';
  approval.consumedByReceipt = receiptPath;
  writeFileSync(path, `${JSON.stringify(approval, null, 2)}\n`);

  return result('PASS', 'MUTATION_COMMITTED', 'COMMIT_COMPLETE', {
    approvalId: approval.id,
    revisionBefore: before.head,
    revisionAfter: after.head,
    receipt: receiptPath,
    deliveryEvidence,
  });
}
