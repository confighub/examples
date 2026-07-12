import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join } from 'node:path';

const OAUTH = true;
// Deployment-local live files are excluded like the gitignore excludes them:
// the lifecycle contract under test is the shipped app, not this deployment.
const SKIP = new Set(['.git', 'node_modules', '.lifecycle', 'live-bindings.json', 'cost-findings.json', 'package-lock.json', 'previews', 'reviews', 'receipts']);

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

  // The fleet record owns the app's browser client identity from export time.
  // The fleet record owns the app's browser client identity for its whole
  // lifecycle. The record legitimately advances (register, rotate, revoke),
  // so the test pins state-machine consistency, not one frozen stage.
  const exportedClient = record.oauthClient;
  assert.ok(exportedClient, 'fleet record must carry the oauthClient block');
  assert.deepEqual(exportedClient.stateMachine, ['unregistered', 'registered', 'rotated', 'revoked']);
  assert.ok(exportedClient.stateMachine.includes(exportedClient.state), `unknown client state ${exportedClient.state}`);
  assert.ok(exportedClient.clientName, 'the client name must be pre-declared');
  assert.ok(exportedClient.requiredRedirectUris.length >= 1, 'the serving origin must be declared');
  if (exportedClient.state === 'unregistered') {
    assert.equal(exportedClient.clientId, '');
    assert.equal(exportedClient.org, '');
    assert.equal(exportedClient.organizationId, '');
    assert.equal(exportedClient.externalOrganizationId, '');
    assert.deepEqual(exportedClient.redirectUris, []);
    assert.deepEqual(exportedClient.mutations, []);
  } else {
    assert.ok(exportedClient.clientId, 'a non-unregistered client must carry its client id');
    assert.ok(exportedClient.org, 'a registered client must carry its verified ConfigHub org');
    assert.ok(exportedClient.organizationId, 'a registered client must carry its internal org id');
    assert.ok(exportedClient.externalOrganizationId, 'a registered client must carry its external org id');
    assert.ok(exportedClient.redirectUris.length >= 1, 'a registered client must record its redirect URIs');
    assert.ok(exportedClient.mutations.length >= 1, 'every state advance must be a recorded mutation');
    for (const mutation of exportedClient.mutations) {
      assert.ok(mutation.at, 'client mutations must be timestamped');
    }
  }

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

  writeFileSync(join(appDir, 'data/live-bindings.json'), JSON.stringify({
    schema: 'confighub.live-bindings.v0',
    configHub: {objectUrl: 'https://confighub.example.test/unit', server: 'https://confighub.example.test'},
    action: {endpoint: 'blocked:legacy-action', contract: {kind: 'ConfigHub-governed-action.v0', operation: 'set'}},
  }, null, 2) + '\n');
  const legacyMigrate = runJson(appDir, ['migrate'], 0);
  assert.equal(legacyMigrate.status, 'MIGRATED');
  const migratedBindings = readJson(join(appDir, 'data/live-bindings.json'));
  assert.equal(migratedBindings.schema, 'confighub.live-bindings.v1');
  assert.match(migratedBindings.configHub.organizationId, /^blocked:migration-requires/);
  assert.match(migratedBindings.configHub.externalOrganizationId, /^blocked:migration-requires/);
  assert.deepEqual(migratedBindings.action.contract.scopeFields, []);
  assert.deepEqual(migratedBindings.action.contract.requires, []);
  rmSync(join(appDir, 'data/live-bindings.json'));

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

  // Expected rotate-auth blockers: typed BLOCK at exit 0. The client
  // registration state machine (unregistered -> registered -> rotated ->
  // revoked) refuses transitions that skip states.
  const recordPath = join(appDir, 'confighub/registry/fleet-record.json');
  if (OAUTH) {
    // The deployment's real record may already be registered; reset the
    // copy to the unregistered origin so every transition is exercised.
    const originRecord = readJson(recordPath);
    originRecord.oauthClient.state = 'unregistered';
    originRecord.oauthClient.clientId = '';
    originRecord.oauthClient.redirectUris = [];
    originRecord.oauthClient.mutations = [];
    writeFileSync(recordPath, JSON.stringify(originRecord, null, 2) + '\n');

    // unregistered -> rotate skips the registered state: typed BLOCK.
    const unregisteredRotate = runJson(appDir, ['rotate-auth', '--client-id', 'client_rotated'], 0);
    assert.equal(unregisteredRotate.verdict, 'BLOCK');
    assert.equal(unregisteredRotate.reason, 'CLIENT_UNREGISTERED');
    assert.equal(unregisteredRotate.status, 'ROTATE_AUTH_BLOCKED_UNREGISTERED');

    // Record a registration the way a successful oauth:register run does.
    const registeredRecord = readJson(recordPath);
    const registeredAt = new Date().toISOString();
    registeredRecord.oauthClient.state = 'registered';
    registeredRecord.oauthClient.clientId = 'client_initial';
    registeredRecord.oauthClient.createdAt = registeredAt;
    registeredRecord.oauthClient.redirectUris = ['http://localhost:5173/'];
    registeredRecord.oauthClient.mutations = [{
      kind: 'register',
      action: 'created',
      clientId: 'client_initial',
      redirectUri: 'http://localhost:5173/',
      redirectUriAdded: true,
      at: registeredAt,
    }];
    writeFileSync(recordPath, JSON.stringify(registeredRecord, null, 2) + '\n');

    const blockedRotate = runJson(appDir, ['rotate-auth'], 0);
    assert.equal(blockedRotate.verdict, 'BLOCK');
    assert.equal(blockedRotate.reason, 'NO_CLIENT');
    assert.equal(blockedRotate.status, 'ROTATE_AUTH_BLOCKED_NO_CLIENT');

    const sameClient = runJson(appDir, ['rotate-auth', '--client-id', 'client_initial'], 0);
    assert.equal(sameClient.verdict, 'BLOCK');
    assert.equal(sameClient.reason, 'SAME_CLIENT');
    assert.equal(sameClient.status, 'ROTATE_AUTH_BLOCKED_SAME_CLIENT');

    // registered -> rotated records the rotation in the fleet record.
    const rotated = runJson(appDir, ['rotate-auth', '--client-id', 'client_rotated'], 0);
    assert.equal(rotated.verdict, 'PASS');
    assert.equal(rotated.status, 'AUTH_ROTATION_PREPARED');
    assert.equal(rotated.clientState, 'rotated');
    assert.equal(rotated.previousClientId, 'client_initial');
    assert.ok(rotated.checklist.length >= 3);
    const auth = readJson(join(appDir, '.lifecycle/auth.json'));
    assert.equal(auth.currentClientId, 'client_rotated');
    const rotatedClient = readJson(recordPath).oauthClient;
    assert.equal(rotatedClient.state, 'rotated');
    assert.equal(rotatedClient.clientId, 'client_rotated');
    assert.ok(rotatedClient.lastRotatedAt, 'rotation must record lastRotatedAt');
    const rotateMutation = rotatedClient.mutations[rotatedClient.mutations.length - 1];
    assert.equal(rotateMutation.kind, 'rotate');
    assert.equal(rotateMutation.from, 'client_initial');
    assert.equal(rotateMutation.to, 'client_rotated');

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

  const liveWithoutBindings = runJson(appDir, [
    'registry', '--to', 'LIVE',
    '--actor', 'operator-a',
    '--evidence', 'proof-receipt-2026-07-02',
    '--confighub-url', 'https://confighub.example.test/org/space/unit',
  ], 0, env);
  assert.equal(liveWithoutBindings.verdict, 'BLOCK');
  assert.equal(liveWithoutBindings.reason, 'LIVE_BINDINGS_NOT_READY');
  assert.equal(liveWithoutBindings.status, 'REGISTRY_TRANSITION_BLOCKED');
  assert.equal(liveWithoutBindings.liveBindings.status, 'LIVE_BINDINGS_MISSING');

  const workflow = readJson(join(appDir, 'data/operational-workflow.json'));
  writeFileSync(join(appDir, 'data/live-bindings.json'), JSON.stringify({
    schema: 'confighub.live-bindings.v1',
    status: 'live-values-bound',
    configHub: {
      objectUrl: 'https://confighub.example.test/org/space/unit',
      server: 'https://confighub.example.test',
      organizationId: 'org_live_123',
      externalOrganizationId: 'org_live_external',
      space: 'stage-space',
      spaceId: 'space_live_123',
      unit: 'app-config',
      variant: 'stage',
    },
    approval: {
      objectId: 'changeset-review-123',
      mode: 'explicit approval before mutation',
      scope: workflow.approval.scopeFields,
    },
    action: {
      endpoint: 'blocked:governed-write-executor-not-installed',
      method: 'POST',
      description: 'Governed action endpoint for this workflow.',
      contract: workflow.governedAction,
    },
    proof: {
      receiptObjectId: 'receipt-live-123',
      proofTabs: workflow.proofTabs.map(tab => tab.id),
    },
    runtime: {
      evidenceSource: 'argocd/app/stage-app#runtime',
      readback: 'deployment/stage-app ready',
    },
  }, null, 2) + '\n');
  const bindingBlocked = spawnSync(process.execPath, ['scripts/binding-check.mjs'], {cwd: appDir, encoding: 'utf8'});
  assert.equal(bindingBlocked.status, 0, bindingBlocked.stderr || bindingBlocked.stdout);
  const bindingBlockedJson = JSON.parse(bindingBlocked.stdout);
  assert.equal(bindingBlockedJson.status, 'LIVE_BINDINGS_BLOCKED');
  assert.match(bindingBlockedJson.blocked.join(' '), /blocked:governed-write-executor-not-installed/);
  const liveBlocked = runJson(appDir, [
    'registry', '--to', 'LIVE',
    '--actor', 'operator-a',
    '--evidence', 'proof-receipt-2026-07-02',
    '--confighub-url', 'https://confighub.example.test/org/space/unit',
  ], 0, env);
  assert.equal(liveBlocked.verdict, 'BLOCK');
  assert.equal(liveBlocked.reason, 'LIVE_BINDINGS_NOT_READY');
  assert.equal(liveBlocked.liveBindings.status, 'LIVE_BINDINGS_BLOCKED');
  assert.equal(readJson(join(appDir, 'confighub/registry/fleet-record.json')).state, 'WATCH');

  writeFileSync(join(appDir, 'data/live-bindings.json'), JSON.stringify({
    schema: 'confighub.live-bindings.v1',
    status: 'live-values-bound',
    configHub: {
      objectUrl: 'https://confighub.example.test/org/space/unit',
      server: 'https://confighub.example.test',
      organizationId: 'org_live_123',
      externalOrganizationId: 'org_live_external',
      space: 'stage-space',
      spaceId: 'space_live_123',
      unit: 'app-config',
      variant: 'stage',
    },
    approval: {
      objectId: 'approval-live-123',
      mode: 'explicit approval before mutation',
      scope: workflow.approval.scopeFields,
    },
    action: {
      endpoint: 'https://confighub.example.test/api/actions/apply',
      method: 'POST',
      description: 'Governed action endpoint for this workflow.',
      contract: workflow.governedAction,
    },
    proof: {
      receiptObjectId: 'receipt-live-123',
      proofTabs: workflow.proofTabs.map(tab => tab.id),
    },
    runtime: {
      evidenceSource: 'argocd/app/stage-app#runtime',
      readback: 'deployment/stage-app ready',
    },
  }, null, 2) + '\n');
  const bindingReady = spawnSync(process.execPath, ['scripts/binding-check.mjs'], {cwd: appDir, encoding: 'utf8'});
  assert.equal(bindingReady.status, 0, bindingReady.stderr || bindingReady.stdout);
  const bindingReview = JSON.parse(bindingReady.stdout);
  assert.equal(bindingReview.status, 'LIVE_BINDINGS_REVIEW_READY');
  assert.equal(bindingReview.reason, 'LIVE_BINDINGS_REVIEW_READY');
  assert.equal(bindingReview.reviewReady, true);
  assert.equal(bindingReview.executionReadyWithConfirmation, true);
  assert.equal(bindingReview.commitReady, false);
  assert.equal(bindingReview.atomicity.reason, 'PROVIDER_ATOMIC_EXPECTED_REVISION_UNAVAILABLE');

  const live = runJson(appDir, [
    'registry', '--to', 'LIVE',
    '--actor', 'operator-a',
    '--evidence', 'proof-receipt-2026-07-02',
    '--confighub-url', 'https://confighub.example.test/org/space/unit',
  ], 0, env);
  assert.equal(live.verdict, 'BLOCK');
  assert.equal(live.reason, 'PROVIDER_ATOMIC_EXPECTED_REVISION_UNAVAILABLE');
  assert.equal(live.liveBindings.status, 'LIVE_BINDINGS_REVIEW_READY');
  assert.match(live.nextGate, /confighub[/]issues[/]4714/);
  assert.equal(readJson(join(appDir, 'confighub/registry/fleet-record.json')).state, 'WATCH');

  // Lifecycle transitions after LIVE still need deterministic coverage. Seed
  // a historical LIVE record explicitly; this is test setup, not a claim that
  // the current generated executor can cross the atomic platform gate.
  const historicalLive = readJson(join(appDir, 'confighub/registry/fleet-record.json'));
  historicalLive.state = 'LIVE';
  historicalLive.transitions.push({
    from: 'WATCH',
    to: 'LIVE',
    actor: 'test-fixture',
    evidence: 'fixture:historical-live-before-atomic-gate',
    timestamp: new Date().toISOString(),
    generatorVersion: historicalLive.generator?.version || 'test-fixture',
    configHubUrl: 'https://confighub.example.test/org/space/unit',
  });
  writeFileSync(join(appDir, 'confighub/registry/fleet-record.json'), JSON.stringify(historicalLive, null, 2) + '\n');
  const historicalIndex = readJson(indexPath);
  historicalIndex.orgs[orgSlug].apps[record.app.id].state = 'LIVE';
  writeFileSync(indexPath, JSON.stringify(historicalIndex, null, 2) + '\n');

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
  // Decommission leaves a client record: registered/rotated -> revoked, with
  // the revocation recorded as a mutation. An unregistered client stays as-is.
  if (OAUTH) {
    assert.equal(done.oauthClientState, 'revoked');
    assert.equal(retiredRecord.oauthClient.state, 'revoked');
    assert.ok(retiredRecord.oauthClient.revokedAt, 'revocation must record revokedAt');
    const revokeMutation = retiredRecord.oauthClient.mutations[retiredRecord.oauthClient.mutations.length - 1];
    assert.equal(revokeMutation.kind, 'revoke');
    assert.equal(revokeMutation.clientId, 'client_rotated');
    assert.match(revokeMutation.evidence, /decommission receipt/);
  } else {
    // Fixture-only apps normally never register a client, but a deployment
    // that did register one still gets a real revocation on decommission.
    const endState = retiredRecord.oauthClient.state;
    assert.ok(['unregistered', 'revoked'].includes(endState), endState);
    assert.equal(done.oauthClientState, endState);
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
