import { readFile } from 'node:fs/promises';

// Typed live-binding classification. Expected not-ready states are successful
// classifications printed to stdout at exit 0 ({verdict, reason}). Exit is
// non-zero only when the file exists but cannot be read or parsed.
function emit(result, exitCode = 0) {
  const line = JSON.stringify(result, null, 2);
  if (exitCode === 0) {
    console.log(line);
  } else {
    console.error(line);
  }
  process.exit(exitCode);
}

let raw;
try {
  raw = await readFile('data/live-bindings.json', 'utf8');
} catch (error) {
  if (error && error.code === 'ENOENT') {
    emit({
      verdict: 'WATCH',
      reason: 'LIVE_BINDINGS_MISSING',
      status: 'LIVE_BINDINGS_MISSING',
      requiredFile: 'data/live-bindings.json',
      exampleFile: 'data/live-bindings.example.json',
      message: 'No deployment-local live bindings yet. This is the correct safe default on a fresh clone.',
    }, 0);
  }
  emit({verdict: 'ERROR', reason: 'LIVE_BINDINGS_UNREADABLE', status: 'LIVE_BINDINGS_UNREADABLE', detail: String((error && error.message) || error)}, 1);
}

let bindings;
try {
  bindings = JSON.parse(raw);
} catch (error) {
  emit({verdict: 'ERROR', reason: 'LIVE_BINDINGS_UNPARSEABLE', status: 'LIVE_BINDINGS_UNPARSEABLE', detail: String((error && error.message) || error)}, 1);
}

const required = [
  ['configHub', 'objectUrl'],
  ['approval', 'objectId'],
  ['action', 'endpoint'],
  ['proof', 'receiptObjectId'],
  ['runtime', 'evidenceSource'],
];

function hasPlaceholder(value) {
  if (typeof value === 'string') {
    return value.includes('<') || value.includes('>') || value.includes('example-fill');
  }
  if (Array.isArray(value)) {
    return value.some(hasPlaceholder);
  }
  if (value && typeof value === 'object') {
    return Object.values(value).some(hasPlaceholder);
  }
  return false;
}

function isBlockedValue(value) {
  return typeof value === 'string' && value.startsWith('blocked:');
}

const missing = [];
for (const [section, field] of required) {
  if (!bindings[section] || !bindings[section][field]) missing.push(`${section}.${field}`);
}
const contract = bindings.action && bindings.action.contract;
if (!contract) missing.push('action.contract');
if (missing.length) {
  emit({verdict: 'WATCH', reason: 'LIVE_BINDINGS_INCOMPLETE', status: 'LIVE_BINDINGS_INCOMPLETE', missing}, 0);
}

const blocked = required
  .filter(([section, field]) => isBlockedValue(bindings[section] && bindings[section][field]))
  .map(([section, field]) => `${section}.${field}=${bindings[section][field]}`);
if (blocked.length) {
  emit({
    verdict: 'WATCH',
    reason: 'LIVE_BINDINGS_BLOCKED',
    status: 'LIVE_BINDINGS_BLOCKED',
    blocked,
    message: 'Live read surfaces exist, but at least one required binding is explicitly blocked.',
  }, 0);
}

const contractProblems = [];
if (contract.kind !== 'ConfigHub-governed-action.v0') contractProblems.push('action.contract.kind must be ConfigHub-governed-action.v0');
if (!contract.operation) contractProblems.push('action.contract.operation is required');
if (!Array.isArray(contract.scopeFields)) contractProblems.push('action.contract.scopeFields must be an array');
if (!Array.isArray(contract.requires)) contractProblems.push('action.contract.requires must be an array');
if (contractProblems.length) {
  emit({verdict: 'WATCH', reason: 'LIVE_BINDINGS_CONTRACT_INVALID', status: 'LIVE_BINDINGS_CONTRACT_INVALID', problems: contractProblems}, 0);
}

if (hasPlaceholder(bindings)) {
  emit({
    verdict: 'WATCH',
    reason: 'LIVE_BINDINGS_PLACEHOLDER',
    status: 'LIVE_BINDINGS_PLACEHOLDER',
    message: 'data/live-bindings.json still contains example placeholder values.',
  }, 0);
}

emit({
  verdict: 'PASS',
  reason: 'LIVE_BINDINGS_READY',
  status: 'LIVE_BINDINGS_READY',
  checked: [
    ...required.map(([section, field]) => `${section}.${field}`),
    'action.contract.kind',
    'action.contract.operation',
    'action.contract.scopeFields',
    'action.contract.requires',
  ],
}, 0);
