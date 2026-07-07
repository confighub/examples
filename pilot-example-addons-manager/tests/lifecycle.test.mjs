import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join } from 'node:path';

const OAUTH = true;
const SKIP = new Set(['.git', 'node_modules', '.lifecycle']);

const workRoot = mkdtempSync(join(tmpdir(), 'app-lifecycle-test-'));
const appDir = join(workRoot, 'app');
const regenDir = join(workRoot, 'regen');
const indexPath = join(workRoot, 'fleet-index.json');

function copyApp(target) {
  cpSync('.', target, {
    recursive: true,
    filter: source => !SKIP.has(basename(source)),
  });
}

function run(dir, cliArgs, env = {}) {
  return spawnSync(process.execPath, ['lifecycle.mjs', ...cliArgs], {
    cwd: dir,
    encoding: 'utf8',
    env: {...process.env, ...env},
  });
}

function runJson(dir, cliArgs, expectedStatus, env = {}) {
  const result = run(dir, [...cliArgs, '--json'], env);
  assert.equal(result.status, expectedStatus, result.stderr || result.stdout);
  const source = result.stdout || result.stderr;
  const start = source.indexOf('{');
  assert.notEqual(start, -1, result.stderr || result.stdout);
  const parsed = JSON.parse(source.slice(start));
  // Contract invariant: a classification (PASS/WATCH/BLOCK/ASK) exits 0;
  // only ERROR exits non-zero.
  if (typeof parsed.verdict === 'string') {
    const classified = ['PASS', 'WATCH', 'BLOCK', 'ASK'].includes(parsed.verdict);
    assert.equal(result.status === 0, classified, `verdict ${parsed.verdict} must map to exit ${classified ? '0' : 'non-zero'}: ${source}`);
  }
  return parsed;
}

function readJson(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

try {
  copyApp(appDir);
  copyApp(regenDir);

  const manifest = readJson(join(appDir, 'app-export-manifest.json'));
  const record = readJson(join(appDir, 'confighub/registry/fleet-record.json'));
  assert.equal(record.state, 'WATCH');
  assert.deepEqual(record.stateMachine, ['WATCH', 'LIVE', 'DEPRECATED', 'RETIRED']);
  assert.equal(record.transitions.length, 1);
  assert.equal(record.transitions[0].to, 'WATCH');
  for (const field of ['actor', 'evidence', 'timestamp', 'generatorVersion']) {
    assert.ok(record.transitions[0][field], `initial transition must record ${field}`);
  }
  assert.ok('configHubUrl' in record.transitions[0], 'initial transition must carry configHubUrl');
  assert.ok(record.owner, 'fleet record must carry an owner');
  assert.ok(record.onCall, 'fleet record must carry an on-call');
  assert.ok(record.generator.version, 'fleet record must pin the generator version');

  const orgSlug = (record.fleetIndex.space || '').replace(/-app-fleet$/, '');
  writeFileSync(indexPath, JSON.stringify({
    schema: 'confighub.app-fleet-index.v0',
    model: record.fleetIndex.model,
    orgs: {[orgSlug]: {space: record.fleetIndex.space, apps: {[record.app.id]: record}}},
  }, null, 2) + '\n');

  const install = runJson(appDir, ['install'], 0);
  assert.equal(install.verdict, 'PASS');
  assert.equal(install.status, 'INSTALL_COMPLETE');
  assert.equal(install.generator.version, manifest.generator.version);
  assert.ok(existsSync(join(appDir, '.lifecycle/state.json')));

  const migrate = runJson(appDir, ['migrate'], 0);
  assert.equal(migrate.status, 'MIGRATE_CURRENT');
  assert.ok(migrate.results.every(row => row.state === 'current' || row.state === 'absent-optional'));

  const report = runJson(appDir, ['upgrade'], 0);
  assert.equal(report.status, 'UPGRADE_LOCAL_STATE_REPORT');
  assert.equal(report.pinnedGenerator.version, manifest.generator.version);

  const readmePath = join(regenDir, 'README.md');
  const upstreamReadme = readFileSync(readmePath, 'utf8') + '\nUpstream regeneration note.\n';
  writeFileSync(readmePath, upstreamReadme);
  const regenManifestPath = join(regenDir, 'app-export-manifest.json');
  const regenManifest = readJson(regenManifestPath);
  regenManifest.file_hashes['README.md'] = createHash('sha256').update(upstreamReadme).digest('hex');
  writeFileSync(regenManifestPath, JSON.stringify(regenManifest, null, 2) + '\n');

  const ready = runJson(appDir, ['upgrade', '--from', regenDir], 0);
  assert.equal(ready.status, 'UPGRADE_READY');
  assert.ok(ready.updates.includes('README.md'), JSON.stringify(ready));

  const localReadmePath = join(appDir, 'README.md');
  const originalLocalReadme = readFileSync(localReadmePath, 'utf8');
  writeFileSync(localReadmePath, originalLocalReadme + '\nLocal note.\n');
  // Conflicts are an expected upgrade outcome: typed WATCH at exit 0.
  const conflicted = runJson(appDir, ['upgrade', '--from', regenDir], 0);
  assert.equal(conflicted.verdict, 'WATCH');
  assert.equal(conflicted.reason, 'CONFLICTS');
  assert.equal(conflicted.status, 'UPGRADE_CONFLICTS');
  assert.ok(conflicted.conflicts.some(row => row.path === 'README.md'));

  writeFileSync(localReadmePath, originalLocalReadme);
  const applied = runJson(appDir, ['upgrade', '--from', regenDir, '--apply'], 0);
  assert.equal(applied.status, 'UPGRADE_APPLIED');
  assert.match(readFileSync(localReadmePath, 'utf8'), /Upstream regeneration note/);
  const postUpgradeMigrate = runJson(appDir, ['migrate'], 0);
  assert.equal(postUpgradeMigrate.status, 'MIGRATE_CURRENT');

  // Expected rotate-auth blockers: typed BLOCK at exit 0.
  if (OAUTH) {
    const blockedRotate = runJson(appDir, ['rotate-auth'], 0);
    assert.equal(blockedRotate.verdict, 'BLOCK');
    assert.equal(blockedRotate.reason, 'NO_CLIENT');
    assert.equal(blockedRotate.status, 'ROTATE_AUTH_BLOCKED_NO_CLIENT');
    const rotated = runJson(appDir, ['rotate-auth', '--client-id', 'client_rotated'], 0);
    assert.equal(rotated.verdict, 'PASS');
    assert.equal(rotated.status, 'AUTH_ROTATION_PREPARED');
    assert.ok(rotated.checklist.length >= 3);
    const auth = readJson(join(appDir, '.lifecycle/auth.json'));
    assert.equal(auth.currentClientId, 'client_rotated');
    const repeatRotate = runJson(appDir, ['rotate-auth', '--client-id', 'client_rotated'], 0);
    assert.equal(repeatRotate.verdict, 'BLOCK');
    assert.equal(repeatRotate.reason, 'SAME_CLIENT');
    assert.equal(repeatRotate.status, 'ROTATE_AUTH_BLOCKED_SAME_CLIENT');
  } else {
    const blockedRotate = runJson(appDir, ['rotate-auth', '--client-id', 'client_rotated'], 0);
    assert.equal(blockedRotate.verdict, 'BLOCK');
    assert.equal(blockedRotate.reason, 'FIXTURE_ONLY');
    assert.equal(blockedRotate.status, 'ROTATE_AUTH_NOT_APPLICABLE_FIXTURE_ONLY');
  }

  // Registry state machine: WATCH -> LIVE -> DEPRECATED -> RETIRED, every
  // transition recording actor, evidence, timestamp, generatorVersion, configHubUrl.
  const env = {CONFIGHUB_FLEET_INDEX_FILE: indexPath};
  const registryView = runJson(appDir, ['registry'], 0, env);
  assert.equal(registryView.status, 'REGISTRY_STATE');
  assert.equal(registryView.state, 'WATCH');

  // Expected registry transition blockers: typed BLOCK at exit 0.
  const liveMissing = runJson(appDir, ['registry', '--to', 'LIVE'], 0, env);
  assert.equal(liveMissing.verdict, 'BLOCK');
  assert.equal(liveMissing.status, 'REGISTRY_TRANSITION_BLOCKED');
  assert.equal(liveMissing.reason, 'MISSING_TRANSITION_FIELDS');
  assert.ok(liveMissing.missing.includes('--confighub-url'));

  const skipDeprecated = runJson(appDir, ['registry', '--to', 'DEPRECATED', '--actor', 'operator-a', '--evidence', 'note'], 0, env);
  assert.equal(skipDeprecated.verdict, 'BLOCK');
  assert.equal(skipDeprecated.reason, 'INVALID_TRANSITION');

  const retiredDirect = runJson(appDir, ['registry', '--to', 'RETIRED', '--actor', 'operator-a', '--evidence', 'note'], 0, env);
  assert.equal(retiredDirect.verdict, 'BLOCK');
  assert.equal(retiredDirect.reason, 'RETIRED_REQUIRES_DECOMMISSION');

  const live = runJson(appDir, [
    'registry', '--to', 'LIVE',
    '--actor', 'operator-a',
    '--evidence', 'proof-receipt-2026-07-02',
    '--confighub-url', 'https://confighub.example.test/org/space/unit',
  ], 0, env);
  assert.equal(live.status, 'REGISTRY_TRANSITION_RECORDED');
  assert.equal(live.state, 'LIVE');
  assert.equal(live.fleetIndex, 'updated');
  assert.equal(readJson(indexPath).orgs[orgSlug].apps[record.app.id].state, 'LIVE');

  const deprecated = runJson(appDir, [
    'registry', '--to', 'DEPRECATED',
    '--actor', 'operator-a',
    '--evidence', 'replacement app announced',
  ], 0, env);
  assert.equal(deprecated.status, 'REGISTRY_TRANSITION_RECORDED');
  assert.equal(deprecated.state, 'DEPRECATED');
  assert.equal(deprecated.transition.configHubUrl, 'https://confighub.example.test/org/space/unit');

  // Confirm-required preview: typed ASK at exit 0, no mutation.
  const preview = runJson(appDir, ['decommission'], 0);
  assert.equal(preview.verdict, 'ASK');
  assert.equal(preview.reason, 'CONFIRM_REQUIRED');
  assert.equal(preview.status, 'DECOMMISSION_BLOCKED_CONFIRM_REQUIRED');
  assert.equal(preview.wouldTransition, 'DEPRECATED -> RETIRED');

  const done = runJson(appDir, ['decommission', '--confirm'], 0, env);
  assert.equal(done.verdict, 'PASS');
  assert.equal(done.status, 'DECOMMISSIONED');
  assert.equal(done.state, 'RETIRED');
  assert.equal(done.fleetIndex, 'deregistered');
  assert.match(done.transition.evidence, /decommission receipt/);
  const retiredRecord = readJson(join(appDir, 'confighub/registry/fleet-record.json'));
  assert.equal(retiredRecord.state, 'RETIRED');
  assert.deepEqual(retiredRecord.transitions.map(row => row.to), ['WATCH', 'LIVE', 'DEPRECATED', 'RETIRED']);
  for (const row of retiredRecord.transitions) {
    for (const field of ['actor', 'evidence', 'timestamp', 'generatorVersion']) {
      assert.ok(row[field], `transition to ${row.to} must record ${field}`);
    }
    assert.ok('configHubUrl' in row, `transition to ${row.to} must carry configHubUrl`);
  }
  const retiredIndexEntry = readJson(indexPath).orgs[orgSlug].apps[record.app.id];
  assert.equal(retiredIndexEntry.state, 'RETIRED');
  assert.ok(retiredIndexEntry.deregistration.deregisteredAt);

  const refused = spawnSync(process.execPath, ['server.mjs'], {cwd: appDir, encoding: 'utf8'});
  assert.equal(refused.status, 3, refused.stderr || refused.stdout);
  assert.match(refused.stderr, /DECOMMISSIONED/);

  const rollback = runJson(appDir, ['rollback'], 0);
  assert.equal(rollback.status, 'ROLLBACK_COMPLETE');
  assert.equal(readJson(join(appDir, 'confighub/registry/fleet-record.json')).state, 'DEPRECATED');
  assert.equal(readJson(join(appDir, '.lifecycle/state.json')).status, 'installed');
} finally {
  rmSync(workRoot, {recursive: true, force: true});
}
