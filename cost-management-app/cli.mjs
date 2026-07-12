#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { grantApproval, executeApproved, ALLOWED_FUNCTIONS } from './src/executor.mjs';

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

function hasPlaceholder(value) {
  if (typeof value === 'string') return value.includes('<') || value.includes('>') || value.includes('example-fill');
  if (Array.isArray(value)) return value.some(hasPlaceholder);
  if (value && typeof value === 'object') return Object.values(value).some(hasPlaceholder);
  return false;
}

function isBlockedValue(value) {
  return typeof value === 'string' && value.startsWith('blocked:');
}

function bindingStatus() {
  if (!liveBindings) {
    return {
      status: 'LIVE_BINDINGS_MISSING',
      readyForCommit: false,
      reason: 'data/live-bindings.json is not present.',
    };
  }
  if (hasPlaceholder(liveBindings)) {
    return {
      status: 'LIVE_BINDINGS_PLACEHOLDER',
      readyForCommit: false,
      reason: 'data/live-bindings.json still contains placeholder values.',
    };
  }
  const required = [
    ['configHub.objectUrl', liveBindings.configHub?.objectUrl],
    ['approval.objectId', liveBindings.approval?.objectId],
    ['action.endpoint', liveBindings.action?.endpoint],
    ['action.contract.kind', liveBindings.action?.contract?.kind],
    ['proof.receiptObjectId', liveBindings.proof?.receiptObjectId],
    ['runtime.evidenceSource', liveBindings.runtime?.evidenceSource],
  ];
  const missing = required.filter(([, value]) => !value).map(([field]) => field);
  if (missing.length) {
    return {
      status: 'LIVE_BINDINGS_INCOMPLETE',
      readyForCommit: false,
      reason: `Missing live binding fields: ${missing.join(', ')}`,
    };
  }
  const blocked = required.filter(([, value]) => isBlockedValue(value)).map(([field, value]) => `${field}=${value}`);
  if (blocked.length) {
    return {
      status: 'LIVE_BINDINGS_BLOCKED',
      readyForCommit: false,
      reason: `Live read surface exists, but commit remains blocked: ${blocked.join(', ')}`,
    };
  }
  if (liveBindings.action.contract.kind !== 'ConfigHub-governed-action.v0') {
    return {
      status: 'LIVE_BINDINGS_INCOMPLETE',
      readyForCommit: false,
      reason: 'action.contract.kind must be ConfigHub-governed-action.v0',
    };
  }
  return {
    status: 'LIVE_BINDINGS_READY',
    readyForCommit: true,
    reason: 'Live ConfigHub object, approval, action, receipt, and runtime bindings are present.',
  };
}

function selectedVariant() {
  if (variantArg) {
    return workflow.variants.find(variant => variant.id === variantArg || variant.variant === variantArg || variant.unit === variantArg);
  }
  return workflow.variants[0];
}

function output(value, exitCode = 0) {
  if (json) {
    console.log(JSON.stringify(value, null, 2));
  } else {
    printHuman(value);
  }
  return exitCode;
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
    verdict: bindings.readyForCommit ? 'PASS' : 'WATCH',
    reason: bindings.status,
    status: bindings.readyForCommit ? 'PASS' : 'WATCH',
    app: workflow.app.name,
    commandLoop: workflow.operationalLoop,
    sharedContract: 'data/operational-workflow.json',
    liveBindings: bindings.status,
    authMode: workflow.authMode,
    checks: [
      {id: 'workflow-contract', status: 'PASS'},
      {id: 'variant-scope', status: workflow.variants.length ? 'PASS' : 'BLOCK'},
      {id: 'live-bindings', status: bindings.readyForCommit ? 'PASS' : 'WATCH', reason: bindings.reason},
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
        severity: finding.severity,
        code: finding.rule,
        where: finding.space
          ? `${finding.space}${finding.unit ? `/${finding.unit}` : ''}${finding.workload ? ` (${finding.workload})` : ''}`
          : finding.workload,
        message: finding.recommendation?.summary || finding.rule,
        monthly: finding.priced ? `${finding.priced.monthly} ${finding.priced.currency} (${finding.priced.claim})` : null,
        nextAction: finding.recommendation?.preview || 'Review the finding in data/cost-findings.json.',
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
  if (!bindings.readyForCommit) {
    rows.push({
      severity: 'high',
      code: bindings.status,
      message: bindings.reason,
      nextAction: 'Bind live ConfigHub object, approval, action, proof, and runtime evidence before commit.',
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
  const reason = !bindings.readyForCommit
    ? bindings.status
    : (blocked ? 'COST_FINDINGS_NEED_ACTION' : 'NO_BLOCKING_FINDINGS');
  return {
    verdict: blocked ? 'WATCH' : 'PASS',
    reason,
    status: blocked ? 'WATCH' : 'PASS',
    app: workflow.app.name,
    costTotals: cost ? cost.totals : null,
    costGeneratedAt: cost ? cost.generatedAt : null,
    findings: rows,
    nextGate: cost
      ? 'Preview a governed dry-run diff for a finding, or bind missing live proof first.'
      : 'Run the cost sweep, then preview a governed dry-run diff for a finding.',
  };
}

function preview() {
  const variant = selectedVariant();
  if (!variant) {
    return {
      status: 'BLOCK',
      error: 'VARIANT_REQUIRED',
      message: 'No Variant is available to preview.',
    };
  }
  return {
    status: 'PREVIEW_READY',
    app: workflow.app.name,
    variant,
    approvalScope: workflow.approval.scopeFields,
    mutation: 'none',
    message: 'Preview only. No ConfigHub, Git, controller, or runtime mutation has been performed.',
    nextGate: 'Record approval against this exact scope before commit.',
  };
}

function approve() {
  if (args.includes('--grant')) {
    const argsFlag = valueAfter('--args');
    const granted = grantApproval({
      space: valueAfter('--space'),
      unit: valueAfter('--unit'),
      functionName: valueAfter('--function'),
      args: argsFlag === '' ? [] : argsFlag.split(','),
      actor: valueAfter('--actor'),
      reason: valueAfter('--reason'),
    });
    granted.app = workflow.app.name;
    granted.nextGate = granted.verdict === 'PASS'
      ? `Run commit --approval ${granted.approval.id} to execute exactly this scope.`
      : 'Fix the refusal and grant again.';
    return granted;
  }
  const variant = selectedVariant();
  return {
    status: 'APPROVAL_SCOPE_PREVIEW',
    app: workflow.app.name,
    variant,
    scopeFields: workflow.approval.scopeFields,
    allowedFunctions: Object.keys(ALLOWED_FUNCTIONS),
    mutation: 'none',
    message: 'Preview only. Grant a real single-use approval with: approve --grant --space <s> --unit <u> --function <fn> --args <a[,b]> --actor <who> --reason <why>.',
    nextGate: 'Grant an approval, then run commit --approval <id>.',
  };
}

function commit() {
  const bindings = bindingStatus();
  const variant = selectedVariant();
  const approvalId = valueAfter('--approval');
  if (approvalId) {
    // The write path: one approved, scoped, revision-verified mutation.
    // It needs the live read surface and approval/action bindings; runtime
    // evidence gates the delivery claim on the receipt, not the mutation.
    if (bindings.status === 'LIVE_BINDINGS_MISSING' || bindings.status === 'LIVE_BINDINGS_PLACEHOLDER' || bindings.status === 'LIVE_BINDINGS_INCOMPLETE') {
      return output({
        verdict: 'BLOCK',
        reason: bindings.status,
        status: 'COMMIT_BLOCKED',
        app: workflow.app.name,
        detail: bindings.reason,
        nextGate: 'Create data/live-bindings.json from verified reads before executing approvals.',
      }, 0);
    }
    const executed = executeApproved({approvalId});
    executed.app = workflow.app.name;
    if (!executed.nextGate) {
      executed.nextGate = executed.verdict === 'PASS'
        ? 'Read the receipt, then verify and receipt close the loop; delivery needs its own apply step.'
        : 'Resolve the typed refusal, re-grant if needed, and run commit again.';
    }
    return output(executed, 0);
  }
  if (!bindings.readyForCommit) {
    // Expected operational blocker, not a shell failure: typed BLOCK at exit 0.
    return output({
      verdict: 'BLOCK',
      reason: 'APPROVED_CONFIGHUB_MUTATION_REQUIRED',
      status: 'COMMIT_BLOCKED',
      error: 'APPROVED_CONFIGHUB_MUTATION_REQUIRED',
      app: workflow.app.name,
      variant,
      liveBindings: bindings.status,
      detail: bindings.reason,
      message: 'commit means an approved scoped ConfigHub mutation, not a local CLI flag being accepted.',
      nextGate: 'Create real live bindings and rerun findings, preview, approval, verify, and receipt.',
    }, 0);
  }
  return output({
    verdict: 'BLOCK',
    reason: 'APPROVAL_REQUIRED',
    status: 'COMMIT_BLOCKED',
    error: 'APPROVAL_REQUIRED',
    app: workflow.app.name,
    variant,
    detail: 'The executor runs only a granted single-use approval. Grant one with approve --grant, then commit --approval <id>.',
    nextGate: 'approve --grant --space <s> --unit <u> --function <fn> --args <a> --actor <who>, then commit --approval <id>.',
  }, 0);
}

function verify() {
  const bindings = bindingStatus();
  return {
    verdict: 'WATCH',
    reason: bindings.readyForCommit ? 'RUNTIME_PROOF_PENDING' : bindings.status,
    status: 'WATCH',
    app: workflow.app.name,
    liveBindings: bindings.status,
    proofTabs: workflow.proofTabs,
    message: bindings.readyForCommit
      ? 'Bindings are present. Runtime and receipt proof still need a scenario executor run.'
      : bindings.reason,
    nextGate: 'Run receipt after proof evidence is connected.',
  };
}

function receipt() {
  const bindings = bindingStatus();
  return {
    verdict: 'WATCH',
    reason: bindings.readyForCommit ? 'SCENARIO_EXECUTOR_NOT_RUN' : bindings.status,
    status: 'WAITING_FOR_LIVE_PROOF',
    app: workflow.app.name,
    scenario: workflow.scenario,
    strategy: workflow.strategy,
    liveBindings: bindings.status,
    proofTabs: workflow.proofTabs,
    omissions: bindings.readyForCommit
      ? ['No scenario-specific action has been executed by this generated starter.']
      : [bindings.reason],
    nextGate: bindings.readyForCommit ? 'Wire and run the scenario-specific executor.' : 'Resolve live binding findings.',
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
    commands: ['preflight', 'map', 'list', 'snapshot', 'findings', 'preview', 'approve', 'commit', 'verify', 'receipt', 'guardrails'],
    loop: 'preflight -> map/list -> findings -> preview -> approve/commit -> verify -> receipt',
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
      return output(preview(), selectedVariant() ? 0 : 2);
    case 'approve':
      return output(approve());
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
