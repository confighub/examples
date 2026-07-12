#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { createPreview, recordReview, commitReviewed, reviewPacketStatus, ALLOWED_FUNCTIONS } from './src/executor.mjs';
import { classifyLiveBindings } from './src/live-bindings.mjs';

const args = process.argv.slice(2);
const command = normalizeCommand(args.find(arg => !arg.startsWith('--')) || 'help');
const json = args.includes('--json');
const variantArg = valueAfter('--variant');

function valueAfter(flag) {
  const index = args.indexOf(flag);
  if (index === -1) return '';
  return args[index + 1] || '';
}

function normalizeCommand(value) {
  if (['snapshot', 'list'].includes(value)) return 'map';
  if (value === 'apply') return 'commit';
  if (value === 'guardrail') return 'guardrails';
  return value;
}

async function loadJsonFile(path, fallback = null) {
  try {
    return JSON.parse(await readFile(path, 'utf8'));
  } catch {
    return fallback;
  }
}

const workflow = await loadJsonFile('data/operational-workflow.json');
if (!workflow) {
  console.error('data/operational-workflow.json is required');
  process.exit(1);
}
const liveBindings = await loadJsonFile('data/live-bindings.json');

function bindingStatus() {
  return classifyLiveBindings(liveBindings);
}

function selectedVariant() {
  if (variantArg) {
    return workflow.variants.find(variant => variant.id === variantArg || variant.variant === variantArg || variant.unit === variantArg);
  }
  return workflow.variants[0];
}

function output(value, exitCode = null) {
  if (json) {
    console.log(JSON.stringify(value, null, 2));
  } else {
    printHuman(value);
  }
  return exitCode === null ? (value?.verdict === 'ERROR' ? 1 : 0) : exitCode;
}

function printHuman(value) {
  console.log(`${value.status || 'OK'}: ${value.app || workflow.app.name}`);
  if (value.message) console.log(value.message);
  if (value.reason) console.log(value.reason);
  if (value.costTotals) {
    const t = value.costTotals;
    console.log(`Scanned ${t.containersScanned} containers in ${t.spacesWithWorkloads} spaces; ${t.containersMissingRequests} missing requests.`);
    console.log(`Configured request cost ${t.configuredMonthlyRequestCost}/mo; claimed savings ${t.claimedMonthlySavings}/mo (bound units only).`);
  }
  for (const row of value.findings || []) {
    const where = row.where ? ` ${row.where}` : '';
    const monthly = row.monthly ? ` [${row.monthly}]` : '';
    console.log(`- ${row.severity} ${row.code}${where}${monthly}: ${row.message}`);
  }
  if (value.nextGate) console.log(`Next: ${value.nextGate}`);
}

function preflight() {
  const bindings = bindingStatus();
  return {
    verdict: bindings.readyForLive ? 'PASS' : 'WATCH',
    reason: bindings.status,
    status: bindings.readyForLive ? 'PASS' : 'WATCH',
    app: workflow.app.name,
    commandLoop: workflow.operationalLoop,
    sharedContract: 'data/operational-workflow.json',
    liveBindings: bindings.status,
    authMode: workflow.authMode,
    checks: [
      {id: 'workflow-contract', status: 'PASS'},
      {id: 'variant-scope', status: workflow.variants.length ? 'PASS' : 'BLOCK'},
      {id: 'action-authority', status: bindings.readyForReview ? 'PASS' : 'WATCH', reason: bindings.reason},
      {id: 'delivery-proof', status: bindings.readyForLive ? 'PASS' : 'WATCH', reason: bindings.reason},
    ],
    nextGate: 'Run map/list, then findings, before previewing any change.',
  };
}

function mapVariants() {
  return {
    verdict: 'PASS',
    reason: 'VARIANT_SCOPE_MAPPED',
    status: 'PASS',
    app: workflow.app.name,
    scopeModel: workflow.scopeModel,
    variants: workflow.variants,
    nextGate: 'Run findings to see blockers and proof gaps.',
  };
}

async function findings() {
  const bindings = bindingStatus();
  const rows = [];
  const cost = await loadJsonFile('data/cost-findings.json');
  if (cost && Array.isArray(cost.findings)) {
    for (const finding of cost.findings) {
      rows.push({
        id: finding.id,
        severity: finding.severity,
        code: finding.rule,
        where: finding.space
          ? `${finding.space}${finding.unit ? `/${finding.unit}` : ''}${finding.workload ? ` (${finding.workload})` : ''}`
          : finding.workload,
        message: finding.recommendation?.summary || finding.rule,
        monthly: finding.priced ? `${finding.priced.monthly} ${finding.priced.currency} (${finding.priced.claim})` : null,
        nextAction: finding.recommendation?.preview || 'Review the finding in data/cost-findings.json.',
        actionable: Boolean(finding.recommendation?.action),
      });
    }
  } else {
    rows.push({
      severity: 'medium',
      code: 'COST_SWEEP_NOT_RUN',
      message: 'No cost findings exist yet for this deployment.',
      nextAction: 'Run npm run cost:sweep with a ConfigHub session that can read the org.',
    });
  }
  if (!bindings.readyForReview) {
    rows.push({
      severity: 'high',
      code: bindings.status,
      message: bindings.reason,
      nextAction: 'Bind the ConfigHub object, org identity, and action authority before preview.',
    });
  } else if (!bindings.readyForLive) {
    rows.push({
      severity: 'medium',
      code: bindings.status,
      message: bindings.reason,
      nextAction: 'Review the exact dry-run diff. After explicit confirmation, the CLI can execute and verify one ConfigHub revision; controller delivery remains a separate gate.',
    });
  }
  for (const rule of workflow.stopRules || []) {
    rows.push({
      severity: 'medium',
      code: 'STOP_RULE',
      message: rule,
      nextAction: 'Keep this rule visible in GUI and CLI.',
    });
  }
  const blocked = rows.some(row => row.severity === 'high');
  const watch = blocked || !bindings.readyForLive;
  const reason = !bindings.readyForReview
    ? bindings.status
    : (!bindings.readyForLive ? bindings.status : (blocked ? 'COST_FINDINGS_NEED_ACTION' : 'NO_BLOCKING_FINDINGS'));
  return {
    verdict: watch ? 'WATCH' : 'PASS',
    reason,
    status: watch ? 'WATCH' : 'PASS',
    app: workflow.app.name,
    costTotals: cost ? cost.totals : null,
    costGeneratedAt: cost ? cost.generatedAt : null,
    findings: rows,
    nextGate: cost
      ? 'Preview a governed dry-run diff for a finding, or bind missing live proof first.'
      : (workflow.domainEngine?.kind === 'cost'
        ? 'Run the read-only cost sweep, then inspect its findings.'
        : 'Use preview --variant <variant-id> to inspect scope. This app will not invent a mutation until its domain engine produces an actionable finding.'),
  };
}

async function preview() {
  const findingId = valueAfter('--finding');
  if (!findingId) {
    const variant = selectedVariant();
    if (variantArg && variant) {
      return {
        verdict: 'WATCH',
        reason: 'VARIANT_SCOPE_PREVIEW_ONLY',
        status: 'SCOPE_PREVIEW_READY',
        app: workflow.app.name,
        variant,
        mutation: 'none',
        message: 'This is a read-only Variant scope preview. Choose an actionable finding for an exact function dry-run.',
        nextGate: 'Run findings --json. If a finding has no governed action, keep it as guidance rather than inventing a mutation.',
      };
    }
    return {
      verdict: 'BLOCK',
      reason: 'FINDING_REQUIRED',
      status: 'PREVIEW_BLOCKED',
      message: 'Choose one actionable finding from `findings --json`, then pass --finding <id>.',
    };
  }
  const bindings = bindingStatus();
  if (!bindings.readyForReview) {
    return {
      verdict: 'BLOCK',
      reason: bindings.status,
      status: 'PREVIEW_BLOCKED',
      detail: bindings.reason,
      nextGate: 'Bind and verify the ConfigHub action authority before previewing a mutation.',
    };
  }
  const cost = await loadJsonFile('data/cost-findings.json');
  const finding = cost?.findings?.find(row => row.id === findingId);
  if (!finding) {
    return {
      verdict: 'BLOCK',
      reason: 'FINDING_NOT_FOUND',
      status: 'PREVIEW_BLOCKED',
      findingId,
      message: 'The finding is not present in the current findings file. Rerun findings and choose an exact id.',
    };
  }
  const created = createPreview({finding, boundAuthority: liveBindings.configHub});
  created.app = workflow.app.name;
  created.nextGate = created.verdict === 'PASS'
    ? `Inspect ${created.path}, then record that exact local review with review --record --preview ${created.preview.id}.`
    : 'Resolve the typed refusal, refresh findings, and preview again.';
  return created;
}

function review() {
  if (args.includes('--record')) {
    const recorded = recordReview({
      previewId: valueAfter('--preview'),
      reason: valueAfter('--reason'),
    });
    recorded.app = workflow.app.name;
    recorded.nextGate = recorded.reason === 'LOCAL_REVIEW_RECORDED'
      ? `Ask for explicit approval of this exact scope, then run commit --review ${recorded.review.id} --confirm-execute before the review expires.`
      : 'Fix the refusal and record the review again.';
    return recorded;
  }
  return {
    verdict: 'WATCH',
    reason: 'LOCAL_REVIEW_NOT_RECORDED',
    status: 'LOCAL_REVIEW_SCOPE',
    app: workflow.app.name,
    scopeFields: workflow.approval.scopeFields,
    allowedFunctions: Object.keys(ALLOWED_FUNCTIONS),
    mutation: 'none',
    message: 'A local review record is evidence, not ConfigHub approval or mutation permission. Create and inspect an exact preview first.',
    nextGate: 'Run review --record --preview <id>, inspect the expiring review, then explicitly confirm commit for that review id.',
  };
}

function commit() {
  const bindings = bindingStatus();
  const variant = selectedVariant();
  const reviewId = valueAfter('--review');
  if (reviewId) {
            // The executor validates the stored preview and local review, then
            // requires explicit confirmation before one governed ConfigHub write.
    if (!bindings.readyForReview) {
      return output({
        verdict: 'BLOCK',
        reason: bindings.status,
        status: 'COMMIT_BLOCKED',
        app: workflow.app.name,
        detail: bindings.reason,
        nextGate: 'Create data/live-bindings.json from verified reads before recording a local review.',
      }, 0);
    }
    const executed = commitReviewed({reviewId, confirmed: args.includes('--confirm-execute')});
    executed.app = workflow.app.name;
    if (!executed.nextGate) {
      executed.nextGate = executed.verdict === 'PASS'
        ? 'Read the receipt, then prove controller delivery and runtime state.'
        : (executed.verdict === 'ASK'
          ? `Inspect the review packet, then rerun commit --review ${reviewId} --confirm-execute.`
          : 'Resolve the typed refusal or unverified execution receipt before another action.');
    }
    return output(executed);
  }
  if (!bindings.readyForReview) {
    // Expected operational blocker, not a shell failure: typed BLOCK at exit 0.
    return output({
      verdict: 'BLOCK',
      reason: 'LIVE_BINDINGS_REQUIRED',
      status: 'COMMIT_BLOCKED',
      error: 'LIVE_BINDINGS_REQUIRED',
      app: workflow.app.name,
      variant,
      liveBindings: bindings.status,
      detail: bindings.reason,
      message: 'commit means checking one exact preview plus its local review record, not accepting a local mutation flag.',
      nextGate: 'Create real live bindings and rerun findings, preview, review, verify, and receipt.',
    }, 0);
  }
  return output({
    verdict: 'BLOCK',
    reason: 'LOCAL_REVIEW_REQUIRED',
    status: 'COMMIT_BLOCKED',
    error: 'LOCAL_REVIEW_REQUIRED',
    app: workflow.app.name,
    variant,
    detail: 'The executor checks only a local review record tied to a stored dry-run preview. That record is not ConfigHub approval.',
    nextGate: 'preview --finding <id>, inspect the saved diff, review --record --preview <preview-id>, then commit --review <id>.',
  }, 0);
}

function verify() {
  const bindings = bindingStatus();
  const reviewPacket = reviewPacketStatus();
  const configRevisionRecorded = reviewPacket.reason === 'CONFIG_REVISION_COMMITTED'
    || reviewPacket.recordedOutcome?.reason === 'CONFIG_REVISION_COMMITTED';
  const reason = !bindings.readyForReview
    ? bindings.status
    : (reviewPacket.reason === 'LOCAL_REVIEW_RECORDED'
      ? 'EXECUTION_CONFIRMATION_REQUIRED'
      : (configRevisionRecorded ? 'CONFIG_REVISION_COMMITTED_DELIVERY_PENDING' : reviewPacket.reason));
  return {
    verdict: 'WATCH',
    reason,
    status: 'WATCH',
    app: workflow.app.name,
    liveBindings: bindings.status,
    reviewPacket,
    proofTabs: workflow.proofTabs,
    message: !bindings.readyForReview ? bindings.reason : reviewPacket.message,
    nextGate: configRevisionRecorded
      ? 'Verify controller delivery and the exact runtime field before calling the operation live.'
      : (bindings.readyForReview && reviewPacket.reason === 'LOCAL_REVIEW_RECORDED'
        ? `Run commit --review ${reviewPacket.reviewId} --confirm-execute after explicit approval.`
        : (!bindings.readyForReview ? 'Resolve the live authority gaps, then create an exact preview.' : 'Create and inspect an exact preview, then record the local review.')),
  };
}

function receipt() {
  const bindings = bindingStatus();
  const reviewPacket = reviewPacketStatus();
  const configRevisionRecorded = reviewPacket.reason === 'CONFIG_REVISION_COMMITTED'
    || reviewPacket.recordedOutcome?.reason === 'CONFIG_REVISION_COMMITTED';
  const reason = !bindings.readyForReview
    ? bindings.status
    : (reviewPacket.reason === 'LOCAL_REVIEW_RECORDED'
      ? 'EXECUTION_CONFIRMATION_REQUIRED'
      : (configRevisionRecorded ? 'CONFIG_REVISION_COMMITTED_DELIVERY_PENDING' : reviewPacket.reason));
  return {
    verdict: 'WATCH',
    reason,
    status: 'WAITING_FOR_LIVE_PROOF',
    app: workflow.app.name,
    scenario: workflow.scenario,
    strategy: workflow.strategy,
    liveBindings: bindings.status,
    reviewPacket,
    proofTabs: workflow.proofTabs,
    omissions: [bindings.reason, reviewPacket.message].filter(Boolean),
    nextGate: configRevisionRecorded
      ? 'Attach controller and runtime evidence to close the delivery receipt.'
      : (bindings.readyForReview && reviewPacket.reason === 'LOCAL_REVIEW_RECORDED'
        ? `Run commit --review ${reviewPacket.reviewId} --confirm-execute after explicit approval.`
        : (!bindings.readyForReview ? 'Resolve live binding findings.' : 'Record a valid local review packet after binding exact-review authority.')),
  };
}

function guardrails() {
  return {
    status: 'INFO',
    app: workflow.app.name,
    message: 'Guardrails are common but not universal. This app closes through the proof tabs and stop rules in the workflow contract.',
    proofAlternatives: workflow.proofTabs,
    nextGate: 'Use findings, verify, and receipt for this generated package.',
  };
}

function help() {
  return {
    status: 'OK',
    app: workflow.app.name,
    commands: ['preflight', 'map', 'list', 'snapshot', 'findings', 'preview --variant <variant-id>', 'preview --finding <id>', 'review --record --preview <id>', 'commit --review <id> --confirm-execute', 'verify', 'receipt', 'guardrails'],
    loop: 'preflight -> map/list -> findings -> preview -> review/commit -> verify -> receipt',
  };
}

async function main() {
  switch (command) {
    case 'preflight':
      return output(preflight());
    case 'map':
      return output(mapVariants());
    case 'findings':
      return output(await findings());
    case 'preview':
      return output(await preview());
    case 'review':
      return output(review());
    case 'commit':
      return commit();
    case 'verify':
      return output(verify());
    case 'receipt':
      return output(receipt());
    case 'guardrails':
      return output(guardrails());
    case 'help':
    default:
      return output(help());
  }
}

process.exitCode = await main();
