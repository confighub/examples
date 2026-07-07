#!/usr/bin/env node
import { readFile } from 'node:fs/promises';

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
  if (value.nextGate) console.log(`Next: ${value.nextGate}`);
}

function preflight() {
  const bindings = bindingStatus();
  return {
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
    status: 'PASS',
    app: workflow.app.name,
    scopeModel: workflow.scopeModel,
    variants: workflow.variants,
    nextGate: 'Run findings to see blockers and proof gaps.',
  };
}

function findings() {
  const bindings = bindingStatus();
  const rows = [];
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
  return {
    status: rows.some(row => row.severity === 'high') ? 'WATCH' : 'PASS',
    app: workflow.app.name,
    findings: rows,
    nextGate: 'Preview a scoped Variant change, or bind missing live proof first.',
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
  const variant = selectedVariant();
  return {
    status: 'APPROVAL_SCOPE_PREVIEW',
    app: workflow.app.name,
    variant,
    scopeFields: workflow.approval.scopeFields,
    mutation: 'none',
    message: 'This generated CLI describes approval scope. It does not create a live approval object until live bindings connect one.',
    nextGate: 'Bind a real approval object before commit.',
  };
}

function commit() {
  const bindings = bindingStatus();
  const variant = selectedVariant();
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
    reason: 'LIVE_ACTION_EXECUTOR_REQUIRED',
    status: 'COMMIT_BLOCKED',
    error: 'LIVE_ACTION_EXECUTOR_REQUIRED',
    app: workflow.app.name,
    variant,
    detail: 'Live bindings are present, but this generated starter has no scenario-specific action executor.',
    nextGate: 'Wire the governed ConfigHub action executor before enabling commit.',
  }, 0);
}

function verify() {
  const bindings = bindingStatus();
  return {
    status: bindings.readyForCommit ? 'WATCH' : 'WATCH',
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
      return output(findings());
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
