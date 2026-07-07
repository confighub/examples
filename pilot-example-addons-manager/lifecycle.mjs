#!/usr/bin/env node
// The app's own life: install, upgrade, migrate, rollback, rotate-auth, decommission.
// The operational loop (preflight -> ... -> receipt) is the app's job; this file is not it.
import { createHash } from 'node:crypto';
import { existsSync } from 'node:fs';
import { copyFile, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';

const args = process.argv.slice(2);
const command = args.find(arg => !arg.startsWith('--')) || 'help';
const json = args.includes('--json');
const confirmed = args.includes('--confirm');
const applyRequested = args.includes('--apply');

function valueAfter(flag) {
  const index = args.indexOf(flag);
  if (index === -1) return '';
  return args[index + 1] || '';
}

const fromDir = valueAfter('--from');
const newClientId = valueAfter('--client-id');
const backupStamp = command === 'rollback' ? valueAfter('--to') : '';
const targetState = command === 'registry' ? valueAfter('--to') : '';
const actorArg = valueAfter('--actor');
const evidenceArg = valueAfter('--evidence');
const configHubUrlArg = valueAfter('--confighub-url');

const MANIFEST_PATH = 'app-export-manifest.json';
const STATE_PATH = '.lifecycle/state.json';
const AUTH_PATH = '.lifecycle/auth.json';
const BACKUP_ROOT = '.lifecycle/backups';
const FLEET_RECORD_PATH = 'confighub/registry/fleet-record.json';
const SKIP_DIRS = new Set(['.git', 'node_modules', '.lifecycle']);
const UNTRACKED_FILES = new Set([MANIFEST_PATH, 'data/live-bindings.json']);

async function loadJsonFile(path, fallback = null) {
  try {
    return JSON.parse(await readFile(path, 'utf8'));
  } catch {
    return fallback;
  }
}

async function writeJsonFile(path, value) {
  await mkdir(dirname(path) || '.', {recursive: true});
  await writeFile(path, JSON.stringify(value, null, 2) + '\n', 'utf8');
}

function output(value, exitCode = 0) {
  if (json) {
    console.log(JSON.stringify(value, null, 2));
  } else {
    console.log(`${value.status}${value.message ? `: ${value.message}` : ''}`);
    if (value.nextGate) console.log(`Next: ${value.nextGate}`);
  }
  return exitCode;
}

function sha256(content) {
  return createHash('sha256').update(content).digest('hex');
}

async function hashOf(path) {
  return sha256(await readFile(path));
}

async function walkFiles(root) {
  const results = [];
  async function walk(dir, prefix) {
    const entries = await readdir(dir, {withFileTypes: true});
    for (const entry of entries) {
      if (SKIP_DIRS.has(entry.name)) continue;
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
      const full = join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(full, relative);
      } else if (entry.isFile()) {
        results.push(relative);
      }
    }
  }
  await walk(root, '');
  return results.sort();
}

function nowIso() {
  return new Date().toISOString();
}

function stamp() {
  return nowIso().replace(/[:.]/g, '-');
}

const manifest = await loadJsonFile(MANIFEST_PATH);
if (command !== 'help' && (!manifest || !manifest.generator || !manifest.file_hashes)) {
  process.exit(output({
    verdict: 'ERROR',
    reason: 'NO_MANIFEST',
    status: 'LIFECYCLE_BLOCKED_NO_MANIFEST',
    message: `${MANIFEST_PATH} with generator and file_hashes blocks is required. Regenerate this app with a current generator.`,
  }, 1));
}
const baseline = manifest ? manifest.file_hashes || {} : {};

async function localDiff() {
  const current = (await walkFiles('.')).filter(path => !UNTRACKED_FILES.has(path));
  const modified = [];
  const missing = [];
  const added = [];
  for (const [path, expected] of Object.entries(baseline)) {
    if (UNTRACKED_FILES.has(path)) continue;
    if (!existsSync(path)) {
      missing.push(path);
    } else if (await hashOf(path) !== expected) {
      modified.push(path);
    }
  }
  for (const path of current) {
    if (!(path in baseline)) added.push(path);
  }
  return {modified, missing, added};
}

function hasPlaceholder(value) {
  if (typeof value === 'string') return value.includes('<') || value.includes('>') || value.includes('example-fill');
  if (Array.isArray(value)) return value.some(hasPlaceholder);
  if (value && typeof value === 'object') return Object.values(value).some(hasPlaceholder);
  return false;
}

async function bindingProbe() {
  const bindings = await loadJsonFile('data/live-bindings.json');
  if (!bindings) return 'LIVE_BINDINGS_MISSING';
  if (hasPlaceholder(bindings)) return 'LIVE_BINDINGS_PLACEHOLDER';
  return 'LIVE_BINDINGS_PRESENT';
}

async function backupFiles(paths, label) {
  const existing = paths.filter(path => existsSync(path));
  const id = `${stamp()}-${label}`;
  const dir = join(BACKUP_ROOT, id);
  for (const path of existing) {
    const target = join(dir, path);
    await mkdir(dirname(target), {recursive: true});
    await copyFile(path, target);
  }
  await writeJsonFile(join(dir, 'backup.json'), {createdAt: nowIso(), label, files: existing});
  return id;
}

async function updateFleetRecord(mutate) {
  const record = await loadJsonFile(FLEET_RECORD_PATH);
  if (!record) return null;
  mutate(record);
  await writeJsonFile(FLEET_RECORD_PATH, record);
  return record;
}

async function readState() {
  return loadJsonFile(STATE_PATH, null);
}

async function writeState(value) {
  await writeJsonFile(STATE_PATH, value);
}

async function install() {
  const required = ['data/operational-workflow.json', 'confighub/self-management.json', FLEET_RECORD_PATH, 'package.json'];
  const missing = required.filter(path => !existsSync(path));
  if (missing.length) {
    return output({
      verdict: 'ERROR',
      reason: 'INSTALL_LAYOUT_MISSING',
      status: 'INSTALL_BLOCKED_LAYOUT',
      missing,
      message: 'Required app files are missing; restore them or regenerate the export.',
    }, 1);
  }
  const drift = await localDiff();
  const bindings = await bindingProbe();
  const backup = await backupFiles([FLEET_RECORD_PATH], 'install');
  await writeState({status: 'installed', installedAt: nowIso(), generator: manifest.generator});
  await updateFleetRecord(record => {
    record.bindings = {status: bindings, checkedAt: nowIso()};
    record.lastLifecycleEvent = {command: 'install', at: nowIso()};
  });
  return output({
    verdict: 'PASS',
    reason: 'INSTALL_COMPLETE',
    status: 'INSTALL_COMPLETE',
    generator: manifest.generator,
    liveBindings: bindings,
    baselineDrift: {modified: drift.modified.length, missing: drift.missing.length, added: drift.added.length},
    backup,
    nextGate: 'Bind live values in data/live-bindings.json, run npm run binding:check, and register the fleet-record Unit through the governed path.',
  });
}

async function upgrade() {
  const local = await localDiff();
  if (!fromDir) {
    return output({
      verdict: 'PASS',
      reason: 'LOCAL_STATE_REPORT',
      status: 'UPGRADE_LOCAL_STATE_REPORT',
      pinnedGenerator: manifest.generator,
      localModifications: local,
      nextGate: 'Regenerate this app with the same or a newer generator version into a separate directory, then rerun: node lifecycle.mjs upgrade --from <regenerated-dir> [--apply] --json',
    });
  }
  if (!existsSync(join(fromDir, MANIFEST_PATH))) {
    return output({
      verdict: 'ERROR',
      reason: 'INVALID_SOURCE',
      status: 'UPGRADE_BLOCKED_INVALID_SOURCE',
      message: `--from must point at a regenerated app directory containing ${MANIFEST_PATH}.`,
    }, 1);
  }
  const nextManifest = await loadJsonFile(join(fromDir, MANIFEST_PATH));
  const nextFiles = (await walkFiles(fromDir)).filter(path => !UNTRACKED_FILES.has(path));
  const nextHashes = {};
  for (const path of nextFiles) {
    nextHashes[path] = await hashOf(join(fromDir, path));
  }
  const paths = new Set([...Object.keys(baseline), ...Object.keys(nextHashes)]);
  const updates = [];
  const additions = [];
  const removals = [];
  const conflicts = [];
  const preservedLocalChanges = [];
  for (const path of [...paths].sort()) {
    if (UNTRACKED_FILES.has(path)) continue;
    const baseHash = baseline[path];
    const nextHash = nextHashes[path];
    const currentHash = existsSync(path) ? await hashOf(path) : null;
    const localChanged = currentHash !== (baseHash ?? null);
    const upstreamChanged = nextHash !== baseHash;
    if (!upstreamChanged) {
      if (localChanged) preservedLocalChanges.push(path);
      continue;
    }
    if (nextHash === undefined) {
      if (localChanged) conflicts.push({path, kind: 'removed-upstream-modified-locally'});
      else removals.push(path);
      continue;
    }
    if (baseHash === undefined) {
      if (currentHash === null) additions.push(path);
      else if (currentHash !== nextHash) conflicts.push({path, kind: 'added-upstream-exists-locally'});
      continue;
    }
    if (localChanged) {
      if (currentHash === nextHash) continue;
      conflicts.push({path, kind: 'modified-both'});
    } else {
      updates.push(path);
    }
  }
  const summary = {
    verdict: 'PASS',
    reason: conflicts.length ? 'CONFLICTS' : (applyRequested ? 'APPLIED' : 'READY'),
    status: conflicts.length ? 'UPGRADE_CONFLICTS' : (applyRequested ? 'UPGRADE_APPLIED' : 'UPGRADE_READY'),
    pinnedGenerator: manifest.generator,
    nextGenerator: nextManifest ? nextManifest.generator || null : null,
    updates,
    additions,
    removals,
    conflicts,
    preservedLocalChanges,
  };
  if (conflicts.length) {
    // Conflicts are an expected upgrade outcome to resolve, not a shell failure.
    summary.verdict = 'WATCH';
    summary.reason = 'CONFLICTS';
    summary.nextGate = 'Resolve the conflicted files by hand (keep or take the regenerated version), then rerun upgrade.';
    return output(summary, 0);
  }
  if (!applyRequested) {
    summary.nextGate = 'Rerun with --apply to take the regenerated files. Local-only changes are preserved.';
    return output(summary);
  }
  summary.backup = await backupFiles([...updates, ...removals, MANIFEST_PATH], 'upgrade');
  for (const path of [...updates, ...additions]) {
    await mkdir(dirname(path) || '.', {recursive: true});
    await copyFile(join(fromDir, path), path);
  }
  for (const path of removals) {
    await rm(path, {force: true});
  }
  await copyFile(join(fromDir, MANIFEST_PATH), MANIFEST_PATH);
  const state = (await readState()) || {};
  await writeState({...state, status: state.status || 'installed', lastUpgrade: {at: nowIso(), toGenerator: summary.nextGenerator}});
  summary.nextGate = 'Run node lifecycle.mjs migrate --json, then npm run verify.';
  return output(summary);
}

// Schema-version migrations for the shared contract files. Keyed by the schema
// id found on disk; each step carries the target schema id and a transform.
const MIGRATIONS = {};

async function migrate() {
  const targets = [
    {id: 'operational_workflow', path: 'data/operational-workflow.json', required: true},
    {id: 'live_bindings', path: 'data/live-bindings.json', required: false},
    {id: 'self_management', path: 'confighub/self-management.json', required: true},
  ];
  const expected = manifest.schemas || {};
  const results = [];
  let missingRequired = false;
  let unknown = false;
  let migrated = false;
  for (const target of targets) {
    const expectedSchema = expected[target.id] || '';
    if (!existsSync(target.path)) {
      if (target.required) {
        missingRequired = true;
        results.push({file: target.path, state: 'missing', expected: expectedSchema});
      } else {
        results.push({file: target.path, state: 'absent-optional', expected: expectedSchema});
      }
      continue;
    }
    let value = await loadJsonFile(target.path);
    let schema = value && typeof value.schema === 'string' ? value.schema : '';
    if (schema === expectedSchema) {
      results.push({file: target.path, state: 'current', schema});
      continue;
    }
    const applied = [];
    while (MIGRATIONS[schema]) {
      const step = MIGRATIONS[schema];
      value = step.transform(value);
      value.schema = step.to;
      applied.push(`${schema} -> ${step.to}`);
      schema = step.to;
    }
    if (schema === expectedSchema && applied.length) {
      await backupFiles([target.path], 'migrate');
      await writeJsonFile(target.path, value);
      migrated = true;
      results.push({file: target.path, state: 'migrated', schema, applied});
    } else {
      unknown = true;
      results.push({file: target.path, state: 'unknown-schema', schema, expected: expectedSchema});
    }
  }
  let status = 'MIGRATE_CURRENT';
  let verdict = 'PASS';
  let reason = migrated ? 'MIGRATED' : 'CURRENT';
  if (missingRequired) {
    // Missing a core contract file the migration must parse: a genuine error.
    status = 'MIGRATE_BLOCKED_MISSING_FILE';
    verdict = 'ERROR';
    reason = 'MISSING_FILE';
  } else if (unknown) {
    // The on-disk schema is unrecognized: a typed classification, not a crash.
    status = 'MIGRATE_UNKNOWN_SCHEMA';
    verdict = 'BLOCK';
    reason = 'UNKNOWN_SCHEMA';
  } else if (migrated) {
    status = 'MIGRATED';
  }
  const result = {verdict, reason, status, results};
  if (missingRequired || unknown) {
    result.nextGate = 'Restore the missing file, add the missing migration, or regenerate the app before continuing.';
  }
  return output(result, missingRequired ? 1 : 0);
}

async function rollback() {
  let entries = [];
  try {
    entries = (await readdir(BACKUP_ROOT, {withFileTypes: true}))
      .filter(entry => entry.isDirectory())
      .map(entry => entry.name)
      .sort();
  } catch {
    entries = [];
  }
  if (!entries.length) {
    return output({verdict: 'BLOCK', reason: 'NO_BACKUP', status: 'ROLLBACK_NO_BACKUP', message: 'No lifecycle backups exist under .lifecycle/backups.'}, 0);
  }
  const chosen = backupStamp || entries[entries.length - 1];
  if (!entries.includes(chosen)) {
    return output({verdict: 'BLOCK', reason: 'NO_BACKUP', status: 'ROLLBACK_NO_BACKUP', message: `Backup ${chosen} not found.`, available: entries}, 0);
  }
  const dir = join(BACKUP_ROOT, chosen);
  const meta = await loadJsonFile(join(dir, 'backup.json'), {files: []});
  const restored = [];
  for (const path of meta.files || []) {
    const source = join(dir, path);
    if (!existsSync(source)) continue;
    await mkdir(dirname(path) || '.', {recursive: true});
    await copyFile(source, path);
    restored.push(path);
  }
  const state = (await readState()) || {};
  await writeState({...state, status: 'installed', rolledBackAt: nowIso(), rolledBackFrom: chosen});
  return output({
    verdict: 'PASS',
    reason: 'ROLLBACK_COMPLETE',
    status: 'ROLLBACK_COMPLETE',
    backup: chosen,
    restored,
    note: 'App-side files restored. ConfigHub-side app config rolls back separately by restoring the app config Unit revision through the governed path.',
  });
}

async function rotateAuth() {
  if (manifest.auth_mode !== 'browser-oauth') {
    return output({
      verdict: 'BLOCK',
      reason: 'FIXTURE_ONLY',
      status: 'ROTATE_AUTH_NOT_APPLICABLE_FIXTURE_ONLY',
      message: 'This export opted out of browser OAuth; there is no client to rotate.',
      nextGate: 'Regenerate the app with browser OAuth before treating it as a live ConfigHub app.',
    }, 0);
  }
  if (!newClientId) {
    return output({
      verdict: 'BLOCK',
      reason: 'NO_CLIENT',
      status: 'ROTATE_AUTH_BLOCKED_NO_CLIENT',
      message: 'Provision a replacement browser OAuth client through the supported ConfigHub path, then rerun with --client-id <new-client-id>.',
    }, 0);
  }
  const auth = (await loadJsonFile(AUTH_PATH, {previous: []})) || {previous: []};
  if (auth.currentClientId === newClientId) {
    return output({
      verdict: 'BLOCK',
      reason: 'SAME_CLIENT',
      status: 'ROTATE_AUTH_BLOCKED_SAME_CLIENT',
      message: 'The new client id matches the currently recorded client id; rotation needs a fresh client.',
    }, 0);
  }
  const rotatedAt = nowIso();
  const previous = [...(auth.previous || [])];
  if (auth.currentClientId) {
    previous.push({clientId: auth.currentClientId, rotatedOutAt: rotatedAt});
  }
  const backup = await backupFiles([FLEET_RECORD_PATH], 'rotate-auth');
  await writeJsonFile(AUTH_PATH, {currentClientId: newClientId, rotatedAt, previous});
  await updateFleetRecord(record => {
    record.authRotation = {lastRotatedAt: rotatedAt, clientIdRef: AUTH_PATH};
    record.lastLifecycleEvent = {command: 'rotate-auth', at: rotatedAt};
  });
  const workflow = await loadJsonFile('data/operational-workflow.json', {});
  const slug = workflow && workflow.app && workflow.app.slug ? workflow.app.slug : 'app';
  return output({
    verdict: 'PASS',
    reason: 'AUTH_ROTATION_PREPARED',
    status: 'AUTH_ROTATION_PREPARED',
    backup,
    rotatedAt,
    checklist: [
      `Update the ${slug}-oauth secret (OAUTH_CLIENT_ID) through the governed config path.`,
      'Restart the app with the new OAUTH_CLIENT_ID.',
      'Run npm run oauth:smoke and complete a fresh browser sign-in to /api/me.',
      'Revoke the old client only after the new client is proven.',
    ],
  });
}

// Registry state machine (contract stage 13): WATCH -> LIVE -> DEPRECATED ->
// RETIRED. RETIRED is deliberately absent from the generic edges below: it is
// reachable only through decommission, which records the receipt and
// deregisters from the fleet index.
const REGISTRY_STATES = ['WATCH', 'LIVE', 'DEPRECATED', 'RETIRED'];
const REGISTRY_TRANSITIONS = {
  WATCH: ['LIVE'],
  LIVE: ['DEPRECATED'],
  DEPRECATED: [],
  RETIRED: [],
};

function lastConfigHubUrl(record) {
  const transitions = Array.isArray(record.transitions) ? record.transitions : [];
  for (let i = transitions.length - 1; i >= 0; i -= 1) {
    if (transitions[i] && transitions[i].configHubUrl) return transitions[i].configHubUrl;
  }
  return '';
}

function appendTransition(record, to, fields) {
  const entry = {
    from: record.state || null,
    to,
    actor: fields.actor,
    evidence: fields.evidence,
    timestamp: nowIso(),
    generatorVersion: manifest.generator && manifest.generator.version ? manifest.generator.version : '',
    configHubUrl: fields.configHubUrl,
  };
  record.state = to;
  record.transitions = [...(Array.isArray(record.transitions) ? record.transitions : []), entry];
  return entry;
}

async function updateFleetIndexEntry(appId, mutate) {
  const indexPath = process.env.CONFIGHUB_FLEET_INDEX_FILE || '';
  if (!indexPath) return 'not-configured';
  if (!existsSync(indexPath)) return 'index-file-missing';
  const index = await loadJsonFile(indexPath);
  let found = false;
  for (const org of Object.values(index && index.orgs ? index.orgs : {})) {
    const app = org && org.apps ? org.apps[appId] : null;
    if (app) {
      mutate(app);
      found = true;
    }
  }
  if (!found) return 'app-not-found';
  await writeJsonFile(indexPath, index);
  return 'updated';
}

async function registry() {
  const record = await loadJsonFile(FLEET_RECORD_PATH);
  if (!record) {
    return output({
      verdict: 'ERROR',
      reason: 'NO_RECORD',
      status: 'REGISTRY_BLOCKED_NO_RECORD',
      message: `${FLEET_RECORD_PATH} is required.`,
    }, 1);
  }
  if (!targetState) {
    return output({
      verdict: 'PASS',
      reason: 'REGISTRY_STATE',
      status: 'REGISTRY_STATE',
      state: record.state || '',
      stateMachine: record.stateMachine || REGISTRY_STATES,
      transitions: record.transitions || [],
      fleetIndexUnit: record.fleetIndex ? record.fleetIndex.unitRef || '' : '',
    });
  }
  const to = targetState.toUpperCase();
  if (!REGISTRY_STATES.includes(to)) {
    return output({
      verdict: 'BLOCK',
      reason: 'UNKNOWN_STATE',
      status: 'REGISTRY_TRANSITION_BLOCKED',
      message: `Unknown registry state ${targetState}. State machine: ${REGISTRY_STATES.join(' -> ')}.`,
    }, 0);
  }
  if (to === 'RETIRED') {
    return output({
      verdict: 'BLOCK',
      reason: 'RETIRED_REQUIRES_DECOMMISSION',
      status: 'REGISTRY_TRANSITION_BLOCKED',
      message: 'RETIRED requires a decommission receipt and fleet-index deregistration. Run node lifecycle.mjs decommission --confirm --json.',
    }, 0);
  }
  const from = record.state || '';
  const allowed = REGISTRY_TRANSITIONS[from] || [];
  if (!allowed.includes(to)) {
    return output({
      verdict: 'BLOCK',
      reason: 'INVALID_TRANSITION',
      status: 'REGISTRY_TRANSITION_BLOCKED',
      message: `No ${from} -> ${to} edge. Allowed from ${from}: ${allowed.join(', ') || 'none (RETIRED goes through decommission)'}.`,
    }, 0);
  }
  const actor = actorArg;
  const evidence = evidenceArg;
  const configHubUrl = configHubUrlArg || (to === 'LIVE' ? '' : lastConfigHubUrl(record));
  const missing = [];
  if (!actor) missing.push('--actor');
  if (!evidence) missing.push('--evidence');
  if (to === 'LIVE' && !configHubUrl) missing.push('--confighub-url');
  if (missing.length) {
    return output({
      verdict: 'BLOCK',
      reason: 'MISSING_TRANSITION_FIELDS',
      status: 'REGISTRY_TRANSITION_BLOCKED',
      missing,
      message: to === 'LIVE'
        ? 'LIVE requires the stage-12 proof layers green: pass --actor, --evidence <proof-receipt-ref>, and --confighub-url <unit-url>.'
        : 'Every transition records actor, evidence, timestamp, generatorVersion, and configHubUrl.',
    }, 0);
  }
  const backup = await backupFiles([FLEET_RECORD_PATH], `registry-${to.toLowerCase()}`);
  let entry = null;
  await updateFleetRecord(rec => {
    entry = appendTransition(rec, to, {actor, evidence, configHubUrl});
    rec.lastLifecycleEvent = {command: 'registry', at: entry.timestamp};
  });
  const fleetIndex = await updateFleetIndexEntry(record.app ? record.app.id : '', app => {
    app.state = to;
    app.transitions = [...(Array.isArray(app.transitions) ? app.transitions : []), entry];
  });
  return output({
    verdict: 'PASS',
    reason: 'TRANSITION_RECORDED',
    status: 'REGISTRY_TRANSITION_RECORDED',
    state: to,
    transition: entry,
    backup,
    fleetIndex,
    nextGate: 'Apply the updated fleet-record Unit through the governed path so the org fleet Space reflects this state.',
  });
}

async function decommission() {
  const record = await loadJsonFile(FLEET_RECORD_PATH);
  if (!record) {
    return output({
      verdict: 'ERROR',
      reason: 'NO_RECORD',
      status: 'DECOMMISSION_BLOCKED_NO_RECORD',
      message: `${FLEET_RECORD_PATH} is required to deregister from the fleet index.`,
    }, 1);
  }
  if (!confirmed) {
    // Awaiting an explicit human decision: typed ASK at exit 0, no mutation.
    return output({
      verdict: 'ASK',
      reason: 'CONFIRM_REQUIRED',
      status: 'DECOMMISSION_BLOCKED_CONFIRM_REQUIRED',
      state: record.state || '',
      wouldTransition: `${record.state || ''} -> RETIRED`,
      wouldUpdate: [FLEET_RECORD_PATH, STATE_PATH],
      deregistersFrom: record.fleetIndex ? record.fleetIndex.unitRef || '' : '',
      message: 'Nothing changed. Rerun with --confirm to decommission and deregister this app.',
    }, 0);
  }
  const at = nowIso();
  const backup = await backupFiles([FLEET_RECORD_PATH], 'decommission');
  let transition = null;
  await updateFleetRecord(rec => {
    transition = appendTransition(rec, 'RETIRED', {
      actor: actorArg || rec.owner || '',
      evidence: `decommission receipt: lifecycle backup ${backup}`,
      configHubUrl: configHubUrlArg || lastConfigHubUrl(rec),
    });
    rec.deregistration = {deregisteredAt: at};
    rec.lastLifecycleEvent = {command: 'decommission', at};
  });
  const state = (await readState()) || {};
  await writeState({...state, status: 'decommissioned', decommissionedAt: at});
  const fleetIndex = await updateFleetIndexEntry(record.app ? record.app.id : '', app => {
    app.state = 'RETIRED';
    app.transitions = [...(Array.isArray(app.transitions) ? app.transitions : []), transition];
    app.deregistration = {deregisteredAt: at};
  });
  return output({
    verdict: 'PASS',
    reason: 'DECOMMISSIONED',
    status: 'DECOMMISSIONED',
    state: 'RETIRED',
    transition,
    backup,
    fleetIndex: fleetIndex === 'updated' ? 'deregistered' : fleetIndex,
    fleetIndexUnit: record.fleetIndex ? record.fleetIndex.unitRef || '' : '',
    nextGate: 'Apply the RETIRED fleet-record Unit change through the governed path, remove delete gates deliberately, then retire the deployment. Run node lifecycle.mjs rollback --json to reverse.',
  });
}

async function help() {
  return output({
    verdict: 'PASS',
    reason: 'LIFECYCLE_HELP',
    status: 'LIFECYCLE_HELP',
    commands: {
      install: 'node lifecycle.mjs install --json',
      upgrade: 'node lifecycle.mjs upgrade [--from <regenerated-dir>] [--apply] --json',
      migrate: 'node lifecycle.mjs migrate --json',
      rollback: 'node lifecycle.mjs rollback [--to <backup-stamp>] --json',
      'rotate-auth': 'node lifecycle.mjs rotate-auth --client-id <new-client-id> --json',
      decommission: 'node lifecycle.mjs decommission --confirm --json',
      registry: 'node lifecycle.mjs registry [--to LIVE|DEPRECATED --actor <who> --evidence <ref> [--confighub-url <url>]] --json',
    },
  });
}

const handlers = {
  install,
  upgrade,
  migrate,
  rollback,
  'rotate-auth': rotateAuth,
  decommission,
  registry,
  help,
};
const handler = handlers[command];
if (!handler) {
  process.exit(output({verdict: 'ERROR', reason: 'UNKNOWN_COMMAND', status: 'LIFECYCLE_UNKNOWN_COMMAND', message: `Unknown lifecycle command: ${command}`}, 1));
}
process.exit(await handler());
